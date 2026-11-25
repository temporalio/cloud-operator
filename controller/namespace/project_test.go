package namespace_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const moveTestAsyncOpID = "op-move"

// projectFakeCloudConfig sets the responses of a projectFakeCloud.
type projectFakeCloudConfig struct {
	createErr error
	moveErr   error
	opState   operationv1.AsyncOperation_State
	opFailure string
}

// projectFakeCloud is a started fake Temporal Cloud server that records the
// requests it receives. Its handlers run on the server goroutine, so they only
// record under the mutex, and the test goroutine asserts.
type projectFakeCloud struct {
	mu                sync.Mutex
	createNamespace   []*tcloudservicev1.CreateNamespaceRequest
	updateNamespace   []*tcloudservicev1.UpdateNamespaceRequest
	moveNamespace     []*tcloudservicev1.MoveNamespaceToProjectRequest
	getAsyncOperation []*tcloudservicev1.GetAsyncOperationRequest
}

// projectTestKubeObjects returns the Project CRs that the tests' refs name.
func projectTestKubeObjects() []runtime.Object {
	newProject := func(name, id string, state cloudcommon.ResourceState) *tcloudv1alpha1.Project {
		p := &tcloudv1alpha1.Project{}
		p.Namespace = kubeNamespace
		p.Name = name
		p.Status.Resource.ID = id
		p.Status.Resource.State = state
		return p
	}
	return []runtime.Object{
		newProject("payments", "proj-payments", cloudcommon.ResourceStateActive),
		newProject("activating", "proj-activating", cloudcommon.ResourceStateActivating),
	}
}

