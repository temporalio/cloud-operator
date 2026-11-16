package project_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudprojectv1 "go.temporal.io/cloud-sdk/api/project/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/project"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

// fixtures shared by the ResourceSync tests.
var (
	desired = newProject()

	// notCreated is latest for a project that doesn't exist in Temporal Cloud
	// yet.
	notCreated = func() *tcloudv1alpha1.Project {
		p := &tcloudv1alpha1.Project{}
		p.ObjectMeta = desired.ObjectMeta
		return p
	}()

	// active is latest for a project that matches desired.
	active = func() *tcloudv1alpha1.Project {
		p := desired.DeepCopy()
		p.Spec = tcloudv1alpha1.ProjectSpec{}.FromProto(cloudSpec())
		p.Status.Resource = cloudcommon.ResourceStatus{
			ID: projectID, Version: "rv-1", State: cloudcommon.ResourceStateActive,
		}
		return p
	}()
)

func TestReconciler_ResourceSync_Create(t *testing.T) {
	protected := desired.DeepCopy()
	protected.Spec.Lifecycle = &cloudcommon.LifecycleSpec{EnableDeleteProtection: true}

	// Empty exp fields mean "none": no error message to match and no ID saved.
	cases := []struct {
		name       string
		desired    *tcloudv1alpha1.Project
		serverOpts []fakeserver.NewOption
		expKind    string
		expErr     string
		expID      string
	}{
		{
			name:    "not created yet, so it's created and the new id is saved",
			desired: desired,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnCreateProject(
					func(req *tcloudservicev1.CreateProjectRequest) {
						assert.True(t, proto.Equal(cloudSpec(), req.GetSpec()), "%v", req.GetSpec())
					},
					func() (*tcloudservicev1.CreateProjectResponse, error) {
						return &tcloudservicev1.CreateProjectResponse{ProjectId: "project-new"}, nil
					},
				),
			},
			expKind: kindRequeue,
			expErr:  `created tcloud project "project-new"`,
			expID:   "project-new",
		},
		{
			name:    "lifecycle is sent on create",
			desired: protected,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnCreateProject(
					func(req *tcloudservicev1.CreateProjectRequest) {
						assert.True(t, req.GetSpec().GetLifecycle().GetEnableDeleteProtection())
					},
					func() (*tcloudservicev1.CreateProjectResponse, error) {
						return &tcloudservicev1.CreateProjectResponse{ProjectId: "project-new"}, nil
					},
				),
			},
			expKind: kindRequeue,
			expID:   "project-new",
		},
		{
			name:    "temporal cloud rejects the create, so it fails and saves no id",
			desired: desired,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnCreateProjectReturn(nil, status.Error(codes.InvalidArgument, "bad")),
			},
			expKind: kindTerminal,
			expErr:  "failed creating tcloud project",
		},
		{
			// Not Unavailable: the Temporal Cloud client retries that with
			// backoff, which makes the test take ~30s.
			name:    "the api key can't create projects, so it fails and saves no id",
			desired: desired,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnCreateProjectReturn(nil, status.Error(codes.PermissionDenied, "api key can't create projects")),
			},
			expKind: kindOther,
			expErr:  "failed creating tcloud project",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReconciler(t, tc.serverOpts...)
			latest := project.NewResource(notCreated.DeepCopy())
			before := latest.DeepCopy()

			got, err := r.ResourceSync(
				context.TODO(), project.NewResource(tc.desired.DeepCopy()), latest,
			)
			require.Equal(t, tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			}

			require.Equal(t, before.CR(), latest.CR(), "latest must not be mutated")
			require.NotNil(t, got)
			require.Equal(
				t, tc.expID, got.CR().(*tcloudv1alpha1.Project).Status.Resource.ID,
			)
		})
	}
}

