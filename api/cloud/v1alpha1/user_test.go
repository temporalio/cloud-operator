package v1alpha1_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

// userCR returns a User CR named alice whose status already references a
// Namespace CR, so tests can check that FromProto keeps both.
func userCR() tcloudv1alpha1.User {
	u := tcloudv1alpha1.User{}
	u.Name = "alice"
	u.Namespace = "apps"
	u.Status.References.Namespaces = []tcloudv1alpha1.NamespaceResolvedReference{
		{Namespace: "apps", Name: "payments", ID: "payments.acct"},
	}
	return u
}

func TestUser_FromProto(t *testing.T) {
	t.Run("spec and status come from the proto", func(tt *testing.T) {
		require := require.New(tt)
		created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		modified := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
		invited := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
		expires := time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC)
		u := userCR()

		got := u.FromProto(&tcloudidentityv1.User{
			Id:               "user-1",
			ResourceVersion:  "rv-1",
			State:            tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
			AsyncOperationId: "op-1",
			CreatedTime:      timestamppb.New(created),
			LastModifiedTime: timestamppb.New(modified),
			Invitation: &tcloudidentityv1.Invitation{
				CreatedTime: timestamppb.New(invited),
				ExpiredTime: timestamppb.New(expires),
			},
			Spec: &tcloudidentityv1.UserSpec{
				Email: "alice@example.com",
				Access: &tcloudidentityv1.Access{
					AccountAccess: &tcloudidentityv1.AccountAccess{
						Role:        tcloudidentityv1.AccountAccess_ROLE_DEVELOPER,
						CustomRoles: []string{"role-b", "role-a"},
					},
					NamespaceAccesses: map[string]*tcloudidentityv1.NamespaceAccess{
						"payments.acct": {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE},
					},
					ProjectAccesses: map[string]*tcloudidentityv1.ProjectAccess{
						"project-1": {Role: tcloudidentityv1.ProjectAccess_PROJECT_ROLE_READ},
					},
				},
			},
		})

		require.Equal(tcloudv1alpha1.UserSpec{
			Email: "alice@example.com",
			Access: tcloudv1alpha1.AccountScopedAccess{
				AccountAccess: &tcloudv1alpha1.AccountAccess{
					Role:        tcloudv1alpha1.AccountRoleDeveloper,
					CustomRoles: []cloudcommon.ResourceReferenceOrID{id("role-a"), id("role-b")},
				},
				NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
					nsAccess(id("payments.acct"), tcloudv1alpha1.NamespacePermissionWrite),
				},
				ProjectAccesses: []tcloudv1alpha1.ProjectAccessEntry{
					projectAccess(id("project-1"), tcloudv1alpha1.ProjectRoleRead),
				},
			},
		}, got.Spec)
		require.Equal(cloudcommon.ResourceStatus{
			ID:               "user-1",
			Version:          "rv-1",
			State:            cloudcommon.ResourceStateActive,
			AsyncOperationID: "op-1",
			CreatedTime:      &metav1.Time{Time: created},
			LastModifiedTime: &metav1.Time{Time: modified},
		}, got.Status.Resource)
		require.Equal(&tcloudv1alpha1.UserInvitation{
			CreatedTime: &metav1.Time{Time: invited},
			ExpiredTime: &metav1.Time{Time: expires},
		}, got.Status.Invitation)
		require.Equal(u.ObjectMeta, got.ObjectMeta)
		require.Equal(u.Status.References, got.Status.References, "references are kept")
	})

	t.Run("no invitation once it is accepted", func(tt *testing.T) {
		u := userCR()

		got := u.FromProto(&tcloudidentityv1.User{Id: "user-1"})

		require.Nil(tt, got.Status.Invitation)
	})

	t.Run("a user without access has an empty access", func(tt *testing.T) {
		u := userCR()

		// Users created by SAML just-in-time provisioning have no access.
		got := u.FromProto(&tcloudidentityv1.User{
			Id:   "user-1",
			Spec: &tcloudidentityv1.UserSpec{Email: "alice@example.com"},
		})

		require.Equal(tt, tcloudv1alpha1.AccountScopedAccess{}, got.Spec.Access)
	})

	t.Run("nil proto keeps only metadata and references", func(tt *testing.T) {
		require := require.New(tt)
		u := userCR()

		got := u.FromProto(nil)

		require.Equal(u.ObjectMeta, got.ObjectMeta)
		require.Equal(u.Status.References, got.Status.References)
		require.Equal(tcloudv1alpha1.UserSpec{}, got.Spec)
		require.Equal(cloudcommon.ResourceStatus{}, got.Status.Resource)
		require.Nil(got.Status.Invitation)
	})
}
