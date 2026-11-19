package user_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/user"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const createOpID = "op-create"

// fixtures shared by the ResourceSync tests.
var (
	desired = newUser()

	// notCreated is latest for a user that doesn't exist in Temporal Cloud
	// yet, with its reference resolved.
	notCreated = func() *tcloudv1alpha1.User {
		u := &tcloudv1alpha1.User{}
		u.ObjectMeta = desired.ObjectMeta
		u.Status.References = resolvedRefs(nsID)
		return u
	}()

	// pendingCreate is latest for a user the operator created but that
	// GetUser hasn't returned yet.
	pendingCreate = func() *tcloudv1alpha1.User {
		u := notCreated.DeepCopy()
		u.Status.Resource = cloudcommon.ResourceStatus{ID: userID, AsyncOperationID: createOpID}
		return u
	}()

	// active is latest for a user that matches desired.
	active = func() *tcloudv1alpha1.User {
		u := desired.DeepCopy()
		u.Spec = tcloudv1alpha1.UserSpec{}.FromProto(cloudSpec())
		u.Status.Resource = cloudcommon.ResourceStatus{
			ID: userID, Version: "rv-1", State: cloudcommon.ResourceStateActive,
		}
		u.Status.References = resolvedRefs(nsID)
		return u
	}()
)

// runSync checks that ResourceSync doesn't modify desired or latest.
func runSync(
	t *testing.T,
	desired *tcloudv1alpha1.User,
	latest *tcloudv1alpha1.User,
	serverOpts ...fakeserver.NewOption,
) (kttypes.Resource, error) {
	t.Helper()
	r := newReconciler(t, nil, serverOpts...)
	desiredRes := user.NewResource(desired.DeepCopy())
	latestRes := user.NewResource(latest.DeepCopy())
	desiredBefore := desiredRes.DeepCopy()
	latestBefore := latestRes.DeepCopy()

	got, err := r.ResourceSync(context.TODO(), desiredRes, latestRes)
	require.Equal(t, desiredBefore.CR(), desiredRes.CR(), "desired must not be mutated")
	require.Equal(t, latestBefore.CR(), latestRes.CR(), "latest must not be mutated")
	require.NotNil(t, got)
	return got, err
}

// recordCreate records the CreateUser request so tests can check whether it was called.
func recordCreate(err error) (*atomic.Pointer[tcloudservicev1.CreateUserRequest], fakeserver.NewOption) {
	var got atomic.Pointer[tcloudservicev1.CreateUserRequest]
	return &got, fakeserver.WithCreateUser(func(
		_ context.Context, req *tcloudservicev1.CreateUserRequest,
	) (*tcloudservicev1.CreateUserResponse, error) {
		got.Store(req)
		if err != nil {
			return nil, err
		}
		return &tcloudservicev1.CreateUserResponse{
			UserId:         "user-new",
			AsyncOperation: &operationv1.AsyncOperation{Id: createOpID},
		}, nil
	})
}

// recordUpdate records the UpdateUser request so tests can check whether it was called.
func recordUpdate(err error) (*atomic.Pointer[tcloudservicev1.UpdateUserRequest], fakeserver.NewOption) {
	var got atomic.Pointer[tcloudservicev1.UpdateUserRequest]
	return &got, fakeserver.WithUpdateUser(func(
		_ context.Context, req *tcloudservicev1.UpdateUserRequest,
	) (*tcloudservicev1.UpdateUserResponse, error) {
		got.Store(req)
		return &tcloudservicev1.UpdateUserResponse{}, err
	})
}

