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

	var tcloudGroup *tcloudidentityv1.UserGroup
	if latest.PreviouslyReconciled() {
		var resp *tcloudservicev1.GetUserGroupResponse
		resp, err = tcc.GetUserGroup(ctx, &tcloudservicev1.GetUserGroupRequest{
			GroupId: latest.ResourceID(),
		})
		if status.Code(err) == codes.NotFound {
			// Either the user group was deleted on tcloud or it was
			// just created and the operation has not completed it yet
			// GetUserGroup can't tell these apart, so ResourceSync decides
			err = nil
		} else if err != nil {
			return latest, fmt.Errorf("failed getting tcloud user group: %w", err)
		}
		tcloudGroup = resp.GetGroup()
	}

	if tcloudGroup == nil {
		// The UserGroup does not exist in Temporal Cloud, so we empty the spec and status.
		// We keep the refs because CREATE needs the namespace and project IDs. The ID
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
	*cr = *cr.FromProto(tcloudGroup)
	// GetUserGroupMembers fails for a Google group, and Temporal Cloud keeps
	// the member list only for a cloud group.
	if cr.Spec.CloudGroup != nil {
		cr.Spec.CloudGroup.Members, err = getMembers(ctx, tcc, cr.Status.Resource.ID)
		if err != nil {
			return latest, fmt.Errorf("failed getting tcloud user group members: %w", err)
		}
	}
	return latest, nil
}

// getMembers returns the ID of each User in the Temporal Cloud user group.
func getMembers(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	groupID string,
) ([]cloudcommon.ResourceReferenceOrID, error) {
	var out []cloudcommon.ResourceReferenceOrID
	req := &tcloudservicev1.GetUserGroupMembersRequest{GroupId: groupID, PageSize: 1000}
	for {
		resp, err := tcc.GetUserGroupMembers(ctx, req)
		if err != nil {
			return nil, err
		}
		for _, m := range resp.GetMembers() {
			out = append(out, cloudcommon.ResourceReferenceOrID{ID: m.GetMemberId().GetUserId()})
		}
		if resp.GetNextPageToken() == "" {
			return out, nil
		}
		req.PageToken = resp.GetNextPageToken()
	}
}

// resolveReferences returns the Temporal Cloud IDs of the Namespace, Project
// and User custom resources that the CR's spec refers to.
func (r *Reconciler) resolveReferences(
	ctx context.Context,
	cr *tcloudv1alpha1.UserGroup,
) (tcloudv1alpha1.UserGroupReferences, error) {
	var out tcloudv1alpha1.UserGroupReferences
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
	if err != nil {
		return out, err
	}
	out.Users, err = resolve(
		ctx, r.KubeReader(), cr.Namespace, userRefs(cr.Spec),
		reference.ResolveUserID,
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

// specRef is a ref in a UserGroup spec, with its path.
type specRef struct {
	path    string
	refOrID cloudcommon.ResourceReferenceOrID
}

// namespaceRefs returns every Namespace ref in the supplied spec, together
// with where it sits in the spec, for example
// spec.access.namespaceAccesses[1].namespace.
func namespaceRefs(spec tcloudv1alpha1.UserGroupSpec) []specRef {
	var out []specRef
	for i, e := range spec.Access.NamespaceAccesses {
		if e.Namespace.Ref != nil {
			out = append(out, specRef{
				path:    fmt.Sprintf("spec.access.namespaceAccesses[%d].namespace", i),
				refOrID: e.Namespace,
			})
		}
	}
	return out
}

// projectRefs returns every Project ref in the supplied spec, together with
// where it sits in the spec, for example
// spec.access.projectAccesses[1].project.
func projectRefs(spec tcloudv1alpha1.UserGroupSpec) []specRef {
	var out []specRef
	for i, e := range spec.Access.ProjectAccesses {
		if e.Project.Ref != nil {
			out = append(out, specRef{
				path:    fmt.Sprintf("spec.access.projectAccesses[%d].project", i),
				refOrID: e.Project,
			})
		}
	}
	return out
}

// userRefs returns every User ref in the supplied spec, together with where
// it sits in the spec, for example spec.cloudGroup.members[1].
func userRefs(spec tcloudv1alpha1.UserGroupSpec) []specRef {
	if spec.CloudGroup == nil {
		return nil
	}
	var out []specRef
	for i, m := range spec.CloudGroup.Members {
		if m.Ref != nil {
			out = append(out, specRef{
				path:    fmt.Sprintf("spec.cloudGroup.members[%d]", i),
				refOrID: m,
			})
		}
	}
	return out
}
