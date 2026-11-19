package user_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/user"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

// runLatest calls ResourceLatest and checks that it leaves its argument unchanged,
// since the Controller compares the two to decide whether to sync.
func runLatest(
	t *testing.T,
	cr *tcloudv1alpha1.User,
	kubeObjs []runtime.Object,
	serverOpts ...fakeserver.NewOption,
) (*tcloudv1alpha1.User, error) {
	t.Helper()
	r := newReconciler(t, kubeObjs, serverOpts...)
	current := user.NewResource(cr.DeepCopy())
	before := current.DeepCopy()

	got, err := r.ResourceLatest(context.TODO(), current)
	require.Equal(t, before.CR(), current.CR(), "current must not be mutated")
	require.NotNil(t, got)
	return userOf(got), err
}

func TestReconciler_ResourceLatest(t *testing.T) {
	payments := []runtime.Object{referencedNamespace(nsID)}

	t.Run("not created yet, so the spec is empty but refs are resolved", func(t *testing.T) {
		got, err := runLatest(t, newUser(), payments)

		require.NoError(t, err)
		require.Equal(t, tcloudv1alpha1.UserSpec{}, got.Spec)
		require.Equal(t, cloudcommon.ResourceStatus{}, got.Status.Resource)
		require.Equal(t, resolvedRefs(nsID), got.Status.References)
	})

	t.Run("a resource-id annotation alone is treated as never created", func(t *testing.T) {
		cr := newUser()
		cr.Annotations = map[string]string{cloudcommon.AnnotationKeyResourceID: "user-existing"}

		got, err := runLatest(t, cr, payments)

		require.NoError(t, err)
		require.Equal(t, tcloudv1alpha1.UserSpec{}, got.Spec)
		require.Empty(t, got.Status.Resource.ID)
	})

	// A Namespace CR that has no Temporal Cloud ID yet, or isn't Active yet,
	// can't be granted access to, so it isn't listed.
	notReady := []struct {
		name string
		ns   *tcloudv1alpha1.Namespace
	}{
		{"a namespace without an id yet", referencedNamespace("")},
		{"a namespace that is still activating", func() *tcloudv1alpha1.Namespace {
			ns := referencedNamespace(nsID)
			ns.Status.Resource.State = cloudcommon.ResourceStateActivating
			return ns
		}()},
	}
	for _, tc := range notReady {
		t.Run(tc.name+" isn't listed", func(t *testing.T) {
			got, err := runLatest(t, newUser(), []runtime.Object{tc.ns})

			require.NoError(t, err)
			require.Empty(t, got.Status.References.Namespaces)
		})
	}

	t.Run("each resolved namespace is listed once", func(t *testing.T) {
		cr := newUser()
		write := tcloudv1alpha1.NamespaceAccess{Permission: tcloudv1alpha1.NamespacePermissionWrite}
		cr.Spec.Access.NamespaceAccesses = []tcloudv1alpha1.NamespaceScopedAccess{
			{Namespace: ref(nsName), Access: write},
			{Namespace: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Namespace: kubeNamespace, Name: nsName},
			}, Access: write},
			{Namespace: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Namespace: "shared", Name: "billing"},
			}, Access: write},
			{Namespace: id("legacy.acct"), Access: write},
		}

		got, err := runLatest(t, cr, payments)

		require.NoError(t, err)
		require.Equal(t, resolvedRefs(nsID), got.Status.References)
	})

	t.Run("already created, so it's read from temporal cloud by its id", func(t *testing.T) {
		invited := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		expires := invited.Add(30 * 24 * time.Hour)
		var gotReq atomic.Pointer[tcloudservicev1.GetUserRequest]
		getUser := fakeserver.WithGetUser(func(
			_ context.Context, req *tcloudservicev1.GetUserRequest,
		) (*tcloudservicev1.GetUserResponse, error) {
			gotReq.Store(req)
			return &tcloudservicev1.GetUserResponse{User: &tcloudidentityv1.User{
				Id:               userID,
				ResourceVersion:  "rv-2",
				State:            tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
				AsyncOperationId: "op-cloud",
				Spec:             cloudSpec(),
				Invitation: &tcloudidentityv1.Invitation{
					CreatedTime: timestamppb.New(invited),
					ExpiredTime: timestamppb.New(expires),
				},
			}}, nil
		})

		got, err := runLatest(t, linkedUser(cloudcommon.ResourceStateActive, ""), payments, getUser)

		require.NoError(t, err)
		require.NotNil(t, gotReq.Load())
		require.Equal(t, userID, gotReq.Load().GetUserId())
		require.Equal(t, tcloudv1alpha1.UserSpec{}.FromProto(cloudSpec()), got.Spec)
		require.Equal(t, id(nsID), got.Spec.Access.NamespaceAccesses[0].Namespace)
		require.Equal(t, cloudcommon.ResourceStatus{
			ID:               userID,
			Version:          "rv-2",
			State:            cloudcommon.ResourceStateActive,
			AsyncOperationID: "op-cloud",
		}, got.Status.Resource)
		require.Equal(t, &tcloudv1alpha1.UserInvitation{
			CreatedTime: &metav1.Time{Time: invited},
			ExpiredTime: &metav1.Time{Time: expires},
		}, got.Status.Invitation)
		require.Equal(t, resolvedRefs(nsID), got.Status.References)
	})

	t.Run("not found right after the create, so the id and operation are kept", func(t *testing.T) {
		got, err := runLatest(t, linkedUser("", "op-create"), payments,
			fakeserver.OnGetUserReturn(nil, status.Error(codes.NotFound, "user not found")))

		require.NoError(t, err)
		require.Equal(t, tcloudv1alpha1.UserSpec{}, got.Spec)
		require.Equal(t, cloudcommon.ResourceStatus{
			ID: userID, AsyncOperationID: "op-create",
		}, got.Status.Resource)
		require.Equal(t, resolvedRefs(nsID), got.Status.References)
	})

	t.Run("not found after it was seen, so the id is kept but not its last operation", func(t *testing.T) {
		got, err := runLatest(t, linkedUser(cloudcommon.ResourceStateActive, "op-cloud"), payments,
			fakeserver.OnGetUserReturn(nil, status.Error(codes.NotFound, "user not found")))

		require.NoError(t, err)
		require.Equal(t, tcloudv1alpha1.UserSpec{}, got.Spec)
		require.Equal(t, cloudcommon.ResourceStatus{ID: userID}, got.Status.Resource)
	})

	t.Run("temporal cloud can't be read, so an error is returned", func(t *testing.T) {
		_, err := runLatest(t, linkedUser(cloudcommon.ResourceStateActive, ""), payments,
			fakeserver.OnGetUserReturn(nil, status.Error(codes.PermissionDenied, "api key can't read users")))

		require.ErrorContains(t, err, "failed getting tcloud user")
	})
}