// syncWithProjectFakeCloud starts a projectFakeCloud configured by cfg, calls
// ResourceSync for desiredCR and latestCR, and checks that ResourceSync
// mutated neither of them.
func syncWithProjectFakeCloud(
	t *testing.T,
	cfg projectFakeCloudConfig,
	desiredCR *tcloudv1alpha1.Namespace,
	latestCR *tcloudv1alpha1.Namespace,
) (*projectFakeCloud, types.Resource, error) {
	t.Helper()
	require := require.New(t)

	c := &projectFakeCloud{}
	srv, err := fakeserver.New(
		fakeserver.WithCreateNamespace(func(
			_ context.Context, req *tcloudservicev1.CreateNamespaceRequest,
		) (*tcloudservicev1.CreateNamespaceResponse, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.createNamespace = append(c.createNamespace, req)
			if cfg.createErr != nil {
				return nil, cfg.createErr
			}
			return &tcloudservicev1.CreateNamespaceResponse{Namespace: "my-ns.acct1"}, nil
		}),
		fakeserver.WithUpdateNamespace(func(
			_ context.Context, req *tcloudservicev1.UpdateNamespaceRequest,
		) (*tcloudservicev1.UpdateNamespaceResponse, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.updateNamespace = append(c.updateNamespace, req)
			return &tcloudservicev1.UpdateNamespaceResponse{}, nil
		}),
		fakeserver.WithMoveNamespaceToProject(func(
			_ context.Context, req *tcloudservicev1.MoveNamespaceToProjectRequest,
		) (*tcloudservicev1.MoveNamespaceToProjectResponse, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.moveNamespace = append(c.moveNamespace, req)
			if cfg.moveErr != nil {
				return nil, cfg.moveErr
			}
			return &tcloudservicev1.MoveNamespaceToProjectResponse{
				AsyncOperation: &operationv1.AsyncOperation{Id: moveTestAsyncOpID},
			}, nil
		}),
		fakeserver.WithGetAsyncOperation(func(
			_ context.Context, req *tcloudservicev1.GetAsyncOperationRequest,
		) (*tcloudservicev1.GetAsyncOperationResponse, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.getAsyncOperation = append(c.getAsyncOperation, req)
			return &tcloudservicev1.GetAsyncOperationResponse{
				AsyncOperation: &operationv1.AsyncOperation{
					Id:            req.GetAsyncOperationId(),
					State:         cfg.opState,
					FailureReason: cfg.opFailure,
				},
			}, nil
		}),
	)
	require.NoError(err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(err)

	r := namespace.NewReconciler(base.WithDefaultCloudClient(*tcc))
	kc := kube.NewFakeClient(projectTestKubeObjects()...)
	r.SetKubeClient(kc)
	r.SetKubeReader(kc)

	desired := namespace.NewResource(desiredCR)
	latest := namespace.NewResource(latestCR)
	desiredBefore := desired.DeepCopy()
	latestBefore := latest.DeepCopy()

	got, err := r.ResourceSync(context.TODO(), desired, latest)

	require.Equal(desiredBefore, desired, "ResourceSync mutated desired")
	require.Equal(latestBefore, latest, "ResourceSync mutated latest")
	return c, got, err
}

// requireSyncError checks err against the expected requeue, terminal or plain
// error, or no error. expErr is a substring of the error message.
func requireSyncError(
	t *testing.T,
	err error,
	expErr string,
	expRequeue bool,
	expTerminal bool,
) {
	t.Helper()
	require := require.New(t)

	if expErr == "" {
		require.NoError(err)
		return
	}
	require.ErrorContains(err, expErr)
	var requeueAfter *requeue.RequeueNeededAfter
	require.Equal(expRequeue, errors.As(err, &requeueAfter), "requeue mismatch, got %v", err)
	var terminal *ctrlerrors.TerminalError
	require.Equal(expTerminal, errors.As(err, &terminal), "terminal mismatch, got %v", err)
}

func TestReconciler_ResourceSync_CreateProject(t *testing.T) {
	cases := []struct {
		name         string
		project      *cloudcommon.ResourceReferenceOrID
		createErr    error
		expErr       string
		expRequeue   bool
		expTerminal  bool
		expCreate    bool
		expProjectID string
	}{
		{
			name:         "fail if Temporal Cloud doesn't find the project id",
			project:      &cloudcommon.ResourceReferenceOrID{ID: "proj-missing"},
			createErr:    status.Error(codes.NotFound, "project not found"),
			expErr:       `in project "proj-missing"`,
			expTerminal:  true,
			expCreate:    true,
			expProjectID: "proj-missing",
		},
		{
			name:         "fail if projects aren't enabled for the account",
			project:      &cloudcommon.ResourceReferenceOrID{ID: "proj-a"},
			createErr:    status.Error(codes.PermissionDenied, "projects are not enabled for this account"),
			expErr:       "projects are not enabled for this account",
			expTerminal:  true,
			expCreate:    true,
			expProjectID: "proj-a",
		},
		{
			name:      "fail if Temporal Cloud denies a create without a project",
			createErr: status.Error(codes.PermissionDenied, "not allowed"),
			expErr:    "not allowed",
			expCreate: true,
		},
		{
			name: "fail if ref names a missing Project CR",
			project: &cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "nope"},
			},
			expErr:     "spec.project (temporal-cloud-operator/",
			expRequeue: true,
		},
		{
			name: "fail if ref names a project that is still activating",
			project: &cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "activating"},
			},
			expErr:     "spec.project (temporal-cloud-operator/",
			expRequeue: true,
		},
		{
			name:       "unset project yields an empty project ID",
			expErr:     "waiting for activation",
			expRequeue: true,
			expCreate:  true,
		},
		{
			name:         "id yields that project ID",
			project:      &cloudcommon.ResourceReferenceOrID{ID: "proj-a"},
			expErr:       "waiting for activation",
			expRequeue:   true,
			expCreate:    true,
			expProjectID: "proj-a",
		},
		{
			name: "ref yields the referenced project's ID",
			project: &cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "payments"},
			},
			expErr:       "waiting for activation",
			expRequeue:   true,
			expCreate:    true,
			expProjectID: "proj-payments",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			desiredCR := &tcloudv1alpha1.Namespace{}
			desiredCR.Name = "my-ns"
			desiredCR.Namespace = kubeNamespace
			desiredCR.Spec.Project = tc.project
			desiredCR.Spec.Replicas = []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}}
			desiredCR.Spec.RetentionDays = 30
			// ResourceLatest clears the spec and status when the Namespace
			// doesn't exist in Temporal Cloud.
			latestCR := &tcloudv1alpha1.Namespace{ObjectMeta: desiredCR.ObjectMeta}

			c, _, err := syncWithProjectFakeCloud(
				tt, projectFakeCloudConfig{createErr: tc.createErr}, desiredCR, latestCR,
			)

			requireSyncError(tt, err, tc.expErr, tc.expRequeue, tc.expTerminal)
			if !tc.expCreate {
				require.Empty(c.createNamespace, "CreateNamespace called")
				return
			}
			require.Len(c.createNamespace, 1)
			require.Equal(tc.expProjectID, c.createNamespace[0].GetProjectId())
		})
	}
}

