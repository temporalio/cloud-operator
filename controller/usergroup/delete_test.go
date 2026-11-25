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
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/usergroup"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const deleteOpID = "op-delete"

// cloudUserGroup makes GetUserGroup return groupID in the supplied state, at
// resource version rv-2.
func cloudUserGroup(state tcloudresourcev1.ResourceState) fakeserver.NewOption {
	return fakeserver.OnGetUserGroupReturn(&tcloudservicev1.GetUserGroupResponse{
		Group: &tcloudidentityv1.UserGroup{Id: groupID, ResourceVersion: "rv-2", State: state},
	}, nil)
}

// recordDelete configures the fake DeleteUserGroup call and records its
// request.
func recordDelete() (*atomic.Pointer[tcloudservicev1.DeleteUserGroupRequest], fakeserver.NewOption) {
	var got atomic.Pointer[tcloudservicev1.DeleteUserGroupRequest]
	return &got, fakeserver.WithDeleteUserGroup(func(
		_ context.Context, req *tcloudservicev1.DeleteUserGroupRequest,
	) (*tcloudservicev1.DeleteUserGroupResponse, error) {
		got.Store(req)
		return &tcloudservicev1.DeleteUserGroupResponse{
			AsyncOperation: &operationv1.AsyncOperation{Id: deleteOpID},
		}, nil
	})
}

// runDelete checks that ResourceDelete leaves its argument unchanged.
func runDelete(
	t *testing.T,
	cr *tcloudv1alpha1.UserGroup,
	serverOpts ...fakeserver.NewOption,
) (kttypes.Resource, error) {
	t.Helper()
	r := newReconciler(t, nil, serverOpts...)
	original := usergroup.NewResource(cr.DeepCopy())
	before := original.DeepCopy()

	got, err := r.ResourceDelete(context.TODO(), original)
	require.Equal(t, before.CR(), original.CR(), "original must not be mutated")
	require.NotNil(t, got)
	return got, err
}

func TestReconciler_ResourceDelete(t *testing.T) {
	retain := linkedUserGroup(cloudcommon.ResourceStateActive, "")
	retain.Annotations = map[string]string{
		cloudcommon.AnnotationKeyDeletionPolicy: string(cloudcommon.DeletionPolicyRetain),
	}
	active := linkedUserGroup(cloudcommon.ResourceStateActive, "")
	deleting := linkedUserGroup(cloudcommon.ResourceStateDeleting, deleteOpID)
	// creating is a user group the operator created but that GetUserGroup
	// hasn't returned yet.
	creating := linkedUserGroup("", createOpID)

	cases := []struct {
		name    string
		cr      *tcloudv1alpha1.UserGroup
		opState operationv1.AsyncOperation_State
		// When getState and getErr are both zero, GetUserGroup is unset, so
		// calling it fails the case.
		getState    tcloudresourcev1.ResourceState
		getErr      error
		expErrKind  string
		expErr      string
		expDeleted  bool
		expResource cloudcommon.ResourceStatus
	}{
		{
			name:        "retain policy yields the group kept",
			cr:          retain,
			expResource: retain.Status.Resource,
		},
		{
			name:        "running earlier delete yields a wait",
			cr:          deleting,
			opState:     operationv1.AsyncOperation_STATE_IN_PROGRESS,
			expErrKind:  kindRequeue,
			expResource: deleting.Status.Resource,
		},
		{
			// Deleting the CR before the create finishes would orphan the Cloud group.
			name:        "running create yields a wait before deleting",
			cr:          creating,
			opState:     operationv1.AsyncOperation_STATE_IN_PROGRESS,
			expErrKind:  kindRequeue,
			expErr:      `waiting for tcloud user group "group-1" to be created`,
			expResource: creating.Status.Resource,
		},
		{
			name:        "group temporal cloud no longer has yields done",
			cr:          active,
			getErr:      status.Error(codes.NotFound, "user group not found"),
			expResource: active.Status.Resource,
		},
		{
			name:        "active group yields a delete with the current resource version",
			cr:          active,
			getState:    tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
			expErrKind:  kindRequeue,
			expErr:      "deleting tcloud user group group-1 (async operation op-delete)",
			expDeleted:  true,
			expResource: linkedUserGroup(cloudcommon.ResourceStateDeleting, deleteOpID).Status.Resource,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			deleted, onDelete := recordDelete()
			serverOpts := []fakeserver.NewOption{onDelete}
			if tc.opState != operationv1.AsyncOperation_STATE_UNSPECIFIED {
				serverOpts = append(serverOpts, asyncOperation(tc.cr.Status.Resource.AsyncOperationID, tc.opState))
			}
			switch {
			case tc.getErr != nil:
				serverOpts = append(serverOpts, fakeserver.OnGetUserGroupReturn(nil, tc.getErr))
			case tc.getState != tcloudresourcev1.ResourceState_RESOURCE_STATE_UNSPECIFIED:
				serverOpts = append(serverOpts, cloudUserGroup(tc.getState))
			}

			got, err := runDelete(tt, tc.cr, serverOpts...)

			requireErrKind(tt, tc.expErrKind, err)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
			}
			if tc.expDeleted {
				req := deleted.Load()
				require.NotNil(req)
				require.Equal(groupID, req.GetGroupId())
				require.Equal("rv-2", req.GetResourceVersion())
			} else {
				require.Nil(deleted.Load())
			}
			require.Equal(tc.expResource, statusOf(got))
		})
	}
}
