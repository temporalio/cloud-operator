package project_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	tcloudprojectv1 "go.temporal.io/cloud-sdk/api/project/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/project"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

func TestReconciler_ResourceDelete(t *testing.T) {
	const opID = "op-delete"

	resourceStatus := func(
		state cloudcommon.ResourceState,
		asyncOpID string,
	) *cloudcommon.ResourceStatus {
		return &cloudcommon.ResourceStatus{
			ID: projectID, Version: "rv-1", State: state, AsyncOperationID: asyncOpID,
		}
	}
	withStatus := func(
		state cloudcommon.ResourceState,
		asyncOpID string,
		annotations map[string]string,
	) *tcloudv1alpha1.Project {
		p := newProject()
		p.Annotations = annotations
		p.Status.Resource = *resourceStatus(state, asyncOpID)
		return p
	}
	active := withStatus(cloudcommon.ResourceStateActive, "", nil)
	protected := active.DeepCopy()
	protected.Spec.Lifecycle = &cloudcommon.LifecycleSpec{EnableDeleteProtection: true}

	opIn := func(s operationv1.AsyncOperation_State) fakeserver.NewOption {
		return fakeserver.OnGetAsyncOperationReturn(
			&tcloudservicev1.GetAsyncOperationResponse{
				AsyncOperation: &operationv1.AsyncOperation{Id: opID, State: s, FailureReason: "nope"},
			}, nil,
		)
	}
	// fromCloud returns the project as Temporal Cloud reports it, with a
	// spec that differs from the CR's so tests can tell which one was sent.
	fromCloud := func(s tcloudresourcev1.ResourceState, protected bool) *tcloudprojectv1.Project {
		return &tcloudprojectv1.Project{
			Id:              projectID,
			ResourceVersion: "rv-2",
			State:           s,
			Spec: &tcloudprojectv1.ProjectSpec{
				DisplayName: "Payments",
				Description: "changed in cloud",
				Lifecycle:   &tcloudprojectv1.LifecycleSpec{EnableDeleteProtection: protected},
			},
		}
	}
	getReturns := func(s tcloudresourcev1.ResourceState) fakeserver.NewOption {
		return fakeserver.OnGetProjectReturn(
			&tcloudservicev1.GetProjectResponse{Project: fromCloud(s, false)}, nil,
		)
	}
	activeInCloud := getReturns(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE)

	cases := []struct {
		name       string
		cr         *tcloudv1alpha1.Project
		serverOpts []fakeserver.NewOption
		deleteErr  error // returned by DeleteProject
		expKind    string
		expErr     string
		expDelete  bool
		expStatus  *cloudcommon.ResourceStatus // nil means the status is unchanged
		expStalled bool
	}{
		// Not linked to a project in Temporal Cloud.
		{
			name:    "never created in temporal cloud, so there's nothing to delete",
			cr:      newProject(),
			expKind: kindNil,
		},
		{
			name: "only a resource-id annotation, so nothing is deleted",
			cr: func() *tcloudv1alpha1.Project {
				p := newProject()
				p.Annotations = map[string]string{cloudcommon.AnnotationKeyResourceID: "project-existing"}
				return p
			}(),
			expKind: kindNil,
		},

		// The deletion policy.
		{
			name: "deletion policy is retain, so the project is kept",
			cr: withStatus(cloudcommon.ResourceStateActive, "", map[string]string{
				cloudcommon.AnnotationKeyDeletionPolicy: "Retain",
			}),
			expKind: kindNil,
		},
		{
			name: "deletion policy is invalid, so it stalls until it's fixed",
			cr: withStatus(cloudcommon.ResourceStateActive, "", map[string]string{
				cloudcommon.AnnotationKeyDeletionPolicy: "destroy",
			}),
			expKind:    kindRequeue,
			expErr:     `invalid deletion policy "destroy"`,
			expStalled: true,
		},

		// An earlier delete decides what happens, so no new delete is sent.
		{
			name:    "already deleted, so it's done",
			cr:      withStatus(cloudcommon.ResourceStateDeleted, opID, nil),
			expKind: kindNil,
		},
		{
			name:    "an earlier delete failed, so it isn't retried",
			cr:      withStatus(cloudcommon.ResourceStateDeleteFailed, opID, nil),
			expKind: kindTerminal,
			expErr:  "not retrying",
		},
		{
			name:       "the delete is still running, so it waits",
			cr:         withStatus(cloudcommon.ResourceStateDeleting, opID, nil),
			serverOpts: []fakeserver.NewOption{opIn(operationv1.AsyncOperation_STATE_IN_PROGRESS)},
			expKind:    kindRequeue,
		},
		{
			name:       "the delete finished, so it's done",
			cr:         withStatus(cloudcommon.ResourceStateDeleting, opID, nil),
			serverOpts: []fakeserver.NewOption{opIn(operationv1.AsyncOperation_STATE_FULFILLED)},
			expKind:    kindNil,
		},
		{
			name:       "the delete failed in temporal cloud, so it fails and stalls",
			cr:         withStatus(cloudcommon.ResourceStateDeleting, opID, nil),
			serverOpts: []fakeserver.NewOption{opIn(operationv1.AsyncOperation_STATE_FAILED)},
			expKind:    kindTerminal,
			expErr:     "deleting tcloud project project-1 failed",
			expStatus:  resourceStatus(cloudcommon.ResourceStateDeleteFailed, opID),
			expStalled: true,
		},

		// Deletion protection.
		{
			name:       "the CR enables deletion protection, so it stalls without calling temporal cloud",
			cr:         protected,
			expKind:    kindRequeue,
			expErr:     "has deletion protection enabled",
			expStalled: true,
		},
		{
			name: "protection is still on in temporal cloud, so it's turned off before deleting",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetProjectReturn(&tcloudservicev1.GetProjectResponse{
					Project: fromCloud(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE, true),
				}, nil),
				fakeserver.OnUpdateProject(
					func(req *tcloudservicev1.UpdateProjectRequest) {
						assert.Equal(t, projectID, req.GetProjectId())
						assert.Equal(t, "rv-2", req.GetResourceVersion())
						assert.False(t, req.GetSpec().GetLifecycle().GetEnableDeleteProtection())
						// Only protection changes: the rest of the spec is
						// Temporal Cloud's, not the CR's.
						assert.Equal(t, "changed in cloud", req.GetSpec().GetDescription())
					},
					nil,
				),
			},
			expKind: kindRequeue,
			expErr:  "turned off deletion protection on tcloud project project-1",
		},
		{
			name: "temporal cloud rejects turning protection off, so an error is returned",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetProjectReturn(&tcloudservicev1.GetProjectResponse{
					Project: fromCloud(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE, true),
				}, nil),
				fakeserver.OnUpdateProjectReturn(
					nil, status.Error(codes.PermissionDenied, "api key can't update projects"),
				),
			},
			expKind: kindOther,
			expErr:  "failed turning off tcloud project deletion protection",
		},

		// The state in Temporal Cloud decides whether the delete request can proceed.
		{
			name:       "still activating in temporal cloud, so it waits before deleting",
			cr:         withStatus(cloudcommon.ResourceStateActivating, "", nil),
			serverOpts: []fakeserver.NewOption{getReturns(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATING)},
			expKind:    kindRequeue,
			expErr:     "to finish Activating before deleting",
		},
		{
			name:       "still updating in temporal cloud, so it waits and saves the state",
			cr:         active,
			serverOpts: []fakeserver.NewOption{getReturns(tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING)},
			expKind:    kindRequeue,
			expErr:     "to finish Updating before deleting",
			expStatus:  resourceStatus(cloudcommon.ResourceStateUpdating, ""),
		},
		{
			name:       "status still says activating but temporal cloud says active, so the delete is sent",
			cr:         withStatus(cloudcommon.ResourceStateActivating, "", nil),
			serverOpts: []fakeserver.NewOption{activeInCloud},
			expKind:    kindRequeue,
			expErr:     "deleting tcloud project project-1 (async operation op-delete)",
			expDelete:  true,
			expStatus:  resourceStatus(cloudcommon.ResourceStateDeleting, opID),
		},

		// Sending the delete.
		{
			name:       "active, so the delete is sent with the current resource version",
			cr:         active,
			serverOpts: []fakeserver.NewOption{activeInCloud},
			expKind:    kindRequeue,
			expErr:     "deleting tcloud project project-1 (async operation op-delete)",
			expDelete:  true,
			expStatus:  resourceStatus(cloudcommon.ResourceStateDeleting, opID),
		},
		{
			name: "already gone from temporal cloud, so it's done",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetProjectReturn(nil, status.Error(codes.NotFound, "gone")),
			},
			expKind: kindNil,
		},
		{
			name: "temporal cloud can't be read, so an error is returned",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetProjectReturn(
					nil, status.Error(codes.PermissionDenied, "api key can't read projects"),
				),
			},
			expKind: kindOther,
			expErr:  "failed getting tcloud project project-1",
		},
		{
			name:       "gone by the time the delete is sent, so it's done",
			cr:         active,
			serverOpts: []fakeserver.NewOption{activeInCloud},
			deleteErr:  status.Error(codes.NotFound, "gone"),
			expKind:    kindNil,
			expDelete:  true,
		},
		{
			name:       "the project still has namespaces, so the delete is rejected and it stalls",
			cr:         active,
			serverOpts: []fakeserver.NewOption{activeInCloud},
			deleteErr:  status.Error(codes.FailedPrecondition, "project has namespaces"),
			expKind:    kindOther,
			expErr:     "failed deleting tcloud project project-1",
			expDelete:  true,
			expStalled: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var deleted atomic.Bool
			sopts := append(tc.serverOpts, fakeserver.OnDeleteProject(
				func(req *tcloudservicev1.DeleteProjectRequest) {
					deleted.Store(true)
					assert.Equal(t, projectID, req.GetProjectId())
					assert.Equal(t, "rv-2", req.GetResourceVersion())
				},
				func() (*tcloudservicev1.DeleteProjectResponse, error) {
					if tc.deleteErr != nil {
						return nil, tc.deleteErr
					}
					return &tcloudservicev1.DeleteProjectResponse{
						AsyncOperation: &operationv1.AsyncOperation{Id: opID},
					}, nil
				},
			))
			r := newReconciler(t, sopts...)
			original := project.NewResource(tc.cr.DeepCopy())
			before := original.DeepCopy()

			got, err := r.ResourceDelete(context.TODO(), original)
			require.Equal(t, tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			}
			require.Equal(t, before.CR(), original.CR(), "original must not be mutated")
			require.Equal(t, tc.expDelete, deleted.Load())

			expStatus := tc.cr.Status.Resource
			if tc.expStatus != nil {
				expStatus = *tc.expStatus
			}
			cr := got.CR().(*tcloudv1alpha1.Project)
			require.Equal(t, expStatus, cr.Status.Resource)
			stalled := condition.Stalled(got)
			if tc.expStalled {
				require.NotNil(t, stalled)
				require.Equal(t, metav1.ConditionTrue, stalled.Status)
			} else {
				require.Nil(t, stalled)
			}
		})
	}
}
