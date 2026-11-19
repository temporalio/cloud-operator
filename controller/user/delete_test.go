package user_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/user"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const deleteOpID = "op-delete"

// cloudUser makes GetUser return userID in the supplied state, at resource
// version rv-2.
func cloudUser(state tcloudresourcev1.ResourceState) fakeserver.NewOption {
	return fakeserver.OnGetUserReturn(&tcloudservicev1.GetUserResponse{
		User: &tcloudidentityv1.User{Id: userID, ResourceVersion: "rv-2", State: state},
	}, nil)
}

// recordDelete configures the fake DeleteUser call and records its request.
func recordDelete(err error) (*atomic.Pointer[tcloudservicev1.DeleteUserRequest], fakeserver.NewOption) {
	var got atomic.Pointer[tcloudservicev1.DeleteUserRequest]
	return &got, fakeserver.WithDeleteUser(func(
		_ context.Context, req *tcloudservicev1.DeleteUserRequest,
	) (*tcloudservicev1.DeleteUserResponse, error) {
		got.Store(req)
		if err != nil {
			return nil, err
		}
		return &tcloudservicev1.DeleteUserResponse{
			AsyncOperation: &operationv1.AsyncOperation{Id: deleteOpID},
		}, nil
	})
}

// runDelete checks that ResourceDelete leaves its argument unchanged.
func runDelete(
	t *testing.T,
	cr *tcloudv1alpha1.User,
	serverOpts ...fakeserver.NewOption,
) (kttypes.Resource, error) {
	t.Helper()
	r := newReconciler(t, nil, serverOpts...)
	original := user.NewResource(cr.DeepCopy())
	before := original.DeepCopy()

	got, err := r.ResourceDelete(context.TODO(), original)
	require.Equal(t, before.CR(), original.CR(), "original must not be mutated")
	return got, err
}