func TestReconciler_ResourceSync(t *testing.T) {
	descriptionChanged := desired.DeepCopy()
	descriptionChanged.Spec.Description = "payments and billing"

	displayNameChanged := desired.DeepCopy()
	displayNameChanged.Spec.DisplayName = "Billing"

	protectionOff := desired.DeepCopy()
	protectionOff.Spec.Lifecycle = &cloudcommon.LifecycleSpec{}

	// Temporal Cloud returns an empty lifecycle block for a project whose CR
	// leaves lifecycle unset.
	emptyLifecycle := active.DeepCopy()
	emptyLifecycle.Spec.Lifecycle = &cloudcommon.LifecycleSpec{}

	protectedInCloud := active.DeepCopy()
	protectedInCloud.Spec.Lifecycle = &cloudcommon.LifecycleSpec{EnableDeleteProtection: true}

	activating := active.DeepCopy()
	activating.Status.Resource.State = cloudcommon.ResourceStateActivating

	updateFailed := active.DeepCopy()
	updateFailed.Status.Resource.State = cloudcommon.ResourceStateUpdateFailed

	// expectUpdate returns a fake server option that checks the update sends
	// the supplied spec for the active project.
	expectUpdate := func(spec *tcloudprojectv1.ProjectSpec) fakeserver.NewOption {
		return fakeserver.OnUpdateProject(
			func(req *tcloudservicev1.UpdateProjectRequest) {
				assert.Equal(t, projectID, req.GetProjectId())
				assert.Equal(t, "rv-1", req.GetResourceVersion())
				assert.True(t, proto.Equal(spec, req.GetSpec()), "%v", req.GetSpec())
			},
			nil,
		)
	}

	// Cases without an UpdateProject handler would fail with kindOther if
	// an update were sent, so kindNil also proves nothing was sent.
	cases := []struct {
		name       string
		desired    *tcloudv1alpha1.Project
		latest     *tcloudv1alpha1.Project
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
			name:    "temporal cloud returns an empty lifecycle the CR leaves unset, so nothing is sent",
			desired: desired,
			latest:  emptyLifecycle,
			expKind: kindNil,
		},
		{
			name:    "the CR sets protection off and temporal cloud omits lifecycle, so nothing is sent",
			desired: protectionOff,
			latest:  active,
			expKind: kindNil,
		},
		{
			name:    "the description changed, so an update is sent",
			desired: descriptionChanged,
			latest:  active,
			serverOpts: []fakeserver.NewOption{
				expectUpdate(&tcloudprojectv1.ProjectSpec{
					DisplayName: "Payments",
					Description: "payments and billing",
				}),
			},
			expKind: kindRequeue,
			expErr:  "waiting for update to complete",
		},
		{
			name:    "the display name changed, so an update is sent",
			desired: displayNameChanged,
			latest:  active,
			serverOpts: []fakeserver.NewOption{
				expectUpdate(&tcloudprojectv1.ProjectSpec{
					DisplayName: "Billing",
					Description: "payments team",
				}),
			},
			expKind: kindRequeue,
			expErr:  "waiting for update to complete",
		},
		{
			name:    "protection was turned on outside the operator, so an update turns it off",
			desired: desired,
			latest:  protectedInCloud,
			serverOpts: []fakeserver.NewOption{
				expectUpdate(cloudSpec()),
			},
			expKind: kindRequeue,
			expErr:  "waiting for update to complete",
		},
		{
			name:    "temporal cloud rejects the update as stale, so it requeues",
			desired: descriptionChanged,
			latest:  active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnUpdateProjectReturn(nil, status.Error(codes.FailedPrecondition, "stale resource version")),
			},
			expKind: kindRequeue,
			expErr:  "failed updating tcloud project",
		},
		{
			name:    "still activating, so it waits before updating",
			desired: descriptionChanged,
			latest:  activating,
			expKind: kindRequeue,
			expErr:  "to leave state Activating",
		},
		{
			name:    "a previous update failed, so it stops",
			desired: descriptionChanged,
			latest:  updateFailed,
			expKind: kindTerminal,
			expErr:  "contact Temporal Cloud support",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReconciler(t, tc.serverOpts...)
			latest := project.NewResource(tc.latest.DeepCopy())
			before := latest.DeepCopy()

			got, err := r.ResourceSync(
				context.TODO(), project.NewResource(tc.desired.DeepCopy()), latest,
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
