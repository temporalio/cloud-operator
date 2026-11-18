package serviceaccount_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/serviceaccount"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

func TestReconciler_ResourceLatest(t *testing.T) {
	synced := newServiceAccount(ref(nsName))
	synced.Status.Resource.ID = saID

	// projectScoped refers to the Project orders and the Namespace payments.
	projectScoped := newServiceAccount(id(nsID))
	projectScoped.Spec.NamespaceScopedAccess = nil
	projectScoped.Spec.ProjectScopedAccess = &tcloudv1alpha1.ProjectScopedAccess{
		Project: ref("orders"),
		Access:  tcloudv1alpha1.ProjectAccess{Role: tcloudv1alpha1.ProjectRoleRead},
		NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
			{Namespace: ref(nsName)},
		},
	}
	referencedProject := func(state cloudcommon.ResourceState) *tcloudv1alpha1.Project {
		p := &tcloudv1alpha1.Project{}
		p.Namespace = kubeNamespace
		p.Name = "orders"
		p.Status.Resource.ID = "p-orders"
		p.Status.Resource.State = state
		return p
	}

	fromCloud := &tcloudidentityv1.ServiceAccount{
		Id:              saID,
		ResourceVersion: "rv-1",
		State:           tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		Spec:            cloudSpec(),
	}

	cases := []struct {
		name       string
		cr         *tcloudv1alpha1.ServiceAccount
		kubeObjs   []runtime.Object
		serverOpts []fakeserver.NewOption
		expErr     string
		assert     func(t *testing.T, got *tcloudv1alpha1.ServiceAccount)
	}{
		{
			name:     "not created yet, so the spec is empty but refs are resolved",
			cr:       newServiceAccount(ref(nsName)),
			kubeObjs: []runtime.Object{referencedNamespace(nsID)},
			assert: func(t *testing.T, got *tcloudv1alpha1.ServiceAccount) {
				require.Equal(t, tcloudv1alpha1.ServiceAccountSpec{}, got.Spec)
				require.Empty(t, got.Status.Resource.ID)
				require.Equal(t, tcloudv1alpha1.ServiceAccountReferences{
					Namespaces: []cloudcommon.ResolvedReference{
						{Namespace: "apps", Name: "payments", ID: nsID},
					},
				}, got.Status.References)
			},
		},
		{
			name: "a resource-id annotation alone is treated as never created",
			cr: func() *tcloudv1alpha1.ServiceAccount {
				sa := newServiceAccount(ref(nsName))
				sa.Annotations = map[string]string{cloudcommon.AnnotationKeyResourceID: "sa-existing"}
				return sa
			}(),
			kubeObjs: []runtime.Object{referencedNamespace(nsID)},
			assert: func(t *testing.T, got *tcloudv1alpha1.ServiceAccount) {
				require.Equal(t, tcloudv1alpha1.ServiceAccountSpec{}, got.Spec)
				require.Empty(t, got.Status.Resource.ID)
			},
		},
		{
			name:     "a namespace without an id yet isn't listed",
			cr:       newServiceAccount(ref(nsName)),
			kubeObjs: []runtime.Object{referencedNamespace("")},
			assert: func(t *testing.T, got *tcloudv1alpha1.ServiceAccount) {
				require.Empty(t, got.Status.References.Namespaces)
			},
		},
		{
			name: "a namespace that is still activating isn't listed",
			cr:   newServiceAccount(ref(nsName)),
			kubeObjs: func() []runtime.Object {
				ns := referencedNamespace(nsID)
				ns.Status.Resource.State = cloudcommon.ResourceStateActivating
				return []runtime.Object{ns}
			}(),
			assert: func(t *testing.T, got *tcloudv1alpha1.ServiceAccount) {
				require.Empty(t, got.Status.References.Namespaces)
			},
		},
		{
			name: "each resolved namespace is listed once",
			cr: func() *tcloudv1alpha1.ServiceAccount {
				sa := newServiceAccount(id(nsID))
				sa.Spec.NamespaceScopedAccess = nil
				sa.Spec.AccountScopedAccess = &tcloudv1alpha1.AccountScopedAccess{
					NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{
						{Namespace: ref(nsName)},
						{Namespace: cloudcommon.ResourceReferenceOrID{
							Ref: &cloudcommon.ResourceReference{Namespace: kubeNamespace, Name: nsName},
						}},
						{Namespace: cloudcommon.ResourceReferenceOrID{
							Ref: &cloudcommon.ResourceReference{Namespace: "shared", Name: "billing"},
						}},
						{Namespace: id("legacy.acct")},
					},
				}
				return sa
			}(),
			kubeObjs: []runtime.Object{referencedNamespace(nsID)},
			assert: func(t *testing.T, got *tcloudv1alpha1.ServiceAccount) {
				require.Equal(t, []cloudcommon.ResolvedReference{
					{Namespace: "apps", Name: "payments", ID: nsID},
				}, got.Status.References.Namespaces)
			},
		},
		{
			name:     "already created, so it's read from temporal cloud by its id",
			cr:       synced,
			kubeObjs: []runtime.Object{referencedNamespace(nsID)},
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetServiceAccount(
					func(req *tcloudservicev1.GetServiceAccountRequest) {
						assert.Equal(t, saID, req.GetServiceAccountId())
					},
					func() (*tcloudservicev1.GetServiceAccountResponse, error) {
						return &tcloudservicev1.GetServiceAccountResponse{ServiceAccount: fromCloud}, nil
					},
				),
			},
			assert: func(t *testing.T, got *tcloudv1alpha1.ServiceAccount) {
				require.Equal(t, saID, got.Status.Resource.ID)
				require.Equal(t, "rv-1", got.Status.Resource.Version)
				require.Equal(t, cloudcommon.ResourceStateActive, got.Status.Resource.State)
				require.Equal(t, id(nsID), got.Spec.NamespaceScopedAccess.Namespace)
				require.Equal(t, tcloudv1alpha1.ServiceAccountReferences{
					Namespaces: []cloudcommon.ResolvedReference{
						{Namespace: "apps", Name: "payments", ID: nsID},
					},
				}, got.Status.References)
			},
		},
		{
			name:     "deleted outside the operator, so it's treated as not created",
			cr:       synced,
			kubeObjs: []runtime.Object{referencedNamespace(nsID)},
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetServiceAccountReturn(nil, status.Error(codes.NotFound, "gone")),
			},
			assert: func(t *testing.T, got *tcloudv1alpha1.ServiceAccount) {
				require.Equal(t, tcloudv1alpha1.ServiceAccountSpec{}, got.Spec)
				require.Empty(t, got.Status.Resource.ID)
			},
		},
		{
			name:     "project and namespace refs are both resolved",
			cr:       projectScoped,
			kubeObjs: []runtime.Object{referencedNamespace(nsID), referencedProject(cloudcommon.ResourceStateActive)},
			assert: func(t *testing.T, got *tcloudv1alpha1.ServiceAccount) {
				require.Equal(t, tcloudv1alpha1.ServiceAccountReferences{
					Namespaces: []cloudcommon.ResolvedReference{
						{Namespace: "apps", Name: "payments", ID: nsID},
					},
					Projects: []cloudcommon.ResolvedReference{
						{Namespace: "apps", Name: "orders", ID: "p-orders"},
					},
				}, got.Status.References)
			},
		},
		{
			name:     "a project that is still activating isn't listed",
			cr:       projectScoped,
			kubeObjs: []runtime.Object{referencedNamespace(nsID), referencedProject(cloudcommon.ResourceStateActivating)},
			assert: func(t *testing.T, got *tcloudv1alpha1.ServiceAccount) {
				require.Empty(t, got.Status.References.Projects)
				require.Len(t, got.Status.References.Namespaces, 1)
			},
		},
		{
			name:     "temporal cloud can't be read, so an error is returned",
			cr:       synced,
			kubeObjs: []runtime.Object{referencedNamespace(nsID)},
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetServiceAccountReturn(nil, status.Error(codes.PermissionDenied, "api key can't read service accounts")),
			},
			expErr: "failed getting tcloud service account",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReconciler(t, tc.kubeObjs, tc.serverOpts...)
			current := serviceaccount.NewResource(tc.cr.DeepCopy())
			before := current.DeepCopy()

			latest, err := r.ResourceLatest(context.TODO(), current)
			require.Equal(t, before.CR(), current.CR(), "current must not be mutated")
			require.NotNil(t, latest)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
				return
			}
			require.NoError(t, err)
			tc.assert(t, latest.CR().(*tcloudv1alpha1.ServiceAccount))
		})
	}
}
