package usergroup

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
// latest observed state of the Temporal Cloud user group.
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

	cr := latest.CR().(*tcloudv1alpha1.UserGroup)

	refs, err := r.resolveReferences(ctx, cr)
	if err != nil {
		return latest, fmt.Errorf("failed resolving references: %w", err)
	}

	var tcloudUserGroup *tcloudidentityv1.UserGroup
	if latest.PreviouslyReconciled() {
		var resp *tcloudservicev1.GetUserGroupResponse
		resp, err = tcc.GetUserGroup(ctx, &tcloudservicev1.GetUserGroupRequest{
			GroupId: latest.ResourceID(),
		})
		if status.Code(err) == codes.NotFound {
			// Either the user was deleted on tcloud or it was
			// just created and the operation has not completed it yet
			// GetUserGroup can't tell these apart, so ResourceSync decides
			err = nil
		} else if err != nil {
			return latest, fmt.Errorf("failed getting tcloud user group: %w", err)
		}
		tcloudUserGroup = resp.GetGroup()
	}

	if tcloudUserGroup == nil {
		// The UserGroup does not exist in Temporal Cloud, so we empty the spec and status.
		// We keep the refs because CREATE needs the namespace IDs. The ID
		// is kept so ResourceSync doesn't create a second group.
		var resource cloudcommon.ResourceStatus
		resource.ID = cr.Status.Resource.ID
		if cr.Status.Resource.State == "" {
			resource.AsyncOperationID = cr.Status.Resource.AsyncOperationID
		}
		cr.Spec = tcloudv1alpha1.UserGroupSpec{}
		cr.Status = tcloudv1alpha1.UserGroupStatus{Resource: resource, References: refs}
		return latest, nil
	}

	cr.Status.References = refs
	*cr = *cr.FromProto(tcloudUserGroup)
	return latest, nil
}

// resolveReferences returns the Temporal Cloud IDs of the Namespace custom
// resources that the CR's spec refers to.
func (r *Reconciler) resolveReferences(
	ctx context.Context,
	cr *tcloudv1alpha1.UserGroup,
) (tcloudv1alpha1.UserGroupReferences, error) {
	var out tcloudv1alpha1.UserGroupReferences
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
		out.Namespaces = append(out.Namespaces, cloudcommon.ResolvedReference{
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
func namespaceRefs(spec tcloudv1alpha1.UserGroupSpec) []namespaceRef {
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
