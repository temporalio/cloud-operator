package usergroup_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/usergroup"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

func TestReconciler_ResourceLatest(t *testing.T) {
	payments := []runtime.Object{referencedNamespace(nsID)}

	// withOrders refers to the Project orders as well as the Namespace payments.
	withOrders := newUserGroup()
	withOrders.Spec.Access.ProjectAccesses = []tcloudv1alpha1.ProjectAccessEntry{{
		Project: ref(projectName),
		Access:  tcloudv1alpha1.ProjectAccess{Role: tcloudv1alpha1.ProjectRoleRead},
	}}

	fromCloud := &tcloudidentityv1.UserGroup{
		Id:               groupID,
		ResourceVersion:  "rv-2",
		State:            tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		AsyncOperationId: "op-cloud",
		Spec:             cloudSpec(),
	}
	notFound := status.Error(codes.NotFound, "user group not found")

	cases := []struct {
		name        string
		cr          *tcloudv1alpha1.UserGroup
		kubeObjs    []runtime.Object
		getGroup    *tcloudidentityv1.UserGroup
		getErr      error
		expGetIDs   []string
		expSpec     tcloudv1alpha1.UserGroupSpec
		expResource cloudcommon.ResourceStatus
		expRefs     tcloudv1alpha1.UserGroupReferences
	}{
		{
			name:     "never created group yields an empty spec and resolved refs",
			cr:       withOrders,
			kubeObjs: []runtime.Object{referencedNamespace(nsID), referencedProject(cloudcommon.ResourceStateActive)},
			expRefs: tcloudv1alpha1.UserGroupReferences{
				Namespaces: resolvedRefs(nsID).Namespaces,
				Projects: []cloudcommon.ResolvedReference{
					{Namespace: kubeNamespace, Name: projectName, ID: projectID},
				},
			},
		},
		{
			name:      "created group yields its spec and status from temporal cloud",
			cr:        linkedUserGroup(cloudcommon.ResourceStateActive, ""),
			kubeObjs:  payments,
			getGroup:  fromCloud,
			expGetIDs: []string{groupID},
			expSpec:   tcloudv1alpha1.UserGroupSpec{}.FromProto(cloudSpec()),
			expResource: cloudcommon.ResourceStatus{
				ID:               groupID,
				Version:          "rv-2",
				State:            cloudcommon.ResourceStateActive,
				AsyncOperationID: "op-cloud",
			},
			expRefs: resolvedRefs(nsID),
		},
		{
			// Sync needs the operation ID to wait for the pending create.
			name:        "not found right after the create yields the id and operation",
			cr:          linkedUserGroup("", "op-create"),
			kubeObjs:    payments,
			getErr:      notFound,
			expGetIDs:   []string{groupID},
			expResource: cloudcommon.ResourceStatus{ID: groupID, AsyncOperationID: "op-create"},
			expRefs:     resolvedRefs(nsID),
		},
		{
			name:        "not found after it was seen yields the id without its last operation",
			cr:          linkedUserGroup(cloudcommon.ResourceStateActive, "op-cloud"),
			kubeObjs:    payments,
			getErr:      notFound,
			expGetIDs:   []string{groupID},
			expResource: cloudcommon.ResourceStatus{ID: groupID},
			expRefs:     resolvedRefs(nsID),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			var mu sync.Mutex
			var gotGetIDs []string
			getUserGroup := fakeserver.WithGetUserGroup(func(
				_ context.Context, req *tcloudservicev1.GetUserGroupRequest,
			) (*tcloudservicev1.GetUserGroupResponse, error) {
				mu.Lock()
				defer mu.Unlock()
				gotGetIDs = append(gotGetIDs, req.GetGroupId())
				return &tcloudservicev1.GetUserGroupResponse{Group: tc.getGroup}, tc.getErr
			})
			r := newReconciler(tt, tc.kubeObjs, getUserGroup)
			current := usergroup.NewResource(tc.cr.DeepCopy())
			before := current.DeepCopy()

			got, err := r.ResourceLatest(context.TODO(), current)

			// The Controller compares current with the result to decide
			// whether to sync.
			require.Equal(before.CR(), current.CR(), "current must not be mutated")
			require.NoError(err)
			mu.Lock()
			require.Equal(tc.expGetIDs, gotGetIDs)
			mu.Unlock()
			require.Equal(tc.expSpec, userGroupOf(got).Spec)
			require.Equal(tc.expResource, statusOf(got))
			require.Equal(tc.expRefs, userGroupOf(got).Status.References)
		})
	}
}
