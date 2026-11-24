package v1alpha1_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// userGroupCR returns a UserGroup CR whose status already references a
// Namespace CR, so tests can check that FromProto keeps it.
func userGroupCR() tcloudv1alpha1.UserGroup {
	g := tcloudv1alpha1.UserGroup{}
	g.Name = "oncall"
	g.Namespace = "apps"
	g.Status.References = tcloudv1alpha1.UserGroupReferences{
		Namespaces: []cloudcommon.ResolvedReference{
			{Namespace: "apps", Name: "payments", ID: "payments.acct"},
		},
	}
	return g
}

func TestUserGroup_FromProto(t *testing.T) {
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	modified := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	cloudGroup := &tcloudidentityv1.UserGroupSpec_CloudGroup{CloudGroup: &tcloudidentityv1.CloudGroupSpec{}}

	cases := []struct {
		name      string
		in        *tcloudidentityv1.UserGroup
		expSpec   tcloudv1alpha1.UserGroupSpec
		expStatus cloudcommon.ResourceStatus
	}{
		{
			name: "cloud group with access yields its spec and status",
			in: &tcloudidentityv1.UserGroup{
				Id:               "group-1",
				ResourceVersion:  "rv-1",
				State:            tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
				AsyncOperationId: "op-1",
				CreatedTime:      timestamppb.New(created),
				LastModifiedTime: timestamppb.New(modified),
				Spec: &tcloudidentityv1.UserGroupSpec{
					DisplayName: "oncall",
					GroupType:   cloudGroup,
					Access: &tcloudidentityv1.Access{
						AccountAccess: &tcloudidentityv1.AccountAccess{
							Role: tcloudidentityv1.AccountAccess_ROLE_DEVELOPER,
						},
						NamespaceAccesses: map[string]*tcloudidentityv1.NamespaceAccess{
							"payments.acct": {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE},
						},
					},
				},
			},
			expSpec: tcloudv1alpha1.UserGroupSpec{
				DisplayName: "oncall",
				CloudGroup:  &tcloudv1alpha1.CloudGroupSpec{},
				Access: tcloudv1alpha1.AccountScopedAccess{
					AccountAccess: &tcloudv1alpha1.AccountAccess{Role: tcloudv1alpha1.AccountRoleDeveloper},
					NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
						nsAccess(id("payments.acct"), tcloudv1alpha1.NamespacePermissionWrite),
					},
				},
			},
			expStatus: cloudcommon.ResourceStatus{
				ID:               "group-1",
				Version:          "rv-1",
				State:            cloudcommon.ResourceStateActive,
				AsyncOperationID: "op-1",
				CreatedTime:      &metav1.Time{Time: created},
				LastModifiedTime: &metav1.Time{Time: modified},
			},
		},
		{
			name: "google group yields its email address",
			in: &tcloudidentityv1.UserGroup{
				Id: "group-1",
				Spec: &tcloudidentityv1.UserGroupSpec{
					DisplayName: "Payments On Call",
					GroupType: &tcloudidentityv1.UserGroupSpec_GoogleGroup{
						GoogleGroup: &tcloudidentityv1.GoogleGroupSpec{EmailAddress: "oncall@example.com"},
					},
				},
			},
			expSpec: tcloudv1alpha1.UserGroupSpec{
				DisplayName: "Payments On Call",
				GoogleGroup: &tcloudv1alpha1.GoogleGroupSpec{EmailAddress: "oncall@example.com"},
			},
			expStatus: cloudcommon.ResourceStatus{ID: "group-1"},
		},
		{
			name: "scim group yields no group type",
			in: &tcloudidentityv1.UserGroup{
				Id: "group-1",
				Spec: &tcloudidentityv1.UserGroupSpec{
					DisplayName: "engineering",
					GroupType: &tcloudidentityv1.UserGroupSpec_ScimGroup{
						ScimGroup: &tcloudidentityv1.SCIMGroupSpec{IdpId: "idp-1"},
					},
				},
			},
			expSpec:   tcloudv1alpha1.UserGroupSpec{DisplayName: "engineering"},
			expStatus: cloudcommon.ResourceStatus{ID: "group-1"},
		},
		{
			name: "group without access yields an empty access",
			in: &tcloudidentityv1.UserGroup{
				Id: "group-1",
				Spec: &tcloudidentityv1.UserGroupSpec{
					DisplayName: "oncall",
					GroupType:   cloudGroup,
				},
			},
			expSpec: tcloudv1alpha1.UserGroupSpec{
				DisplayName: "oncall",
				CloudGroup:  &tcloudv1alpha1.CloudGroupSpec{},
			},
			expStatus: cloudcommon.ResourceStatus{ID: "group-1"},
		},
		{
			name: "nil proto yields only metadata and references",
			in:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			g := userGroupCR()

			got := g.FromProto(tc.in)

			require.Equal(tc.expSpec, got.Spec)
			require.Equal(tc.expStatus, got.Status.Resource)
			require.Equal(g.ObjectMeta, got.ObjectMeta)
			require.Equal(g.Status.References, got.Status.References, "references are kept")
		})
	}
}
