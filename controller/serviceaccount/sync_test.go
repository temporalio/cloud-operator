package serviceaccount_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/serviceaccount"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

// fixtures shared by the ResourceSync tests.
var (
	desired = newServiceAccount(ref(nsName))

	// notCreated is latest for a service account that doesn't exist in
	// Temporal Cloud yet, with its reference resolved.
	notCreated = func() *tcloudv1alpha1.ServiceAccount {
		sa := &tcloudv1alpha1.ServiceAccount{}
		sa.ObjectMeta = desired.ObjectMeta
		sa.Status.References = resolvedRefs(nsID)
		return sa
	}()

	// active is latest for a service account that matches desired.
	active = func() *tcloudv1alpha1.ServiceAccount {
		sa := desired.DeepCopy()
		sa.Spec = tcloudv1alpha1.ServiceAccountSpec{}.FromProto(cloudSpec())
		sa.Status.Resource = cloudcommon.ResourceStatus{
			ID: saID, Version: "rv-1", State: cloudcommon.ResourceStateActive,
		}
		sa.Status.References = resolvedRefs(nsID)
		return sa
	}()
)

func TestReconciler_ResourceSync_Create(t *testing.T) {
	projectRef := &tcloudv1alpha1.ServiceAccount{}
	projectRef.ObjectMeta = desired.ObjectMeta
	projectRef.Spec = tcloudv1alpha1.ServiceAccountSpec{
		DisplayName: "deployer",
		ProjectScopedAccess: &tcloudv1alpha1.ProjectScopedAccess{
			Project: ref("payments"),
			Access:  tcloudv1alpha1.ProjectAccess{Role: tcloudv1alpha1.ProjectRoleWrite},
		},
	}

	unresolved := notCreated.DeepCopy()
	unresolved.Status.References = tcloudv1alpha1.ServiceAccountReferences{}

	// Empty exp fields mean "none": no error message to match and no ID saved.
	cases := []struct {
		name       string
		desired    *tcloudv1alpha1.ServiceAccount
		latest     *tcloudv1alpha1.ServiceAccount
		serverOpts []fakeserver.NewOption
		expKind    string
		expErr     string
		expID      string
	}{
		{
			name:    "not created yet, so it's created and the new id is saved",
			desired: desired,
			latest:  notCreated,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnCreateServiceAccount(
					func(req *tcloudservicev1.CreateServiceAccountRequest) {
						assert.True(t, proto.Equal(cloudSpec(), req.GetSpec()), "%v", req.GetSpec())
					},
					func() (*tcloudservicev1.CreateServiceAccountResponse, error) {
						return &tcloudservicev1.CreateServiceAccountResponse{ServiceAccountId: "sa-new"}, nil
					},
				),
			},
			expKind: kindRequeue,
			expErr:  `created tcloud service account "sa-new"`,
			expID:   "sa-new",
		},
		{
			name:    "temporal cloud rejects the create, so it fails and saves no id",
			desired: desired,
			latest:  notCreated,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnCreateServiceAccountReturn(nil, status.Error(codes.InvalidArgument, "bad")),
			},
			expKind: kindTerminal,
			expErr:  "failed creating tcloud service account",
		},
		{
			name:    "a namespace ref has no id yet, so it waits without calling temporal cloud",
			desired: desired,
			latest:  unresolved,
			expKind: kindRequeue,
			expErr:  "spec.namespaceScopedAccess.namespace (apps/payments)",
		},
		{
			name:    "project refs aren't supported yet, so it fails",
			desired: projectRef,
			latest:  notCreated,
			expKind: kindTerminal,
			expErr:  "spec.projectScopedAccess.project",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReconciler(t, nil, tc.serverOpts...)
			latest := serviceaccount.NewResource(tc.latest.DeepCopy())
			before := latest.DeepCopy()

			got, err := r.ResourceSync(
				context.TODO(), serviceaccount.NewResource(tc.desired.DeepCopy()), latest,
			)
			require.Equal(t, tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			}

			require.Equal(t, before.CR(), latest.CR(), "latest must not be mutated")
			require.NotNil(t, got)
			require.Equal(
				t, tc.expID, got.CR().(*tcloudv1alpha1.ServiceAccount).Status.Resource.ID,
			)
		})
	}
}

func TestReconciler_ResourceSync(t *testing.T) {
	descriptionChanged := desired.DeepCopy()
	descriptionChanged.Spec.Description = "deploys payments"

	activating := active.DeepCopy()
	activating.Status.Resource.State = cloudcommon.ResourceStateActivating

	movedNamespace := active.DeepCopy()
	movedNamespace.Spec.NamespaceScopedAccess.Namespace = id("other.acct1")

	unresolved := active.DeepCopy()
	unresolved.Status.References = tcloudv1alpha1.ServiceAccountReferences{}

	cases := []struct {
		name       string
		desired    *tcloudv1alpha1.ServiceAccount
		latest     *tcloudv1alpha1.ServiceAccount
		serverOpts []fakeserver.NewOption
		expKind    string
		expErr     string
	}{
		{
			name:    "temporal cloud already matches, so nothing is sent",
			desired: desired,
			latest:  active,
			expKind: kindNil,
		},
		{
			name:    "the description changed, so an update is sent",
			desired: descriptionChanged,
			latest:  active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnUpdateServiceAccount(
					func(req *tcloudservicev1.UpdateServiceAccountRequest) {
						assert.Equal(t, saID, req.GetServiceAccountId())
						assert.Equal(t, "rv-1", req.GetResourceVersion())
						assert.Equal(t, "deploys payments", req.GetSpec().GetDescription())
					},
					nil,
				),
			},
			expKind: kindRequeue,
			expErr:  "waiting for update to complete",
		},
		{
			name:    "still activating, so it waits before updating",
			desired: descriptionChanged,
			latest:  activating,
			expKind: kindRequeue,
			expErr:  "to leave state Activating",
		},
		{
			name:    "a namespace ref has no id yet, so it waits without calling temporal cloud",
			desired: descriptionChanged,
			latest:  unresolved,
			expKind: kindRequeue,
			expErr:  "spec.namespaceScopedAccess.namespace (apps/payments)",
		},
		{
			name:    "moved to another namespace outside the operator, so it fails",
			desired: desired,
			latest:  movedNamespace,
			expKind: kindTerminal,
			expErr:  `is scoped to namespace "other.acct1"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReconciler(t, nil, tc.serverOpts...)
			latest := serviceaccount.NewResource(tc.latest.DeepCopy())
			before := latest.DeepCopy()

			got, err := r.ResourceSync(
				context.TODO(), serviceaccount.NewResource(tc.desired.DeepCopy()), latest,
			)
			require.Equal(t, tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			}
			require.Equal(t, before.CR(), latest.CR(), "latest must not be mutated")
			require.Equal(t, before.CR(), got.CR(), "update must not change status")
		})
	}
}
