package v1alpha1_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

func id(s string) cloudcommon.ResourceReferenceOrID {
	return cloudcommon.ResourceReferenceOrID{ID: s}
}

func ref(name string) cloudcommon.ResourceReferenceOrID {
	return cloudcommon.ResourceReferenceOrID{Ref: &cloudcommon.ResourceReference{Name: name}}
}

func nsAccess(
	namespace cloudcommon.ResourceReferenceOrID,
	permission tcloudv1alpha1.NamespacePermission,
) tcloudv1alpha1.NamespaceScopedAccess {
	return tcloudv1alpha1.NamespaceScopedAccess{
		Namespace: namespace,
		Access:    tcloudv1alpha1.NamespaceAccess{Permission: permission},
	}
}

func projectAccess(
	project cloudcommon.ResourceReferenceOrID,
	role tcloudv1alpha1.ProjectRole,
) tcloudv1alpha1.ProjectAccessEntry {
	return tcloudv1alpha1.ProjectAccessEntry{
		Project: project,
		Access:  tcloudv1alpha1.ProjectAccess{Role: role},
	}
}

func requireProtoEqual(t *testing.T, exp, got proto.Message) {
	t.Helper()
	require.True(t, proto.Equal(exp, got), "expected:\n%v\ngot:\n%v", exp, got)
}

