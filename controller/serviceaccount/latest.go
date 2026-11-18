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
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/resource/reference"
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

	getServiceAccount := func(id string) (*tcloudidentityv1.ServiceAccount, error) {
		resp, err := tcc.GetServiceAccount(ctx, &tcloudservicev1.GetServiceAccountRequest{
			ServiceAccountId: id,
		})
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return resp.GetServiceAccount(), nil
	}

	var tsa *tcloudidentityv1.ServiceAccount
	switch {
	case latest.PreviouslyReconciled():
		// The service account may have been deleted by another entity or user
		// (not the operator). We treat it the same as if it was never created
		// and make sure ResourceSync recreates it in this pass.
		tsa, err = getServiceAccount(latest.ResourceID())
		if err != nil {
			return latest, fmt.Errorf("failed getting tcloud service account: %w", err)
		}
	case latest.AdoptionPolicy() == cloudcommon.AdoptionPolicyAdopt:
		resourceID := latest.ResourceID()
		tsa, err = getServiceAccount(resourceID)
		if err != nil {
			return latest, fmt.Errorf("failed getting tcloud service account %q to adopt: %w", resourceID, err)
		}
		// There's nothing to adopt. Without an error the controller would see
		// no difference from the annotations-only CR and report success.
		if tsa == nil {
			err = fmt.Errorf("no tcloud service account with ID %q to adopt", resourceID)
			return latest, err
		}
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

// resolveReferences returns the Temporal Cloud IDs of the Namespace and
// Project custom resources that the CR's spec refers to.
func (r *Reconciler) resolveReferences(
	ctx context.Context,
	cr *tcloudv1alpha1.ServiceAccount,
) (tcloudv1alpha1.ServiceAccountReferences, error) {
	var out tcloudv1alpha1.ServiceAccountReferences
	var err error
	out.Namespaces, err = resolve(
		ctx, r.KubeReader(), cr.Namespace, namespaceRefs(cr.Spec),
		reference.ResolveNamespaceID,
	)
	if err != nil {
		return out, err
	}
	out.Projects, err = resolve(
		ctx, r.KubeReader(), cr.Namespace, projectRefs(cr.Spec),
		reference.ResolveProjectID,
	)
	return out, err
}

// resolve returns a resolved reference for each custom resource in refs that
// resolveID reports ready. Refs that aren't ready yet are left out.
func resolve(
	ctx context.Context,
	kr client.Reader,
	referrerNamespace string,
	refs []specRef,
	resolveID func(
		context.Context, client.Reader, string, cloudcommon.ResourceReferenceOrID,
	) (string, error),
) ([]cloudcommon.ResolvedReference, error) {
	var out []cloudcommon.ResolvedReference
	// The spec might reference the same custom resource more than once by
	// mistake. However, it is only listed once in the resolved references.
	seen := map[client.ObjectKey]bool{}
	for _, sr := range refs {
		key := client.ObjectKey{Namespace: sr.refOrID.Ref.Namespace, Name: sr.refOrID.Ref.Name}
		if key.Namespace == "" {
			key.Namespace = referrerNamespace
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		id, err := resolveID(ctx, kr, referrerNamespace, sr.refOrID)
		if errors.Is(err, reference.ErrNotReady) {
			continue
		}
		if err != nil {
			return out, err
		}
		out = append(out, cloudcommon.ResolvedReference{
			Namespace: key.Namespace, Name: key.Name, ID: id,
		})
	}
	return out, nil
}

// specRef is a ref in a ServiceAccount spec, with its path.
type specRef struct {
	path    string
	refOrID cloudcommon.ResourceReferenceOrID
}

// namespaceRefs returns every Namespace ref in the supplied spec, together
// with where it sits in the spec, for example
// spec.accountScopedAccess.namespaceAccesses[1].namespace.
func namespaceRefs(spec tcloudv1alpha1.ServiceAccountSpec) []specRef {
	var out []specRef
	add := func(path string, refOrID cloudcommon.ResourceReferenceOrID) {
		if refOrID.Ref != nil {
			out = append(out, specRef{path: path, refOrID: refOrID})
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

// projectRefs returns every Project ref in the supplied spec, together with
// where it sits in the spec, for example
// spec.accountScopedAccess.projectAccesses[1].project.
func projectRefs(spec tcloudv1alpha1.ServiceAccountSpec) []specRef {
	var out []specRef
	add := func(path string, refOrID cloudcommon.ResourceReferenceOrID) {
		if refOrID.Ref != nil {
			out = append(out, specRef{path: path, refOrID: refOrID})
		}
	}

	switch {
	case spec.AccountScopedAccess != nil:
		for i, e := range spec.AccountScopedAccess.ProjectAccesses {
			add(fmt.Sprintf("spec.accountScopedAccess.projectAccesses[%d].project", i), e.Project)
		}
	case spec.ProjectScopedAccess != nil:
		add("spec.projectScopedAccess.project", spec.ProjectScopedAccess.Project)
	}
	return out
}
