package project_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudprojectv1 "go.temporal.io/cloud-sdk/api/project/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/project"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

func TestReconciler_ResourceLatest(t *testing.T) {
	synced := newProject()
	synced.Status.Resource.ID = projectID

	fromCloud := &tcloudprojectv1.Project{
		Id:               projectID,
		ResourceVersion:  "rv-1",
		State:            tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		AsyncOperationId: "op-1",
		Spec: &tcloudprojectv1.ProjectSpec{
			DisplayName: "Payments (renamed in cloud)",
			Description: "changed outside the operator",
			Lifecycle:   &tcloudprojectv1.LifecycleSpec{EnableDeleteProtection: true},
		},
	}

	cases := []struct {
		name       string
		cr         *tcloudv1alpha1.Project
		serverOpts []fakeserver.NewOption
		expErr     string
		assert     func(t *testing.T, got *tcloudv1alpha1.Project)
	}{
		{
			name: "not created yet, so the spec and status are empty",
			cr:   newProject(),
			assert: func(t *testing.T, got *tcloudv1alpha1.Project) {
				require.Equal(t, tcloudv1alpha1.ProjectSpec{}, got.Spec)
				require.Equal(t, tcloudv1alpha1.ProjectStatus{}, got.Status)
			},
		},
		{
			name: "a resource-id annotation alone is treated as never created",
			cr: func() *tcloudv1alpha1.Project {
				p := newProject()
				p.Annotations = map[string]string{cloudcommon.AnnotationKeyResourceID: "project-existing"}
				return p
			}(),
			assert: func(t *testing.T, got *tcloudv1alpha1.Project) {
				require.Equal(t, tcloudv1alpha1.ProjectSpec{}, got.Spec)
				require.Empty(t, got.Status.Resource.ID)
			},
		},
		{
			name: "already created, so it's read from temporal cloud by its id",
			cr:   synced,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetProject(
					func(req *tcloudservicev1.GetProjectRequest) {
						assert.Equal(t, projectID, req.GetProjectId())
					},
					func() (*tcloudservicev1.GetProjectResponse, error) {
						return &tcloudservicev1.GetProjectResponse{Project: fromCloud}, nil
					},
				),
			},
			assert: func(t *testing.T, got *tcloudv1alpha1.Project) {
				require.Equal(t, tcloudv1alpha1.ProjectSpec{
					DisplayName: "Payments (renamed in cloud)",
					Description: "changed outside the operator",
					Lifecycle:   &cloudcommon.LifecycleSpec{EnableDeleteProtection: true},
				}, got.Spec)
				require.Equal(t, projectID, got.Status.Resource.ID)
				require.Equal(t, "rv-1", got.Status.Resource.Version)
				require.Equal(t, cloudcommon.ResourceStateActive, got.Status.Resource.State)
				require.Equal(t, "op-1", got.Status.Resource.AsyncOperationID)
				// Object metadata is kept from the CR.
				require.Equal(t, projectName, got.Name)
				require.Equal(t, kubeNamespace, got.Namespace)
			},
		},
		{
			name: "deleted outside the operator, so it's treated as not created",
			cr:   synced,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetProjectReturn(nil, status.Error(codes.NotFound, "gone")),
			},
			assert: func(t *testing.T, got *tcloudv1alpha1.Project) {
				require.Equal(t, tcloudv1alpha1.ProjectSpec{}, got.Spec)
				require.Empty(t, got.Status.Resource.ID)
			},
		},
		{
			name: "temporal cloud can't be read, so an error is returned",
			cr:   synced,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetProjectReturn(nil, status.Error(codes.PermissionDenied, "api key can't read projects")),
			},
			expErr: "failed getting tcloud project",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReconciler(t, tc.serverOpts...)
			current := project.NewResource(tc.cr.DeepCopy())
			before := current.DeepCopy()

			latest, err := r.ResourceLatest(context.TODO(), current)
			require.Equal(t, before.CR(), current.CR(), "current must not be mutated")
			require.NotNil(t, latest)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
				return
			}
			require.NoError(t, err)
			tc.assert(t, latest.CR().(*tcloudv1alpha1.Project))
		})
	}
}