func TestServiceAccountSpec_ToProto(t *testing.T) {
	cases := []struct {
		name string
		spec tcloudv1alpha1.ServiceAccountSpec
		refs tcloudv1alpha1.ServiceAccountReferences
		exp  *tcloudidentityv1.ServiceAccountSpec
	}{
		{
			name: "account scoped with ids",
			spec: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "ci",
				Description: "deploys",
				AccountScopedAccess: &tcloudv1alpha1.AccountScopedAccess{
					AccountAccess: &tcloudv1alpha1.AccountAccess{
						Role:        tcloudv1alpha1.AccountRoleDeveloper,
						CustomRoles: []cloudcommon.ResourceReferenceOrID{id("cr-2"), id("cr-1")},
					},
					NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
						nsAccess(id("ns1.acct"), tcloudv1alpha1.NamespacePermissionWrite),
					},
					ProjectAccesses: []tcloudv1alpha1.ProjectAccessEntry{
						projectAccess(id("p-1"), tcloudv1alpha1.ProjectRoleAdmin),
					},
				},
			},
			exp: &tcloudidentityv1.ServiceAccountSpec{
				Name:        "ci",
				Description: "deploys",
				Access: &tcloudidentityv1.Access{
					AccountAccess: &tcloudidentityv1.AccountAccess{
						Role: tcloudidentityv1.AccountAccess_ROLE_DEVELOPER,
						// Sorted.
						CustomRoles: []string{"cr-1", "cr-2"},
					},
					NamespaceAccesses: map[string]*tcloudidentityv1.NamespaceAccess{
						"ns1.acct": {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE},
					},
					ProjectAccesses: map[string]*tcloudidentityv1.ProjectAccess{
						"p-1": {Role: tcloudidentityv1.ProjectAccess_PROJECT_ROLE_ADMIN},
					},
				},
			},
		},
		{
			name: "account scoped refs use the id for their namespace/name",
			spec: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "ci",
				AccountScopedAccess: &tcloudv1alpha1.AccountScopedAccess{
					NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
						nsAccess(id("legacy.acct"), tcloudv1alpha1.NamespacePermissionRead),
						nsAccess(ref("payments"), tcloudv1alpha1.NamespacePermissionAdmin),
						nsAccess(
							cloudcommon.ResourceReferenceOrID{
								Ref: &cloudcommon.ResourceReference{Namespace: "shared", Name: "billing"},
							},
							tcloudv1alpha1.NamespacePermissionWrite,
						),
					},
				},
			},
			refs: tcloudv1alpha1.ServiceAccountReferences{
				Namespaces: []tcloudv1alpha1.NamespaceResolvedReference{
					{Namespace: "apps", Name: "payments", ID: "payments.acct"},
					{Namespace: "shared", Name: "billing", ID: "billing.acct"},
				},
			},
			exp: &tcloudidentityv1.ServiceAccountSpec{
				Name: "ci",
				Access: &tcloudidentityv1.Access{
					NamespaceAccesses: map[string]*tcloudidentityv1.NamespaceAccess{
						"legacy.acct":   {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_READ},
						"payments.acct": {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_ADMIN},
						"billing.acct":  {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE},
					},
				},
			},
		},
		{
			name: "empty account access is omitted",
			spec: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "ci",
				AccountScopedAccess: &tcloudv1alpha1.AccountScopedAccess{
					AccountAccess: &tcloudv1alpha1.AccountAccess{},
				},
			},
			exp: &tcloudidentityv1.ServiceAccountSpec{
				Name:   "ci",
				Access: &tcloudidentityv1.Access{},
			},
		},
		{
			name: "namespace scoped with ref",
			spec: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "worker",
				NamespaceScopedAccess: &tcloudv1alpha1.NamespaceScopedAccess{
					Namespace: ref("payments"),
					Access:    tcloudv1alpha1.NamespaceAccess{Permission: tcloudv1alpha1.NamespacePermissionWrite},
				},
			},
			refs: tcloudv1alpha1.ServiceAccountReferences{
				Namespaces: []tcloudv1alpha1.NamespaceResolvedReference{
					{Namespace: "apps", Name: "payments", ID: "payments.acct"},
				},
			},
			exp: &tcloudidentityv1.ServiceAccountSpec{
				Name: "worker",
				NamespaceScopedAccess: &tcloudidentityv1.NamespaceScopedAccess{
					Namespace: "payments.acct",
					Access:    &tcloudidentityv1.NamespaceAccess{Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE},
				},
			},
		},
		{
			name: "project scoped",
			spec: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "deployer",
				ProjectScopedAccess: &tcloudv1alpha1.ProjectScopedAccess{
					Project: id("p-1"),
					Access:  tcloudv1alpha1.ProjectAccess{Role: tcloudv1alpha1.ProjectRoleWrite},
					NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
						nsAccess(id("ns1.acct"), tcloudv1alpha1.NamespacePermissionAdmin),
					},
				},
			},
			exp: &tcloudidentityv1.ServiceAccountSpec{
				Name: "deployer",
				ProjectScopedAccess: &tcloudidentityv1.ProjectScopedAccess{
					ProjectId: "p-1",
					Access:    &tcloudidentityv1.ProjectAccess{Role: tcloudidentityv1.ProjectAccess_PROJECT_ROLE_WRITE},
					NamespaceAccesses: map[string]*tcloudidentityv1.NamespaceAccess{
						"ns1.acct": {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_ADMIN},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireProtoEqual(t, tc.exp, tc.spec.ToProto("apps", tc.refs))
		})
	}
}

