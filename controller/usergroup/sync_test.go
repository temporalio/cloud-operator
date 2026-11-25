package usergroup_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"google.golang.org/protobuf/proto"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/usergroup"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const (
	createOpID = "op-create"
	newGroupID = "group-new"
	newOpID    = "op-new"
)

// fixtures shared by the ResourceSync tests.
var (
	desired = newUserGroup()

	// notCreated is latest for a user group that doesn't exist in Temporal
	// Cloud yet, with its reference resolved.
	notCreated = func() *tcloudv1alpha1.UserGroup {
		g := &tcloudv1alpha1.UserGroup{}
		g.ObjectMeta = desired.ObjectMeta
		g.Status.References = resolvedRefs(nsID)
		return g
	}()

	// pendingCreate is latest for a user group the operator created but that
	// GetUserGroup hasn't returned yet.
	pendingCreate = func() *tcloudv1alpha1.UserGroup {
		g := notCreated.DeepCopy()
		g.Status.Resource = cloudcommon.ResourceStatus{ID: groupID, AsyncOperationID: createOpID}
		return g
	}()

	// seenBefore is latest for a user group that GetUserGroup returned before
	// but no longer finds.
	seenBefore = func() *tcloudv1alpha1.UserGroup {
		g := notCreated.DeepCopy()
		g.Status.Resource = cloudcommon.ResourceStatus{ID: groupID}
		return g
	}()

	// active is latest for a user group that matches desired.
	active = func() *tcloudv1alpha1.UserGroup {
		g := desired.DeepCopy()
		g.Spec = tcloudv1alpha1.UserGroupSpec{}.FromProto(cloudSpec())
		g.Status.Resource = cloudcommon.ResourceStatus{
			ID: groupID, Version: "rv-1", State: cloudcommon.ResourceStateActive,
		}
		g.Status.References = resolvedRefs(nsID)
		return g
	}()
)

// runSync checks that ResourceSync doesn't modify desired or latest.
func runSync(
	t *testing.T,
	desired *tcloudv1alpha1.UserGroup,
	latest *tcloudv1alpha1.UserGroup,
	serverOpts ...fakeserver.NewOption,
) (kttypes.Resource, error) {
	t.Helper()
	r := newReconciler(t, nil, serverOpts...)
	desiredRes := usergroup.NewResource(desired.DeepCopy())
	latestRes := usergroup.NewResource(latest.DeepCopy())
	desiredBefore := desiredRes.DeepCopy()
	latestBefore := latestRes.DeepCopy()

	got, err := r.ResourceSync(context.TODO(), desiredRes, latestRes)
	require.Equal(t, desiredBefore.CR(), desiredRes.CR(), "desired must not be mutated")
	require.Equal(t, latestBefore.CR(), latestRes.CR(), "latest must not be mutated")
	require.NotNil(t, got)
	return got, err
}

// recordCreate records the CreateUserGroup request so tests can check whether
// it was called.
func recordCreate() (*atomic.Pointer[tcloudservicev1.CreateUserGroupRequest], fakeserver.NewOption) {
	var got atomic.Pointer[tcloudservicev1.CreateUserGroupRequest]
	return &got, fakeserver.WithCreateUserGroup(func(
		_ context.Context, req *tcloudservicev1.CreateUserGroupRequest,
	) (*tcloudservicev1.CreateUserGroupResponse, error) {
		got.Store(req)
		return &tcloudservicev1.CreateUserGroupResponse{
			GroupId:        newGroupID,
			AsyncOperation: &operationv1.AsyncOperation{Id: newOpID},
		}, nil
	})
}

// recordUpdate records the UpdateUserGroup request so tests can check whether
// it was called.
func recordUpdate() (*atomic.Pointer[tcloudservicev1.UpdateUserGroupRequest], fakeserver.NewOption) {
	var got atomic.Pointer[tcloudservicev1.UpdateUserGroupRequest]
	return &got, fakeserver.WithUpdateUserGroup(func(
		_ context.Context, req *tcloudservicev1.UpdateUserGroupRequest,
	) (*tcloudservicev1.UpdateUserGroupResponse, error) {
		got.Store(req)
		return &tcloudservicev1.UpdateUserGroupResponse{}, nil
	})
}

func TestReconciler_ResourceSync_Create(t *testing.T) {
	withOrders := desired.DeepCopy()
	withOrders.Spec.Access.ProjectAccesses = []tcloudv1alpha1.ProjectAccessEntry{{
		Project: ref(projectName),
		Access:  tcloudv1alpha1.ProjectAccess{Role: tcloudv1alpha1.ProjectRoleWrite},
	}}
	ordersResolved := notCreated.DeepCopy()
	ordersResolved.Status.References.Projects = []cloudcommon.ResolvedReference{
		{Namespace: kubeNamespace, Name: projectName, ID: projectID},
	}
	withOrdersSpec := cloudSpec()
	withOrdersSpec.Access.ProjectAccesses = map[string]*tcloudidentityv1.ProjectAccess{
		projectID: {Role: tcloudidentityv1.ProjectAccess_PROJECT_ROLE_WRITE},
	}

	unresolved := notCreated.DeepCopy()
	unresolved.Status.References = tcloudv1alpha1.UserGroupReferences{}

	cases := []struct {
		name          string
		desired       *tcloudv1alpha1.UserGroup
		latest        *tcloudv1alpha1.UserGroup
		expErr        string
		expCreateSpec *tcloudidentityv1.UserGroupSpec
		expResource   cloudcommon.ResourceStatus
	}{
		{
			name:          "resolved namespace and project refs yield their ids in the create",
			desired:       withOrders,
			latest:        ordersResolved,
			expErr:        `created tcloud user group "group-new"`,
			expCreateSpec: withOrdersSpec,
			expResource:   cloudcommon.ResourceStatus{ID: newGroupID, AsyncOperationID: newOpID},
		},
		{
			name:    "unresolved namespace ref yields a wait without a create",
			desired: desired,
			latest:  unresolved,
			expErr:  "spec.access.namespaceAccesses[0].namespace (apps/payments)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			created, onCreate := recordCreate()

			got, err := runSync(tt, tc.desired, tc.latest, onCreate)

			requireRequeue(tt, err)
			require.ErrorContains(err, tc.expErr)
			if tc.expCreateSpec == nil {
				require.Nil(created.Load())
			} else {
				require.NotNil(created.Load())
				require.True(proto.Equal(tc.expCreateSpec, created.Load().GetSpec()), "%v", created.Load().GetSpec())
			}
			require.Equal(tc.expResource, statusOf(got))
		})
	}
}