func TestReconciler_ResourceDelete(t *testing.T) {
	t.Run("never created in temporal cloud, so there's nothing to delete", func(t *testing.T) {
		// The fake server has no hooks, so any Temporal Cloud call would fail.
		got, err := runDelete(t, newUser())

		require.NoError(t, err)
		require.Equal(t, cloudcommon.ResourceStatus{}, statusOf(got))
	})

	t.Run("deletion policy is retain, so the user is kept", func(t *testing.T) {
		cr := linkedUser(cloudcommon.ResourceStateActive, "")
		cr.Annotations = map[string]string{cloudcommon.AnnotationKeyDeletionPolicy: "retain"}

		got, err := runDelete(t, cr)

		require.NoError(t, err)
		require.Equal(t, cr.Status.Resource, statusOf(got))
	})

	t.Run("deletion policy is invalid, so it stalls until it's fixed", func(t *testing.T) {
		cr := linkedUser(cloudcommon.ResourceStateActive, "")
		cr.Annotations = map[string]string{cloudcommon.AnnotationKeyDeletionPolicy: "destroy"}

		got, err := runDelete(t, cr)

		requireRequeue(t, err)
		require.ErrorContains(t, err, `invalid deletion policy "destroy"`)
		requireStalled(t, got)
	})

	t.Run("an earlier delete is still running, so it waits", func(t *testing.T) {
		cr := linkedUser(cloudcommon.ResourceStateDeleting, deleteOpID)
		deleted, onDelete := recordDelete(nil)

		got, err := runDelete(t, cr,
			asyncOperation(deleteOpID, operationv1.AsyncOperation_STATE_IN_PROGRESS), onDelete)

		requireRequeue(t, err)
		require.Nil(t, deleted.Load())
		require.Equal(t, cr.Status.Resource, statusOf(got))
	})

	t.Run("an earlier delete finished, so it's done", func(t *testing.T) {
		deleted, onDelete := recordDelete(nil)

		_, err := runDelete(t, linkedUser(cloudcommon.ResourceStateDeleting, deleteOpID),
			asyncOperation(deleteOpID, operationv1.AsyncOperation_STATE_FULFILLED), onDelete)

		require.NoError(t, err)
		require.Nil(t, deleted.Load())
	})

	t.Run("an earlier delete failed, so it stalls and isn't retried", func(t *testing.T) {
		deleted, onDelete := recordDelete(nil)

		got, err := runDelete(t, linkedUser(cloudcommon.ResourceStateDeleting, deleteOpID),
			asyncOperation(deleteOpID, operationv1.AsyncOperation_STATE_FAILED), onDelete)

		requireTerminal(t, err)
		require.ErrorContains(t, err, "deleting tcloud user user-1 failed")
		require.Nil(t, deleted.Load())
		require.Equal(t, cloudcommon.ResourceStateDeleteFailed, statusOf(got).State)
		requireStalled(t, got)
	})

	// GetUser returns NotFound for a user that was deleted or moved to another
	// account. The second case is a CR that ResourceSync already reported as
	// gone, which keeps the ID but no state, and must still let its finalizer
	// go so the CR can be deleted and re-applied.
	notFound := []struct {
		name string
		cr   *tcloudv1alpha1.User
	}{
		{"an active user", linkedUser(cloudcommon.ResourceStateActive, "")},
		{"a user already reported as gone", linkedUser("", "")},
	}
	for _, tc := range notFound {
		t.Run(tc.name+" that temporal cloud no longer has, so it's done", func(t *testing.T) {
			deleted, onDelete := recordDelete(nil)

			got, err := runDelete(t, tc.cr,
				fakeserver.OnGetUserReturn(nil, status.Error(codes.NotFound, "user not found")),
				onDelete)

			require.NoError(t, err)
			require.Nil(t, deleted.Load())
			require.Equal(t, tc.cr.Status.Resource, statusOf(got))
		})
	}

	t.Run("still updating in temporal cloud, so it waits and saves the state", func(t *testing.T) {
		deleted, onDelete := recordDelete(nil)

		got, err := runDelete(t, linkedUser(cloudcommon.ResourceStateActive, ""),
			cloudUser(tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING), onDelete)

		requireRequeue(t, err)
		require.ErrorContains(t, err, "to finish Updating before deleting")
		require.Nil(t, deleted.Load())
		require.Equal(t, cloudcommon.ResourceStateUpdating, statusOf(got).State)
	})

	t.Run("active, so the delete is sent with the current resource version", func(t *testing.T) {
		cr := linkedUser(cloudcommon.ResourceStateActive, "")
		deleted, onDelete := recordDelete(nil)

		got, err := runDelete(t, cr,
			cloudUser(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE), onDelete)

		requireRequeue(t, err)
		require.ErrorContains(t, err, "deleting tcloud user user-1 (async operation op-delete)")
		req := deleted.Load()
		require.NotNil(t, req)
		require.Equal(t, userID, req.GetUserId())
		require.Equal(t, "rv-2", req.GetResourceVersion())
		exp := cr.Status.Resource
		exp.State = cloudcommon.ResourceStateDeleting
		exp.AsyncOperationID = deleteOpID
		require.Equal(t, exp, statusOf(got))
		require.Nil(t, condition.Stalled(got))
	})

	t.Run("deleted by someone else between the read and the delete, so it's done", func(t *testing.T) {
		deleted, onDelete := recordDelete(status.Error(codes.NotFound, "user not found"))

		_, err := runDelete(t, linkedUser(cloudcommon.ResourceStateActive, ""),
			cloudUser(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE), onDelete)

		require.NoError(t, err)
		require.NotNil(t, deleted.Load())
	})

	// Temporal Cloud refuses these deletes with InvalidArgument and retrying
	// doesn't help, so the CR stalls and points at the retain policy.
	refusals := []struct {
		name string
		msg  string
	}{
		{"an account owner", "account owners cannot be deleted"},
		{"a SCIM user", "SCIM users cannot be deleted via API"},
		{"the only account admin", "user is only account admin"},
	}
	for _, tc := range refusals {
		t.Run("temporal cloud refuses to delete "+tc.name+", so it stalls", func(t *testing.T) {
			deleted, onDelete := recordDelete(status.Error(codes.InvalidArgument, tc.msg))

			got, err := runDelete(t, linkedUser(cloudcommon.ResourceStateActive, ""),
				cloudUser(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE), onDelete)

			requireRequeue(t, err)
			require.ErrorContains(t, err, tc.msg)
			require.ErrorContains(t, err, cloudcommon.AnnotationKeyDeletionPolicy)
			require.NotNil(t, deleted.Load())
			requireStalled(t, got)
		})
	}

	t.Run("temporal cloud rejects the delete for another reason, so it stalls and retries", func(t *testing.T) {
		deleted, onDelete := recordDelete(
			status.Error(codes.PermissionDenied, "user lifecycle management is disabled for account"))

		got, err := runDelete(t, linkedUser(cloudcommon.ResourceStateActive, ""),
			cloudUser(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE), onDelete)

		requireRetried(t, err)
		require.ErrorContains(t, err, "failed deleting tcloud user user-1")
		require.NotNil(t, deleted.Load())
		requireStalled(t, got)
	})

	t.Run("temporal cloud can't be read, so it retries", func(t *testing.T) {
		deleted, onDelete := recordDelete(nil)

		got, err := runDelete(t, linkedUser(cloudcommon.ResourceStateActive, ""),
			fakeserver.OnGetUserReturn(nil, status.Error(codes.PermissionDenied, "api key can't read users")),
			onDelete)

		requireRetried(t, err)
		require.ErrorContains(t, err, "failed getting tcloud user user-1")
		require.Nil(t, deleted.Load())
		require.Nil(t, condition.Stalled(got))
	})
}