func TestServiceAccountSpec_FromProto(t *testing.T) {
	cases := []struct {
		name string
		in   *tcloudidentityv1.ServiceAccountSpec
		exp  tcloudv1alpha1.ServiceAccountSpec
	}{
		{
			name: "account scoped lists are sorted by id",
			in: &tcloudidentityv1.ServiceAccountSpec{
				Name: "ci",
				Access: &tcloudidentityv1.Access{
					AccountAccess: &tcloudidentityv1.AccountAccess{
						Role:        tcloudidentityv1.AccountAccess_ROLE_READ,
						CustomRoles: []string{"cr-2", "cr-1"},
					},
					NamespaceAccesses: map[string]*tcloudidentityv1.NamespaceAccess{
						"b.acct": {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_READ},
						"a.acct": {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE},
					},
					ProjectAccesses: map[string]*tcloudidentityv1.ProjectAccess{
						"p-2": {Role: tcloudidentityv1.ProjectAccess_PROJECT_ROLE_READ},
						"p-1": {Role: tcloudidentityv1.ProjectAccess_PROJECT_ROLE_WRITE},
					},
				},
			},
			exp: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "ci",
				AccountScopedAccess: &tcloudv1alpha1.AccountScopedAccess{
					AccountAccess: &tcloudv1alpha1.AccountAccess{
						Role:        tcloudv1alpha1.AccountRoleRead,
						CustomRoles: []cloudcommon.ResourceReferenceOrID{id("cr-1"), id("cr-2")},
					},
					NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
						nsAccess(id("a.acct"), tcloudv1alpha1.NamespacePermissionWrite),
						nsAccess(id("b.acct"), tcloudv1alpha1.NamespacePermissionRead),
					},
					ProjectAccesses: []tcloudv1alpha1.ProjectAccessEntry{
						projectAccess(id("p-1"), tcloudv1alpha1.ProjectRoleWrite),
						projectAccess(id("p-2"), tcloudv1alpha1.ProjectRoleRead),
					},
				},
			},
		},
		{
			name: "inherited developer project role and unspecified permission are skipped",
			in: &tcloudidentityv1.ServiceAccountSpec{
				Name: "ci",
				Access: &tcloudidentityv1.Access{
					NamespaceAccesses: map[string]*tcloudidentityv1.NamespaceAccess{
						"a.acct": {},
					},
					ProjectAccesses: map[string]*tcloudidentityv1.ProjectAccess{
						"p-1": {Role: tcloudidentityv1.ProjectAccess_PROJECT_ROLE_DEVELOPER},
						"p-2": {Role: tcloudidentityv1.ProjectAccess_PROJECT_ROLE_LIST},
					},
				},
			},
			exp: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "ci",
				AccountScopedAccess: &tcloudv1alpha1.AccountScopedAccess{
					ProjectAccesses: []tcloudv1alpha1.ProjectAccessEntry{
						projectAccess(id("p-2"), tcloudv1alpha1.ProjectRoleList),
					},
				},
			},
		},
		{
			name: "no access at all is account scoped",
			in:   &tcloudidentityv1.ServiceAccountSpec{Name: "ci"},
			exp: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName:         "ci",
				AccountScopedAccess: &tcloudv1alpha1.AccountScopedAccess{},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var spec tcloudv1alpha1.ServiceAccountSpec
			require.Equal(t, tc.exp, spec.FromProto(tc.in))
		})
	}
}

// TestServiceAccountSpec_RoundTrip checks that what Temporal Cloud returns
// for a spec compares equal to the spec, whatever order the CR lists are in.
func TestServiceAccountSpec_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		spec tcloudv1alpha1.ServiceAccountSpec
	}{
		{
			name: "account scoped",
			spec: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "ci",
				AccountScopedAccess: &tcloudv1alpha1.AccountScopedAccess{
					AccountAccess: &tcloudv1alpha1.AccountAccess{
						Role:        tcloudv1alpha1.AccountRoleAdmin,
						CustomRoles: []cloudcommon.ResourceReferenceOrID{id("cr-9"), id("cr-1")},
					},
					NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
						nsAccess(id("z.acct"), tcloudv1alpha1.NamespacePermissionRead),
						nsAccess(id("a.acct"), tcloudv1alpha1.NamespacePermissionAdmin),
					},
					ProjectAccesses: []tcloudv1alpha1.ProjectAccessEntry{
						projectAccess(id("p-2"), tcloudv1alpha1.ProjectRoleMember),
						projectAccess(id("p-1"), tcloudv1alpha1.ProjectRoleContribute),
					},
				},
			},
		},
		{
			name: "project scoped",
			spec: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "deployer",
				ProjectScopedAccess: &tcloudv1alpha1.ProjectScopedAccess{
					Project: id("p-1"),
					Access:  tcloudv1alpha1.ProjectAccess{Role: tcloudv1alpha1.ProjectRoleRead},
					NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
						nsAccess(id("b.acct"), tcloudv1alpha1.NamespacePermissionRead),
						nsAccess(id("a.acct"), tcloudv1alpha1.NamespacePermissionWrite),
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var none tcloudv1alpha1.ServiceAccountReferences
			sent := tc.spec.ToProto("apps", none)
			var fromCloud tcloudv1alpha1.ServiceAccountSpec
			fromCloud = fromCloud.FromProto(sent)
			requireProtoEqual(t, sent, fromCloud.ToProto("apps", none))
		})
	}
}