// After a create, GetUserGroup returns NotFound until Temporal Cloud has
// written the user group. ResourceSync checks the create's async operation
// instead of creating the group again.
func TestReconciler_ResourceSync_NotFound(t *testing.T) {
	cases := []struct {
		name   string
		latest *tcloudv1alpha1.UserGroup
		// A zero opState leaves GetAsyncOperation unset.
		opState     operationv1.AsyncOperation_State
		expErrKind  string
		expErr      []string
		expCreated  bool
		expResource cloudcommon.ResourceStatus
	}{
		{
			name:        "fail if the create failed",
			latest:      pendingCreate,
			opState:     operationv1.AsyncOperation_STATE_FAILED,
			expErrKind:  kindTerminal,
			expErr:      []string{`creating tcloud user group "group-1" failed`, "nope"},
			expResource: pendingCreate.Status.Resource,
		},
		{
			name:        "running create yields a wait without a second create",
			latest:      pendingCreate,
			opState:     operationv1.AsyncOperation_STATE_IN_PROGRESS,
			expErrKind:  kindRequeue,
			expErr:      []string{`waiting for tcloud user group "group-1" to be created`},
			expResource: pendingCreate.Status.Resource,
		},
		{
			name:        "missing group seen before yields a new create",
			latest:      seenBefore,
			expErrKind:  kindRequeue,
			expErr:      []string{`created tcloud user group "group-new"`},
			expCreated:  true,
			expResource: cloudcommon.ResourceStatus{ID: newGroupID, AsyncOperationID: newOpID},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			created, onCreate := recordCreate()
			serverOpts := []fakeserver.NewOption{onCreate}
			if tc.opState != operationv1.AsyncOperation_STATE_UNSPECIFIED {
				serverOpts = append(serverOpts, asyncOperation(createOpID, tc.opState))
			}

			got, err := runSync(tt, desired, tc.latest, serverOpts...)

			requireErrKind(tt, tc.expErrKind, err)
			for _, msg := range tc.expErr {
				require.ErrorContains(err, msg)
			}
			require.Equal(tc.expCreated, created.Load() != nil)
			require.Equal(tc.expResource, statusOf(got))
		})
	}
}

func TestReconciler_ResourceSync_Update(t *testing.T) {
	roleChanged := desired.DeepCopy()
	roleChanged.Spec.Access.AccountAccess.Role = tcloudv1alpha1.AccountRoleRead
	roleChangedSpec := cloudSpec()
	roleChangedSpec.Access.AccountAccess.Role = tcloudidentityv1.AccountAccess_ROLE_READ

	googleGroup := active.DeepCopy()
	googleGroup.Spec = tcloudv1alpha1.UserGroupSpec{
		DisplayName: groupName,
		GoogleGroup: &tcloudv1alpha1.GoogleGroupSpec{EmailAddress: "oncall@example.com"},
	}

	cases := []struct {
		name       string
		desired    *tcloudv1alpha1.UserGroup
		latest     *tcloudv1alpha1.UserGroup
		expErrKind string
		expErr     string
		expUpdate  *tcloudservicev1.UpdateUserGroupRequest
	}{
		{
			name:       "fail if temporal cloud has a google group for a cloud group spec",
			desired:    desired,
			latest:     googleGroup,
			expErrKind: kindTerminal,
			expErr:     `tcloud user group "group-1" is a Google group but the spec is a cloud group`,
		},
		{
			name:    "matching group yields no update",
			desired: desired,
			latest:  active,
		},
		{
			name:       "changed role yields an update with the resource version",
			desired:    roleChanged,
			latest:     active,
			expErrKind: kindRequeue,
			expErr:     "waiting for update to complete",
			expUpdate: &tcloudservicev1.UpdateUserGroupRequest{
				GroupId: groupID, Spec: roleChangedSpec, ResourceVersion: "rv-1",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			updated, onUpdate := recordUpdate()

			got, err := runSync(tt, tc.desired, tc.latest, onUpdate)

			requireErrKind(tt, tc.expErrKind, err)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
			}
			if tc.expUpdate == nil {
				require.Nil(updated.Load())
			} else {
				// The cloud client sets a random async_operation_id on every
				// request, so only the fields ResourceSync sets are compared.
				req := updated.Load()
				require.NotNil(req)
				require.Equal(tc.expUpdate.GetGroupId(), req.GetGroupId())
				require.Equal(tc.expUpdate.GetResourceVersion(), req.GetResourceVersion())
				require.True(proto.Equal(tc.expUpdate.GetSpec(), req.GetSpec()), "%v", req.GetSpec())
			}
			require.Equal(tc.latest.Status, userGroupOf(got).Status, "update must not change status")
		})
	}
}
