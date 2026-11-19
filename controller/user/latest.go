package user

import (
	"context"
	"errors"
	"fmt"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/resource/reference"
	"github.com/temporalio/cloud-operator/pkg/types"
	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceLatest returns a copy of the supplied [types.Resource] with the
// latest observed state of the Temporal Cloud User.
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

	cr := latest.CR().(*tcloudv1alpha1.User)

	refs, err := r.resolveReferences(ctx, cr)
	if err != nil {
		return latest, fmt.Errorf("failed resolving references: %w", err)
	}

	var tcloudUser *tcloudidentityv1.User
	if latest.PreviouslyReconciled() {
		var resp *tcloudservicev1.GetUserResponse
		resp, err = tcc.GetUser(ctx, &tcloudservicev1.GetUserRequest{
			UserId: latest.ResourceID(),
		})
		if status.Code(err) == codes.NotFound {
			// Either the user was deleted on tcloud or it was
			// just created and the operation has not completed it yet
			// GetUser can't tell these apart, so ResourceSync decides
			err = nil
		} else if err != nil {
			return latest, fmt.Errorf("failed getting tcloud user: %w", err)
		}
		tcloudUser = resp.GetUser()
	}

	if tcloudUser == nil {
		// The user does not exist in tcloud so we empty the spec and status.
		// We keep the refs because CREATE needs the namespace IDs.
		// We keep the ID whenever one was saved, so ResourceSync doesn't create the user again,
		// which would send a second invite to the user.email.
		// We keep the async operation only if we haven't seen the user since the create request,
		// so ResourceSync can wait for that create instead of checking an older operation.
		var resource cloudcommon.ResourceStatus
		resource.ID = cr.Status.Resource.ID
		if cr.Status.Resource.State == "" {
			resource.AsyncOperationID = cr.Status.Resource.AsyncOperationID
		}
		cr.Spec = tcloudv1alpha1.UserSpec{}
		cr.Status = tcloudv1alpha1.UserStatus{Resource: resource, References: refs}
		return latest, nil
	}
	cr.Status.References = refs
	*cr = *cr.FromProto(tcloudUser)
	return latest, nil
}

func (r *Reconciler) resolveReferences(
	ctx context.Context,
	cr *tcloudv1alpha1.User,
) (tcloudv1alpha1.UserReferences, error) {
	var out tcloudv1alpha1.UserReferences
	seen := map[client.ObjectKey]bool{}
	for _, nr := range namespaceRefs(cr.Spec) {
		key := client.ObjectKey{Namespace: nr.refOrID.Ref.Namespace, Name: nr.refOrID.Ref.Name}
		if key.Namespace == "" {
			key.Namespace = cr.Namespace
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		id, err := reference.ResolveNamespaceID(ctx, r.KubeReader(), cr.Namespace, nr.refOrID)
		if errors.Is(err, reference.ErrNotReady) {
			continue
		}
		if err != nil {
			return out, err
		}
		out.Namespaces = append(out.Namespaces, tcloudv1alpha1.NamespaceResolvedReference{
			Namespace: key.Namespace, Name: key.Name, ID: id,
		})
	}
	return out, nil
}

type namespaceRef struct {
	path    string
	refOrID cloudcommon.ResourceReferenceOrID
}

// namespaceRefs returns every Namespace ref in the supplied spec, together
// with where it sits in the spec, for example
// spec.access.namespaceAccesses[1].namespace.
func namespaceRefs(spec tcloudv1alpha1.UserSpec) []namespaceRef {
	var out []namespaceRef
	for i, e := range spec.Access.NamespaceAccesses {
		if e.Namespace.Ref != nil {
			out = append(out, namespaceRef{
				path:    fmt.Sprintf("spec.access.namespaceAccesses[%d].namespace", i),
				refOrID: e.Namespace,
			})
		}
	}
	return out
}