func TestServiceAccount_FromProto(t *testing.T) {
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	sa := tcloudv1alpha1.ServiceAccount{}
	sa.Name = "ci"
	sa.Namespace = "apps"
	sa.Status.References.Namespaces = []tcloudv1alpha1.NamespaceResolvedReference{
		{Namespace: "apps", Name: "payments", ID: "payments.acct"},
	}

	cases := []struct {
		name     string
		in       *tcloudidentityv1.ServiceAccount
		expSpec  tcloudv1alpha1.ServiceAccountSpec
		expState cloudcommon.ResourceStatus
	}{
		{
			name: "spec and status come from the proto",
			in: &tcloudidentityv1.ServiceAccount{
				Id:              "sa-1",
				ResourceVersion: "rv-1",
				State:           tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
				CreatedTime:     timestamppb.New(created),
				Spec: &tcloudidentityv1.ServiceAccountSpec{
					Name: "ci",
					NamespaceScopedAccess: &tcloudidentityv1.NamespaceScopedAccess{
						Namespace: "payments.acct",
						Access:    &tcloudidentityv1.NamespaceAccess{Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_READ},
					},
				},
			},
			expSpec: tcloudv1alpha1.ServiceAccountSpec{
				DisplayName: "ci",
				NamespaceScopedAccess: &tcloudv1alpha1.NamespaceScopedAccess{
					Namespace: id("payments.acct"),
					Access:    tcloudv1alpha1.NamespaceAccess{Permission: tcloudv1alpha1.NamespacePermissionRead},
				},
			},
			expState: cloudcommon.ResourceStatus{
				ID:          "sa-1",
				Version:     "rv-1",
				State:       cloudcommon.ResourceStateActive,
				CreatedTime: &metav1.Time{Time: created},
			},
		},
		{
			name: "nil proto keeps only metadata and references",
			in:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sa.FromProto(tc.in)
			require.Equal(t, sa.ObjectMeta, got.ObjectMeta)
			require.Equal(t, sa.Status.References, got.Status.References, "references are kept")
			require.Equal(t, tc.expSpec, got.Spec)
			require.Equal(t, tc.expState.ID, got.Status.Resource.ID)
			require.Equal(t, tc.expState.Version, got.Status.Resource.Version)
			require.Equal(t, tc.expState.State, got.Status.Resource.State)
			if tc.expState.CreatedTime != nil {
				require.True(t, got.Status.Resource.CreatedTime.Time.Equal(created))
			}
		})
	}
}

func TestAccountRoleConversion(t *testing.T) {
	cases := []struct {
		proto tcloudidentityv1.AccountAccess_Role
		cr    tcloudv1alpha1.AccountRole
	}{
		{tcloudidentityv1.AccountAccess_ROLE_OWNER, tcloudv1alpha1.AccountRoleOwner},
		{tcloudidentityv1.AccountAccess_ROLE_ADMIN, tcloudv1alpha1.AccountRoleAdmin},
		{tcloudidentityv1.AccountAccess_ROLE_DEVELOPER, tcloudv1alpha1.AccountRoleDeveloper},
		{tcloudidentityv1.AccountAccess_ROLE_FINANCE_ADMIN, tcloudv1alpha1.AccountRoleFinanceAdmin},
		{tcloudidentityv1.AccountAccess_ROLE_READ, tcloudv1alpha1.AccountRoleRead},
		{tcloudidentityv1.AccountAccess_ROLE_METRICS_READ, tcloudv1alpha1.AccountRoleMetricsRead},
	}
	for _, tc := range cases {
		t.Run(string(tc.cr), func(t *testing.T) {
			var got tcloudv1alpha1.AccountRole
			got.FromProto(tc.proto)
			require.Equal(t, tc.cr, got)
			require.Equal(t, tc.proto, tc.cr.ToProto())
		})
	}

	t.Run("unspecified and unknown from proto", func(t *testing.T) {
		for _, in := range []tcloudidentityv1.AccountAccess_Role{
			tcloudidentityv1.AccountAccess_ROLE_UNSPECIFIED,
			tcloudidentityv1.AccountAccess_Role(99),
		} {
			var got tcloudv1alpha1.AccountRole
			got.FromProto(in)
			require.Empty(t, got)
		}
	})
}