func TestReconciler_ResourceSync_Create(t *testing.T) {
	t.Run("not created yet, so it's created and the id and operation are saved", func(t *testing.T) {
		created, onCreate := recordCreate(nil)

		got, err := runSync(t, desired, notCreated, onCreate)

		requireRequeue(t, err)
		require.ErrorContains(t, err, `created tcloud user "user-new"`)
		require.NotNil(t, created.Load())
		require.True(t, proto.Equal(cloudSpec(), created.Load().GetSpec()), "%v", created.Load().GetSpec())
		require.Equal(t, cloudcommon.ResourceStatus{
			ID: "user-new", AsyncOperationID: createOpID,
		}, statusOf(got))
	})

	t.Run("temporal cloud rejects the create, so it fails and saves no id", func(t *testing.T) {
		created, onCreate := recordCreate(status.Error(codes.InvalidArgument, "invalid email address format"))

		got, err := runSync(t, desired, notCreated, onCreate)

		requireTerminal(t, err)
		require.ErrorContains(t, err, "failed creating tcloud user")
		require.NotNil(t, created.Load())
		require.Equal(t, cloudcommon.ResourceStatus{}, statusOf(got))
	})

	t.Run("a namespace ref has no id yet, so it waits without calling temporal cloud", func(t *testing.T) {
		unresolved := notCreated.DeepCopy()
		unresolved.Status.References = tcloudv1alpha1.UserReferences{}
		created, onCreate := recordCreate(nil)

		_, err := runSync(t, desired, unresolved, onCreate)

		requireRequeue(t, err)
		require.ErrorContains(t, err, "spec.access.namespaceAccesses[0].namespace (apps/payments)")
		require.Nil(t, created.Load())
	})

	t.Run("project refs aren't supported yet, so it fails", func(t *testing.T) {
		projectRef := desired.DeepCopy()
		projectRef.Spec.Access.ProjectAccesses = []tcloudv1alpha1.ProjectAccessEntry{{
			Project: ref("payments"),
			Access:  tcloudv1alpha1.ProjectAccess{Role: tcloudv1alpha1.ProjectRoleWrite},
		}}
		created, onCreate := recordCreate(nil)

		_, err := runSync(t, projectRef, notCreated, onCreate)

		requireTerminal(t, err)
		require.ErrorContains(t, err, "spec.access.projectAccesses[0].project")
		require.Nil(t, created.Load())
	})

	t.Run("a ref and an id for the same namespace, so it fails", func(t *testing.T) {
		duplicate := desired.DeepCopy()
		duplicate.Spec.Access.NamespaceAccesses = append(duplicate.Spec.Access.NamespaceAccesses,
			tcloudv1alpha1.NamespaceScopedAccess{
				Namespace: id(nsID),
				Access:    tcloudv1alpha1.NamespaceAccess{Permission: tcloudv1alpha1.NamespacePermissionRead},
			})
		created, onCreate := recordCreate(nil)

		_, err := runSync(t, duplicate, notCreated, onCreate)

		requireTerminal(t, err)
		require.ErrorContains(t, err, `both refer to tcloud namespace "payments.acct1"`)
		require.Nil(t, created.Load())
	})
}

// After a create, GetUser can return NotFound until Temporal Cloud has written
// the user. ResourceSync checks the create's async operation instead of
// creating the user again.
func TestReconciler_ResourceSync_NotFound(t *testing.T) {
	t.Run("the create is still running, so it waits without creating again", func(t *testing.T) {
		created, onCreate := recordCreate(nil)

		got, err := runSync(t, desired, pendingCreate,
			asyncOperation(createOpID, operationv1.AsyncOperation_STATE_IN_PROGRESS), onCreate)

		requireRequeue(t, err)
		require.ErrorContains(t, err, `waiting for tcloud user "user-1" to be created`)
		require.Nil(t, created.Load())
		require.Equal(t, pendingCreate.Status.Resource, statusOf(got))
	})

	t.Run("the create failed, so it fails with temporal cloud's reason", func(t *testing.T) {
		created, onCreate := recordCreate(nil)

		_, err := runSync(t, desired, pendingCreate,
			asyncOperation(createOpID, operationv1.AsyncOperation_STATE_FAILED), onCreate)

		requireTerminal(t, err)
		require.ErrorContains(t, err, `creating tcloud user "user-1" failed`)
		require.ErrorContains(t, err, "nope")
		require.Nil(t, created.Load())
	})

	// The user isn't recreated, because that could re-invite someone an admin
	// removed or move them back from another account.
	gone := []struct {
		name       string
		latest     *tcloudv1alpha1.User
		serverOpts []fakeserver.NewOption
	}{
		{
			name:       "the create finished but the user is missing",
			latest:     pendingCreate,
			serverOpts: []fakeserver.NewOption{asyncOperation(createOpID, operationv1.AsyncOperation_STATE_FULFILLED)},
		},
		{
			name: "a user seen before is missing",
			latest: func() *tcloudv1alpha1.User {
				u := notCreated.DeepCopy()
				u.Status.Resource = cloudcommon.ResourceStatus{ID: userID}
				return u
			}(),
		},
	}
	for _, tc := range gone {
		t.Run(tc.name+", so it fails without creating again", func(t *testing.T) {
			created, onCreate := recordCreate(nil)

			_, err := runSync(t, desired, tc.latest, append(tc.serverOpts, onCreate)...)

			requireTerminal(t, err)
			require.ErrorContains(t, err, `tcloud user "user-1" no longer exists in this account`)
			require.Nil(t, created.Load())
		})
	}
}

