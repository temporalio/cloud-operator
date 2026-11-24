package usergroup

import (
	"context"
	"errors"
	"fmt"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/protobuf/proto"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/resource/reference"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// TODO: move to controller.go once it exists.
const resourceKind = "user group"

// ResourceSync creates the Temporal Cloud user group if it doesn't exist yet,
// and otherwise updates it to match `desired`. It returns a copy of `latest`
// with any status changes.
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

	desiredGroup := desired.CR().(*tcloudv1alpha1.UserGroup)
	latestGroup := latest.CR().(*tcloudv1alpha1.UserGroup)

	// Creating or updating the group needs an ID for each ref in the spec.
	if err = checkReferences(desiredGroup, latestGroup.Status.References); err != nil {
		return updated, err
	}

	updatedGroup := updated.CR().(*tcloudv1alpha1.UserGroup)
	switch {
	case !latest.(types.Resource).PreviouslyReconciled():
		err = createUserGroup(ctx, tcc, desiredGroup, updatedGroup)
		return updated, err
	case latestGroup.Status.Resource.State == "":
		// ResourceLatest kept the ID, but GetUserGroup returned NotFound. A
		// create that hasn't finished yet has an operation to wait for.
		// Otherwise the group was deleted outside of the operator, and it is
		// created again like a Service Account would be.
		if opID := latestGroup.Status.Resource.AsyncOperationID; opID != "" {
			if err = checkCreate(ctx, tcc, latestGroup.Status.Resource.ID, opID); err != nil {
				return updated, err
			}
		}
		err = createUserGroup(ctx, tcc, desiredGroup, updatedGroup)
		return updated, err
	}

	err = updateUserGroup(ctx, tcc, desiredGroup, latestGroup)
	return updated, err
}

// createUserGroup creates the Temporal Cloud user group for the supplied
// `desired` CR and saves its ID and async operation ID in `updated`.
//
// `desired` is NEVER mutated.
func createUserGroup(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.UserGroup,
	updated *tcloudv1alpha1.UserGroup, // mutated by this function
) error {
	resp, err := tcc.CreateUserGroup(ctx, &tcloudservicev1.CreateUserGroupRequest{
		Spec: desired.Spec.ToProto(desired.Namespace, updated.Status.References),
	})
	if err != nil {
		return cloud.CallError("creating tcloud user group", err)
	}

	// GetUserGroup returns NotFound until the create finishes. The operation ID
	// lets the next reconcile wait instead of creating a second group.
	updated.Status.Resource.ID = resp.GetGroupId()
	updated.Status.Resource.AsyncOperationID = resp.GetAsyncOperation().GetId()
	return requeue.NeededAfter(
		fmt.Errorf("created tcloud user group %q, waiting for the group to be created", resp.GetGroupId()),
		requeue.DefaultRequeueAfterDuration,
	)
}

// checkCreate checks the async operation that createUserGroup saved for a
// group that GetUserGroup didn't find. It returns nil once the create has
// finished, which means the group was deleted afterwards.
func checkCreate(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	groupID string,
	opID string,
) error {
	err := cloud.CheckAsyncOperation(ctx, tcc, opID)
	if errors.Is(err, cloud.ErrAsyncOperationFailed) {
		return fmt.Errorf("creating tcloud user group %q failed: %w", groupID, err)
	}
	var pending *requeue.RequeueNeededAfter
	if errors.As(err, &pending) {
		return requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud user group %q to be created (async operation %s)",
				groupID, opID,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}
	return err
}