func TestReconciler_ResourceSync_MoveProject(t *testing.T) {
	cases := []struct {
		name              string
		project           *cloudcommon.ResourceReferenceOrID
		generation        int64
		desiredRules      []string
		latestProject     string
		latestRules       []string
		latestMove        *tcloudv1alpha1.NamespaceProjectMove
		cloud             projectFakeCloudConfig
		expErr            string
		expRequeue        bool
		expTerminal       bool
		expUpdate         bool
		expMove           *tcloudservicev1.MoveNamespaceToProjectRequest
		expGetOperation   bool
		expProjectMove    *tcloudv1alpha1.NamespaceProjectMove
		expMoveFailureMsg string
	}{
		{
			name:          "fail if spec.connectivityRuleIDs is set",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-new"},
			desiredRules:  []string{"rule-1"},
			latestProject: "proj-old",
			latestRules:   []string{"rule-1"},
			expErr:        "remove the connectivity rules first",
			expTerminal:   true,
		},
		{
			name:          "fail if the last move to the project failed and the spec is unchanged",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-new"},
			generation:    2,
			latestProject: "proj-old",
			latestMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   "op-old",
				Generation:         2,
				Failure:            "project not found",
			},
			expErr:      "change the spec to retry",
			expTerminal: true,
			expProjectMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   "op-old",
				Generation:         2,
				Failure:            "project not found",
			},
		},
		{
			name:          "fail if Temporal Cloud denies the move",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-new"},
			latestProject: "proj-old",
			cloud: projectFakeCloudConfig{
				moveErr: status.Error(codes.PermissionDenied, "projects are not enabled"),
			},
			expErr:      "projects are not enabled",
			expTerminal: true,
			expMove: &tcloudservicev1.MoveNamespaceToProjectRequest{
				Namespace:               "my-ns.acct1",
				DestinationProjectId:    "proj-new",
				ExpectedSourceProjectId: "proj-old",
				ResourceVersion:         "rv-1",
			},
		},
		{
			name:          "fail if Temporal Cloud rejects the move's precondition",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-new"},
			latestProject: "proj-old",
			cloud: projectFakeCloudConfig{
				moveErr: status.Error(codes.FailedPrecondition, "resource version does not match"),
			},
			expErr:     "resource version does not match",
			expRequeue: true,
			expMove: &tcloudservicev1.MoveNamespaceToProjectRequest{
				Namespace:               "my-ns.acct1",
				DestinationProjectId:    "proj-new",
				ExpectedSourceProjectId: "proj-old",
				ResourceVersion:         "rv-1",
			},
		},
		{
			name:          "fail if the move's async operation fails",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-new"},
			generation:    2,
			latestProject: "proj-old",
			latestMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   moveTestAsyncOpID,
				Generation:         2,
			},
			cloud: projectFakeCloudConfig{
				opState:   operationv1.AsyncOperation_STATE_FAILED,
				opFailure: `project "proj-new" was not found`,
			},
			expErr:          `project "proj-new" was not found`,
			expTerminal:     true,
			expGetOperation: true,
			expProjectMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   moveTestAsyncOpID,
				Generation:         2,
			},
			expMoveFailureMsg: `project "proj-new" was not found`,
		},
		{
			name:          "project change yields a move",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-new"},
			generation:    2,
			latestProject: "proj-old",
			expErr:        "moving tcloud namespace",
			expRequeue:    true,
			expMove: &tcloudservicev1.MoveNamespaceToProjectRequest{
				Namespace:               "my-ns.acct1",
				DestinationProjectId:    "proj-new",
				ExpectedSourceProjectId: "proj-old",
				ResourceVersion:         "rv-1",
			},
			expProjectMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   moveTestAsyncOpID,
				Generation:         2,
			},
		},
		{
			name:          "namespace moved outside the operator yields a move back",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-old"},
			generation:    1,
			latestProject: "proj-other",
			expErr:        "moving tcloud namespace",
			expRequeue:    true,
			expMove: &tcloudservicev1.MoveNamespaceToProjectRequest{
				Namespace:               "my-ns.acct1",
				DestinationProjectId:    "proj-old",
				ExpectedSourceProjectId: "proj-other",
				ResourceVersion:         "rv-1",
			},
			expProjectMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-old",
				AsyncOperationID:   moveTestAsyncOpID,
				Generation:         1,
			},
		},
		{
			name: "ref yields a move to the referenced project",
			project: &cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "payments"},
			},
			generation:    1,
			latestProject: "proj-old",
			expErr:        "moving tcloud namespace",
			expRequeue:    true,
			expMove: &tcloudservicev1.MoveNamespaceToProjectRequest{
				Namespace:               "my-ns.acct1",
				DestinationProjectId:    "proj-payments",
				ExpectedSourceProjectId: "proj-old",
				ResourceVersion:         "rv-1",
			},
			expProjectMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-payments",
				AsyncOperationID:   moveTestAsyncOpID,
				Generation:         1,
			},
		},
		{
			name:          "failed move with a changed spec yields a new move",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-new"},
			generation:    3,
			latestProject: "proj-old",
			latestMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   "op-old",
				Generation:         2,
				Failure:            "project not found",
			},
			expErr:     "moving tcloud namespace",
			expRequeue: true,
			expMove: &tcloudservicev1.MoveNamespaceToProjectRequest{
				Namespace:               "my-ns.acct1",
				DestinationProjectId:    "proj-new",
				ExpectedSourceProjectId: "proj-old",
				ResourceVersion:         "rv-1",
			},
			expProjectMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   moveTestAsyncOpID,
				Generation:         3,
			},
		},
		{
			name:          "move in progress yields a requeue without a new move",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-other"},
			generation:    3,
			latestProject: "proj-old",
			latestMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   moveTestAsyncOpID,
				Generation:         2,
			},
			cloud: projectFakeCloudConfig{
				opState: operationv1.AsyncOperation_STATE_IN_PROGRESS,
			},
			expErr:          `waiting for tcloud namespace "my-ns.acct1" to move to project "proj-new"`,
			expRequeue:      true,
			expGetOperation: true,
			expProjectMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   moveTestAsyncOpID,
				Generation:         2,
			},
		},
		{
			name:          "finished move yields a cleared record",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-new"},
			generation:    2,
			latestProject: "proj-old",
			latestMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   moveTestAsyncOpID,
				Generation:         2,
			},
			cloud: projectFakeCloudConfig{
				opState: operationv1.AsyncOperation_STATE_FULFILLED,
			},
			expErr:          `moved tcloud namespace "my-ns.acct1" to project "proj-new"`,
			expRequeue:      true,
			expGetOperation: true,
		},
		{
			name:          "cleared rules with a project change yields an update before the move",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-new"},
			latestProject: "proj-old",
			latestRules:   []string{"rule-1"},
			expErr:        "waiting for update to complete",
			expRequeue:    true,
			expUpdate:     true,
		},
		{
			name:          "matching project yields a cleared failed record",
			project:       &cloudcommon.ResourceReferenceOrID{ID: "proj-old"},
			generation:    2,
			latestProject: "proj-old",
			latestMove: &tcloudv1alpha1.NamespaceProjectMove{
				DestinationProject: "proj-new",
				AsyncOperationID:   "op-old",
				Generation:         2,
				Failure:            "project not found",
			},
		},
		{
			name:          "unset project yields no move",
			latestProject: "proj-old",
		},
		{
			name: "ref to a missing Project CR yields no move",
			project: &cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "nope"},
			},
			latestProject: "proj-old",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			desiredCR := activeNamespace()
			desiredCR.Generation = tc.generation
			desiredCR.Spec.Project = tc.project
			desiredCR.Spec.ConnectivityRuleIDs = tc.desiredRules
			desiredCR.Status.ProjectMove = tc.latestMove.DeepCopy()
			latestCR := activeNamespace()
			latestCR.Generation = tc.generation
			latestCR.Spec.Project = tc.project
			latestCR.Spec.ConnectivityRuleIDs = tc.latestRules
			latestCR.Status.References.Project = tc.latestProject
			latestCR.Status.ProjectMove = tc.latestMove.DeepCopy()

			c, got, err := syncWithProjectFakeCloud(tt, tc.cloud, desiredCR, latestCR)

			requireSyncError(tt, err, tc.expErr, tc.expRequeue, tc.expTerminal)
			require.Equal(tc.expUpdate, len(c.updateNamespace) == 1, "UpdateNamespace calls: %d", len(c.updateNamespace))
			require.Equal(tc.expGetOperation, len(c.getAsyncOperation) == 1, "GetAsyncOperation calls: %d", len(c.getAsyncOperation))
			if tc.expMove == nil {
				require.Empty(c.moveNamespace, "MoveNamespaceToProject called")
			} else {
				require.Len(c.moveNamespace, 1)
				gotMove := c.moveNamespace[0]
				require.Equal(tc.expMove.GetNamespace(), gotMove.GetNamespace())
				require.Equal(tc.expMove.GetDestinationProjectId(), gotMove.GetDestinationProjectId())
				require.Equal(tc.expMove.GetExpectedSourceProjectId(), gotMove.GetExpectedSourceProjectId())
				require.Equal(tc.expMove.GetResourceVersion(), gotMove.GetResourceVersion())
				require.Nil(gotMove.GetDestinationConnectivityRules())
			}

			gotMove := got.CR().(*tcloudv1alpha1.Namespace).Status.ProjectMove.DeepCopy()
			if tc.expMoveFailureMsg != "" {
				require.NotNil(gotMove)
				require.Contains(gotMove.Failure, tc.expMoveFailureMsg)
				gotMove.Failure = ""
			}
			require.Equal(tc.expProjectMove, gotMove)
		})
	}
}