func TestReconciler_ResourceSync_Update(t *testing.T) {
	roleChanged := desired.DeepCopy()
	roleChanged.Spec.Access.AccountAccess.Role = tcloudv1alpha1.AccountRoleRead

	t.Run("temporal cloud already matches, so nothing is sent", func(t *testing.T) {
		updated, onUpdate := recordUpdate(nil)

		got, err := runSync(t, desired, active, onUpdate)

		require.NoError(t, err)
		require.Nil(t, updated.Load())
		require.Equal(t, active.Status, userOf(got).Status)
	})

	t.Run("the role changed, so an update is sent with the resource version", func(t *testing.T) {
		updated, onUpdate := recordUpdate(nil)

		got, err := runSync(t, roleChanged, active, onUpdate)

		requireRequeue(t, err)
		require.ErrorContains(t, err, "waiting for update to complete")
		req := updated.Load()
		require.NotNil(t, req)
		require.Equal(t, userID, req.GetUserId())
		require.Equal(t, "rv-1", req.GetResourceVersion())
		exp := cloudSpec()
		exp.Access.AccountAccess.Role = tcloudidentityv1.AccountAccess_ROLE_READ
		require.True(t, proto.Equal(exp, req.GetSpec()), "%v", req.GetSpec())
		require.Equal(t, active.Status, userOf(got).Status, "update must not change status")
	})

	t.Run("still activating, so it waits before updating", func(t *testing.T) {
		activating := active.DeepCopy()
		activating.Status.Resource.State = cloudcommon.ResourceStateActivating
		updated, onUpdate := recordUpdate(nil)

		_, err := runSync(t, roleChanged, activating, onUpdate)

		requireRequeue(t, err)
		require.ErrorContains(t, err, "to leave state Activating")
		require.Nil(t, updated.Load())
	})

	t.Run("a namespace ref has no id yet, so it waits without calling temporal cloud", func(t *testing.T) {
		unresolved := active.DeepCopy()
		unresolved.Status.References = tcloudv1alpha1.UserReferences{}
		updated, onUpdate := recordUpdate(nil)

		_, err := runSync(t, roleChanged, unresolved, onUpdate)

		requireRequeue(t, err)
		require.ErrorContains(t, err, "spec.access.namespaceAccesses[0].namespace (apps/payments)")
		require.Nil(t, updated.Load())
	})

	t.Run("made an account owner outside the operator, so it fails without updating", func(t *testing.T) {
		owner := active.DeepCopy()
		owner.Spec.Access = tcloudv1alpha1.AccountScopedAccess{
			AccountAccess: &tcloudv1alpha1.AccountAccess{Role: tcloudv1alpha1.AccountRoleOwner},
		}
		updated, onUpdate := recordUpdate(nil)

		_, err := runSync(t, desired, owner, onUpdate)

		requireTerminal(t, err)
		require.ErrorContains(t, err, `tcloud user "user-1" is an account Owner`)
		require.Nil(t, updated.Load())
	})
}
