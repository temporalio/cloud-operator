package usergroup_test

import (
	"context"
	"fmt"
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
	// fromCloudSpec is fromCloud's spec with the members GetUserGroupMembers
	// returns over two pages.
	fromCloudSpec := tcloudv1alpha1.UserGroupSpec{}.FromProto(cloudSpec())
	fromCloudSpec.CloudGroup.Members = []cloudcommon.ResourceReferenceOrID{id("user-1"), id("user-2")}
	notFound := status.Error(codes.NotFound, "user group not found")

	cases := []struct {
		name     string
		cr       *tcloudv1alpha1.UserGroup
		kubeObjs []runtime.Object
		getGroup *tcloudidentityv1.UserGroup
		getErr   error
		// memberPages are the user IDs on each page GetUserGroupMembers returns.
		memberPages [][]string
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
			name:        "created group yields its spec and status from temporal cloud",
			cr:          linkedUserGroup(cloudcommon.ResourceStateActive, ""),
			kubeObjs:    payments,
			getGroup:    fromCloud,
			memberPages: [][]string{{"user-1"}, {"user-2"}},
			expGetIDs:   []string{groupID},
			expSpec:     fromCloudSpec,
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
			getMembers := fakeserver.WithGetUserGroupMembers(func(
				_ context.Context, req *tcloudservicev1.GetUserGroupMembersRequest,
			) (*tcloudservicev1.GetUserGroupMembersResponse, error) {
				// Temporal Cloud rejects a page size over 500.
				if req.GetPageSize() > 500 {
					return nil, status.Error(codes.InvalidArgument, "page size is too large")
				}
				return memberPage(tc.memberPages, req.GetPageToken()), nil
			})
			r := newReconciler(tt, tc.kubeObjs, getUserGroup, getMembers)
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

// memberPage returns the page of pages that pageToken asks for. The token of
// page i is "page-i", and the first page has no token.
func memberPage(pages [][]string, pageToken string) *tcloudservicev1.GetUserGroupMembersResponse {
	resp := &tcloudservicev1.GetUserGroupMembersResponse{}
	for i, page := range pages {
		token := ""
		if i > 0 {
			token = fmt.Sprintf("page-%d", i)
		}
		if token != pageToken {
			continue
		}
		for _, userID := range page {
			resp.Members = append(resp.Members, &tcloudidentityv1.UserGroupMember{
				MemberId: &tcloudidentityv1.UserGroupMemberId{
					MemberType: &tcloudidentityv1.UserGroupMemberId_UserId{UserId: userID},
				},
			})
		}
		if i+1 < len(pages) {
			resp.NextPageToken = fmt.Sprintf("page-%d", i+1)
		}
	}
	return resp
}