// updateUserGroup makes at most one Temporal Cloud call to move an existing
// user group toward the desired state. An update starts an async operation,
// so after it we requeue and let the next reconcile observe the result.
func updateUserGroup(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.UserGroup,
	latest *tcloudv1alpha1.UserGroup,
) error {
	resourceID := latest.Status.Resource.ID
	err := cloud.RequireActive(resourceKind, resourceID, latest.Status.Resource.State)
	if err != nil {
		return err
	}

	want := desired.Spec.ToProto(desired.Namespace, latest.Status.References)
	// latest's spec comes from Temporal Cloud, so every reference in it is
	// already an ID.
	var none tcloudv1alpha1.UserGroupReferences
	got := latest.Spec.ToProto(latest.Namespace, none)

	// The CR can't change its group type (CEL), and Temporal Cloud rejects
	// changing it, so a mismatch means the group isn't the one the spec
	// describes.
	if msg, mismatch := groupTypeMismatch(want, got); mismatch {
		return ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: tcloud user group %q %s",
				ctrlerrors.ErrResourceSync, resourceID, msg,
			),
		)
	}

	// Desired can have refs where latest has their resolved IDs, so compare the
	// protos that would be sent to Temporal Cloud. Temporal Cloud rejects an
	// update that changes nothing.
	if proto.Equal(want, got) {
		return nil
	}

	_, err = tcc.UpdateUserGroup(ctx, &tcloudservicev1.UpdateUserGroupRequest{
		GroupId:         resourceID,
		Spec:            want,
		ResourceVersion: latest.Status.Resource.Version,
	})
	if err != nil {
		return cloud.CallError("updating tcloud user group", err)
	}
	return requeue.NeededAfter(
		fmt.Errorf("updated tcloud user group %q, waiting for update to complete", resourceID),
		requeue.DefaultRequeueAfterDuration,
	)
}

// groupTypeMismatch reports whether the group type or Google group email
// Temporal Cloud reports differs from the desired one, and describes how.
func groupTypeMismatch(want, got *tcloudidentityv1.UserGroupSpec) (string, bool) {
	if groupTypeOf(want) != groupTypeOf(got) {
		return fmt.Sprintf("is a %s but the spec is a %s", groupTypeOf(got), groupTypeOf(want)), true
	}
	if w, g := want.GetGoogleGroup().GetEmailAddress(),
		got.GetGoogleGroup().GetEmailAddress(); w != g {
		return fmt.Sprintf(
			"is the Google group %q but spec.googleGroup.emailAddress is %q", g, w,
		), true
	}
	return "", false
}

// groupTypeOf describes the group type of a Temporal Cloud UserGroupSpec.
func groupTypeOf(spec *tcloudidentityv1.UserGroupSpec) string {
	switch {
	case spec.GetGoogleGroup() != nil:
		return "Google group"
	case spec.GetCloudGroup() != nil:
		return "cloud group"
	default:
		// The CRD has no SCIM group type, so FromProto and ToProto leave the
		// type unset for a SCIM group read from Temporal Cloud.
		return "SCIM group"
	}
}

// checkReferences checks the refs in the spec before anything is sent to
// Temporal Cloud.
func checkReferences(
	cr *tcloudv1alpha1.UserGroup,
	refs tcloudv1alpha1.UserGroupReferences,
) error {
	check := reference.NewCheck(cr.Namespace)
	for _, nr := range namespaceRefs(cr.Spec) {
		id, _ := refs.NamespaceID(*nr.refOrID.Ref, cr.Namespace)
		check.Pending(nr.path, nr.refOrID, id)
	}

	access := cr.Spec.Access
	for i, e := range access.ProjectAccesses {
		check.Unsupported(fmt.Sprintf("spec.access.projectAccesses[%d].project", i), e.Project)
	}
	if access.AccountAccess != nil {
		for i, customRole := range access.AccountAccess.CustomRoles {
			check.Unsupported(fmt.Sprintf("spec.access.accountAccess.customRoles[%d]", i), customRole)
		}
	}
	if err := checkDuplicateNamespaces(cr, refs); err != nil {
		return err
	}
	return check.Err()
}

// checkDuplicateNamespaces returns a terminal error when two namespace accesses
// in the spec refer to the same Temporal Cloud namespace.Ex:
// {ref: payments} and {id: payments.acct1}
func checkDuplicateNamespaces(
	cr *tcloudv1alpha1.UserGroup,
	refs tcloudv1alpha1.UserGroupReferences,
) error {
	const path = "spec.access.namespaceAccesses"
	seen := map[string]int{}
	for i, e := range cr.Spec.Access.NamespaceAccesses {
		id := e.Namespace.ID
		if e.Namespace.Ref != nil {
			id, _ = refs.NamespaceID(*e.Namespace.Ref, cr.Namespace)
		}
		if id == "" {
			continue
		}
		if first, ok := seen[id]; ok {
			return ctrlerrors.NewTerminalError(
				fmt.Errorf(
					"%w: %s[%d].namespace and %s[%d].namespace both refer to tcloud namespace %q",
					ctrlerrors.ErrResourceSync, path, first, path, i, id,
				),
			)
		}
		seen[id] = i
	}
	return nil
}