func TestNamespacePermissionConversion(t *testing.T) {
	cases := []struct {
		proto tcloudidentityv1.NamespaceAccess_Permission
		cr    tcloudv1alpha1.NamespacePermission
	}{
		{tcloudidentityv1.NamespaceAccess_PERMISSION_ADMIN, tcloudv1alpha1.NamespacePermissionAdmin},
		{tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE, tcloudv1alpha1.NamespacePermissionWrite},
		{tcloudidentityv1.NamespaceAccess_PERMISSION_READ, tcloudv1alpha1.NamespacePermissionRead},
	}
	for _, tc := range cases {
		t.Run(string(tc.cr), func(t *testing.T) {
			var got tcloudv1alpha1.NamespacePermission
			got.FromProto(tc.proto)
			require.Equal(t, tc.cr, got)
			require.Equal(t, tc.proto, tc.cr.ToProto())
		})
	}

	t.Run("unspecified and unknown from proto", func(t *testing.T) {
		for _, in := range []tcloudidentityv1.NamespaceAccess_Permission{
			tcloudidentityv1.NamespaceAccess_PERMISSION_UNSPECIFIED,
			tcloudidentityv1.NamespaceAccess_Permission(99),
		} {
			var got tcloudv1alpha1.NamespacePermission
			got.FromProto(in)
			require.Empty(t, got)
		}
	})
}

func TestProjectRoleConversion(t *testing.T) {
	cases := []struct {
		proto tcloudidentityv1.ProjectAccess_ProjectRole
		cr    tcloudv1alpha1.ProjectRole
	}{
		{tcloudidentityv1.ProjectAccess_PROJECT_ROLE_ADMIN, tcloudv1alpha1.ProjectRoleAdmin},
		{tcloudidentityv1.ProjectAccess_PROJECT_ROLE_WRITE, tcloudv1alpha1.ProjectRoleWrite},
		{tcloudidentityv1.ProjectAccess_PROJECT_ROLE_READ, tcloudv1alpha1.ProjectRoleRead},
		{tcloudidentityv1.ProjectAccess_PROJECT_ROLE_LIST, tcloudv1alpha1.ProjectRoleList},
		{tcloudidentityv1.ProjectAccess_PROJECT_ROLE_CONTRIBUTE, tcloudv1alpha1.ProjectRoleContribute},
		{tcloudidentityv1.ProjectAccess_PROJECT_ROLE_MEMBER, tcloudv1alpha1.ProjectRoleMember},
	}
	for _, tc := range cases {
		t.Run(string(tc.cr), func(t *testing.T) {
			var got tcloudv1alpha1.ProjectRole
			got.FromProto(tc.proto)
			require.Equal(t, tc.cr, got)
			require.Equal(t, tc.proto, tc.cr.ToProto())
		})
	}

	t.Run("unspecified, inherited developer and unknown from proto", func(t *testing.T) {
		for _, in := range []tcloudidentityv1.ProjectAccess_ProjectRole{
			tcloudidentityv1.ProjectAccess_PROJECT_ROLE_UNSPECIFIED,
			tcloudidentityv1.ProjectAccess_PROJECT_ROLE_DEVELOPER,
			tcloudidentityv1.ProjectAccess_ProjectRole(99),
		} {
			var got tcloudv1alpha1.ProjectRole
			got.FromProto(in)
			require.Empty(t, got)
		}
	})
}
