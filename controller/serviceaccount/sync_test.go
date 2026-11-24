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
	"github.com/temporalio/cloud-operator/pkg/types"
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

	projectResolved := notCreated.DeepCopy()
	projectResolved.Status.References.Projects = []cloudcommon.ResolvedReference{
		{Namespace: kubeNamespace, Name: "payments", ID: "p-1"},
	}

	customRoleRef := projectRef.DeepCopy()
	customRoleRef.Spec.ProjectScopedAccess = nil
	customRoleRef.Spec.AccountScopedAccess = &tcloudv1alpha1.AccountScopedAccess{
		AccountAccess: &tcloudv1alpha1.AccountAccess{
			CustomRoles: []cloudcommon.ResourceReferenceOrID{ref("auditor")},
		},
	}

	duplicateProjects := projectRef.DeepCopy()
	duplicateProjects.Spec.ProjectScopedAccess = nil
	duplicateProjects.Spec.AccountScopedAccess = &tcloudv1alpha1.AccountScopedAccess{
		ProjectAccesses: []tcloudv1alpha1.ProjectAccessEntry{
			{Project: ref("payments"), Access: tcloudv1alpha1.ProjectAccess{Role: tcloudv1alpha1.ProjectRoleRead}},
			{Project: id("p-1"), Access: tcloudv1alpha1.ProjectAccess{Role: tcloudv1alpha1.ProjectRoleAdmin}},
		},
	}

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
			name:    "a project ref has no id yet, so it waits without calling temporal cloud",
			desired: projectRef,
			latest:  notCreated,
			expKind: kindRequeue,
			expErr:  "spec.projectScopedAccess.project (apps/payments)",
		},
		{
			name:    "a resolved project ref is created with the project's id",
			desired: projectRef,
			latest:  projectResolved,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnCreateServiceAccount(
					func(req *tcloudservicev1.CreateServiceAccountRequest) {
						assert.Equal(t, "p-1", req.GetSpec().GetProjectScopedAccess().GetProjectId())
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
			name:    "custom role refs aren't supported yet, so it fails",
			desired: customRoleRef,
			latest:  notCreated,
			expKind: kindTerminal,
			expErr:  "spec.accountScopedAccess.accountAccess.customRoles[0]",
		},
		{
			name:    "a project ref and an id for the same project, so it fails",
			desired: duplicateProjects,
			latest:  projectResolved,
			expKind: kindTerminal,
			expErr:  `projectAccesses[0].project and spec.accountScopedAccess.projectAccesses[1].project both refer to tcloud project "p-1"`,
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

func TestReconciler_ResourceSync_Adopt(t *testing.T) {
	adoptDesired := newAdoptServiceAccount()

	// found is latest for the active service account to adopt.
	found := adoptDesired.DeepCopy()
	found.Spec = tcloudv1alpha1.ServiceAccountSpec{}.FromProto(cloudSpec())
	found.Status.Resource = cloudcommon.ResourceStatus{
		ID: saID, Version: "rv-1", State: cloudcommon.ResourceStateActive,
	}

	// linked is desired for an adoption that saved the ID but not adopted-on.
	linked := adoptDesired.DeepCopy()
	linked.Status.Resource.ID = saID

	adopted := linked.DeepCopy()
	adopted.Annotations[cloudcommon.AnnotationKeyAdoptedOn] = "2026-11-03T12:00:00Z"

	// gone is latest for a service account deleted outside of the operator.
	gone := adoptDesired.DeepCopy()

	// adoptedFound is both desired and latest once adoption has saved the
	// spec from Temporal Cloud into the CR.
	adoptedFound := found.DeepCopy()
	adoptedFound.Annotations[cloudcommon.AnnotationKeyAdoptedOn] = "2026-11-03T12:00:00Z"

	// Empty exp fields mean no error message to match, no ID saved and an
	// empty spec. No case sets a CreateServiceAccount or UpdateServiceAccount
	// handler, so a create or update would fail with kindOther.
	cases := []struct {
		name       string
		desired    *tcloudv1alpha1.ServiceAccount
		latest     *tcloudv1alpha1.ServiceAccount
		expKind    string
		expErr     string
		expID      string
		expAdopted bool
		expSpec    tcloudv1alpha1.ServiceAccountSpec
	}{
		{
			name:    "a half-finished adoption's service account is gone, so it requeues without creating one",
			desired: linked,
			latest:  gone,
			expKind: kindRequeue,
			expErr:  `no tcloud service account with ID "sa-1" to adopt`,
			expID:   saID,
		},
		{
			name:       "an adopted service account is gone, so it stops without recreating it",
			desired:    adopted,
			latest:     gone,
			expKind:    kindTerminal,
			expErr:     "no longer exists in Temporal Cloud",
			expID:      saID,
			expAdopted: true,
		},
		{
			name:       "the found service account is active, so it's adopted with its spec from temporal cloud",
			desired:    adoptDesired,
			latest:     found,
			expKind:    kindNil,
			expID:      saID,
			expAdopted: true,
			expSpec:    found.Spec,
		},
		{
			name:       "an adopted service account matches temporal cloud, so nothing is sent",
			desired:    adoptedFound,
			latest:     adoptedFound,
			expKind:    kindNil,
			expID:      saID,
			expAdopted: true,
			expSpec:    found.Spec,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReconciler(t, nil)
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
			gotRes := got.(types.Resource)
			require.Equal(t, tc.expID, gotRes.ResourceStatus().ID)
			require.Equal(t, tc.expAdopted, gotRes.IsAdopted())
			require.Equal(t, tc.expSpec, got.CR().(*tcloudv1alpha1.ServiceAccount).Spec)
		})
	}
}
