package serviceaccount

import (
	"context"
	"errors"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/common"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceLatest returns a copy of the supplied [types.Resource] with the
// latest observed state of the Temporal Cloud service account, and the spec's
// references resolved into Status.References.
//
// `current` is NEVER mutated.
func (r *Reconciler) ResourceLatest(
	ctx context.Context,
	current kttypes.Resource,
) (kttypes.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.latest")
	defer func() {
		exit(err)
	}()

	latest := current.DeepCopy().(types.Resource)

	tcc, err := r.CloudClientFor(ctx, current)
	if err != nil {
		return latest, fmt.Errorf("failed getting tcloud client: %w", err)
	}

	cr := latest.CR().(*tcloudv1alpha1.ServiceAccount)

	refs, err := r.resolveReferences(ctx, cr)
	if err != nil {
		return latest, fmt.Errorf("failed resolving references: %w", err)
	}

	var tsa *tcloudidentityv1.ServiceAccount
	if latest.PreviouslyReconciled() {
		var resp *tcloudservicev1.GetServiceAccountResponse
		resp, err = tcc.GetServiceAccount(ctx, &tcloudservicev1.GetServiceAccountRequest{
			ServiceAccountId: latest.ResourceID(),
		})
		if status.Code(err) == codes.NotFound {
			// The service account was deleted by another entity or user (not the operator).
			// We treat it the same as if it was never created and make sure ResourceSync
			// recreates it in this pass.
			err = nil
		} else if err != nil {
			return latest, fmt.Errorf("failed getting tcloud service account: %w", err)
		}
		tsa = resp.GetServiceAccount()
	}

	if tsa == nil {
		// The service account does not exist in Temporal Cloud, so empty the
		// spec and status, keeping the resolved references for the create.
		cr.Spec = tcloudv1alpha1.ServiceAccountSpec{}
		cr.Status = tcloudv1alpha1.ServiceAccountStatus{References: refs}
		return latest, nil
	}

	cr.Status.References = refs
	*cr = *cr.FromProto(tsa)
	return latest, nil
}

// resolveReferences returns the Temporal Cloud IDs of the Namespace custom
// resources that the CR's spec refers to.
func (r *Reconciler) resolveReferences(
	ctx context.Context,
	cr *tcloudv1alpha1.ServiceAccount,
) (tcloudv1alpha1.ServiceAccountReferences, error) {
	var out tcloudv1alpha1.ServiceAccountReferences
	for _, nr := range namespaceRefs(cr.Spec) {
		key := nr.refOrID.Ref.Key(cr.Namespace)
		if _, ok := out.Namespaces[key]; ok {
			continue
		}
		id, err := common.ResolveNamespaceID(ctx, r.KubeReader(), cr.Namespace, nr.refOrID)
		if err != nil && !errors.Is(err, common.ErrReferenceNotReady) {
			return out, err
		}
		if out.Namespaces == nil {
			out.Namespaces = map[string]string{}
		}
		out.Namespaces[key] = id
	}
	return out, nil
}

// namespaceRef is a Namespace ref in a ServiceAccount spec, with its path.
type namespaceRef struct {
	path    string
	refOrID cloudcommon.ResourceReferenceOrID
}

// namespaceRefs returns every Namespace ref in the supplied spec, together
// with where it sits in the spec, for example
// spec.accountScopedAccess.namespaceAccesses[1].namespace.
func namespaceRefs(spec tcloudv1alpha1.ServiceAccountSpec) []namespaceRef {
	var out []namespaceRef
	add := func(path string, refOrID cloudcommon.ResourceReferenceOrID) {
		if refOrID.Ref != nil {
			out = append(out, namespaceRef{path: path, refOrID: refOrID})
		}
	}
	addEntries := func(path string, entries []tcloudv1alpha1.NamespaceScopedAccess) {
		for i, e := range entries {
			add(fmt.Sprintf("%s[%d].namespace", path, i), e.Namespace)
		}
	}

	switch {
	case spec.AccountScopedAccess != nil:
		addEntries("spec.accountScopedAccess.namespaceAccesses", spec.AccountScopedAccess.NamespaceAccesses)
	case spec.NamespaceScopedAccess != nil:
		add("spec.namespaceScopedAccess.namespace", spec.NamespaceScopedAccess.Namespace)
	case spec.ProjectScopedAccess != nil:
		addEntries("spec.projectScopedAccess.namespaceAccesses", spec.ProjectScopedAccess.NamespaceAccesses)
	}
	return out
}
