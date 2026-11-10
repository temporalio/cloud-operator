package serviceaccount

import (
	"context"
	"fmt"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/protobuf/proto"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/common"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceSync creates the Temporal Cloud service account if it doesn't exist
// yet, and otherwise updates it to match `desired`. It returns a copy of
// `latest` with any status changes.
//
// `desired` is NEVER mutated.
// `latest` is NEVER mutated.
func (r *Reconciler) ResourceSync(
	ctx context.Context,
	desired kttypes.Resource,
	latest kttypes.Resource,
) (kttypes.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.sync")
	defer func() {
		exit(err)
	}()

	updated := latest.DeepCopy()

	tcc, err := r.CloudClientFor(ctx, desired)
	if err != nil {
		return updated, fmt.Errorf("failed getting tcloud client: %w", err)
	}

	desiredSA := desired.CR().(*tcloudv1alpha1.ServiceAccount)
	if !latest.(types.Resource).PreviouslyReconciled() {
		err = createServiceAccount(
			ctx, tcc, desiredSA, updated.CR().(*tcloudv1alpha1.ServiceAccount),
		)
		return updated, err
	}

	err = updateServiceAccount(
		ctx, tcc, desiredSA, latest.CR().(*tcloudv1alpha1.ServiceAccount),
	)
	return updated, err
}

// createServiceAccount creates the Temporal Cloud service account for the
// supplied `desired` CR and saves its ID in `updated`.
//
// `desired` is NEVER mutated.
func createServiceAccount(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.ServiceAccount,
	updated *tcloudv1alpha1.ServiceAccount, // mutated by this function
) error {
	refs := updated.Status.References
	if err := checkReferences(desired, refs); err != nil {
		return err
	}

	resp, err := tcc.CreateServiceAccount(ctx, &tcloudservicev1.CreateServiceAccountRequest{
		Spec: desired.Spec.ToProto(desired.Namespace, refs),
	})
	if err != nil {
		return common.CloudCallError("creating tcloud service account", err)
	}

	updated.Status.Resource.ID = resp.GetServiceAccountId()
	return requeue.NeededAfter(
		fmt.Errorf(
			"created tcloud service account %q, waiting for activation",
			resp.GetServiceAccountId(),
		),
		requeue.DefaultRequeueAfterDuration,
	)
}

// updateServiceAccount makes at most one Temporal Cloud call to move an
// existing service account toward the desired state. An update starts an
// async operation, so after it we requeue and let the next reconcile observe
// the result.
func updateServiceAccount(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.ServiceAccount,
	latest *tcloudv1alpha1.ServiceAccount,
) error {
	refs := latest.Status.References
	if err := checkReferences(desired, refs); err != nil {
		return err
	}
	want := desired.Spec.ToProto(desired.Namespace, refs)

	resourceID := latest.Status.Resource.ID
	err := common.RequireActive("service account", resourceID, latest.Status.Resource.State)
	if err != nil {
		return err
	}

	// latest's spec comes from Temporal Cloud, so every reference in it is
	// already an ID.
	var none tcloudv1alpha1.ServiceAccountReferences
	got := latest.Spec.ToProto(latest.Namespace, none)

	// The CR can't change its scope or target (CEL), so a mismatch means the
	// service account was changed outside of the operator.
	if msg, mismatch := scopeMismatch(want, got); mismatch {
		return ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: tcloud service account %q %s",
				ctrlerrors.ErrResourceSync, resourceID, msg,
			),
		)
	}

	// Desired can have refs where latest has their resolved IDs, and the two CRs
	// can differ in other ways Temporal Cloud doesn't care about, so this sync
	// can run even when nothing has changed. We compare the protos that would be
	// sent to Temporal Cloud and return early when the desired and observed ones
	// match, so we don't send an update we don't need.
	if proto.Equal(want, got) {
		return nil
	}

	_, err = tcc.UpdateServiceAccount(ctx, &tcloudservicev1.UpdateServiceAccountRequest{
		ServiceAccountId: resourceID,
		Spec:             want,
		ResourceVersion:  latest.Status.Resource.Version,
	})
	if err != nil {
		return common.CloudCallError("updating tcloud service account", err)
	}
	return requeue.NeededAfter(
		fmt.Errorf(
			"updated tcloud service account %q, waiting for update to complete",
			resourceID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
}

// checkReferences checks the refs in the spec before anything is sent to
// Temporal Cloud.
func checkReferences(
	cr *tcloudv1alpha1.ServiceAccount,
	refs tcloudv1alpha1.ServiceAccountReferences,
) error {
	check := common.NewReferenceCheck(cr.Namespace)
	for _, nr := range namespaceRefs(cr.Spec) {
		check.Pending(nr.path, nr.refOrID, refs.Namespaces[nr.refOrID.Ref.Key(cr.Namespace)])
	}

	// TODO(tomba): support Project and custom role refs once their controllers exist.
	spec := cr.Spec
	switch {
	case spec.AccountScopedAccess != nil:
		a := spec.AccountScopedAccess
		for i, e := range a.ProjectAccesses {
			check.Unsupported(fmt.Sprintf("spec.accountScopedAccess.projectAccesses[%d].project", i), e.Project)
		}
		if a.AccountAccess != nil {
			for i, customRole := range a.AccountAccess.CustomRoles {
				check.Unsupported(fmt.Sprintf("spec.accountScopedAccess.accountAccess.customRoles[%d]", i), customRole)
			}
		}
	case spec.ProjectScopedAccess != nil:
		check.Unsupported("spec.projectScopedAccess.project", spec.ProjectScopedAccess.Project)
	}
	return check.Err()
}

// scopeMismatch reports whether the scope or target Temporal Cloud reports
// differs from the desired one, and describes how.
func scopeMismatch(want, got *tcloudidentityv1.ServiceAccountSpec) (string, bool) {
	if scopeOf(want) != scopeOf(got) {
		return fmt.Sprintf("is %s but the spec is %s", scopeOf(got), scopeOf(want)), true
	}
	if w, g := want.GetNamespaceScopedAccess().GetNamespace(),
		got.GetNamespaceScopedAccess().GetNamespace(); w != g {
		return fmt.Sprintf(
			"is scoped to namespace %q but spec.namespaceScopedAccess.namespace is %q", g, w,
		), true
	}
	if w, g := want.GetProjectScopedAccess().GetProjectId(),
		got.GetProjectScopedAccess().GetProjectId(); w != g {
		return fmt.Sprintf(
			"is scoped to project %q but spec.projectScopedAccess.project is %q", g, w,
		), true
	}
	return "", false
}

// scopeOf describes the scope of a Temporal Cloud ServiceAccountSpec.
func scopeOf(spec *tcloudidentityv1.ServiceAccountSpec) string {
	switch {
	case spec.GetNamespaceScopedAccess() != nil:
		return "namespace scoped"
	case spec.GetProjectScopedAccess() != nil:
		return "project scoped"
	default:
		return "account scoped"
	}
}
