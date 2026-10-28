package namespace_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const (
	deleteTestName      = "tmprl-ns"
	deleteTestID        = "tmprl-ns.acct"
	deleteTestRV        = "rv-1"
	deleteTestAsyncOpID = "async-op-1"
)

var (
	statusActive = tcloudcommon.ResourceStatus{
		ID:              deleteTestID,
		ResourceVersion: deleteTestRV,
		State:           tcloudcommon.ResourceStateActive,
	}

	statusDeleting = tcloudcommon.ResourceStatus{
		ID:               deleteTestID,
		ResourceVersion:  deleteTestRV,
		State:            tcloudcommon.ResourceStateDeleting,
		AsyncOperationID: deleteTestAsyncOpID,
	}

	statusDeleteFailed = tcloudcommon.ResourceStatus{
		ID:               deleteTestID,
		ResourceVersion:  deleteTestRV,
		State:            tcloudcommon.ResourceStateDeleteFailed,
		AsyncOperationID: deleteTestAsyncOpID,
	}

	deleteAccepted = &tcloudservicev1.DeleteNamespaceResponse{
		AsyncOperation: &operationv1.AsyncOperation{Id: deleteTestAsyncOpID},
	}
)

// deleteCase describes one call to ResourceDelete: the resource passed in,
// what the fake Temporal Cloud server returns, and what's expected back.
type deleteCase struct {
	name string
	// policy is the deletion-policy annotation value. Empty means no
	// annotation.
	policy string
	status tcloudcommon.ResourceStatus

	deleteResp *tcloudservicev1.DeleteNamespaceResponse
	deleteErr  error
	asyncOp    *operationv1.AsyncOperation

	// expDelete is whether DeleteNamespace is called (once) with the status
	// ID and resource version.
	expDelete bool
	// expGetAsyncOp is whether GetAsyncOperation is called (once) with the
	// status async operation ID.
	expGetAsyncOp bool
	expErr        string
	expRequeue    bool
	// expTerminal is whether the error is a terminal error.
	expTerminal bool
	// expStatus is the returned resource's Temporal Cloud status. Nil means
	// unchanged from status.
	expStatus *tcloudcommon.ResourceStatus
	// expStalled is a substring of the Stalled=True condition message. Empty
	// means no Stalled condition.
	expStalled string
}

func TestReconciler_ResourceDelete_DeletionPolicy(t *testing.T) {
	cases := []deleteCase{
		{
			name:   "retain makes no cloud calls",
			policy: "retain",
			status: statusActive,
		},
		{
			name:       "no annotation defaults to delete",
			status:     statusActive,
			deleteResp: deleteAccepted,
			expDelete:  true,
			expRequeue: true,
			expStatus:  &statusDeleting,
		},
		{
			name:       "delete calls DeleteNamespace",
			policy:     "delete",
			status:     statusActive,
			deleteResp: deleteAccepted,
			expDelete:  true,
			expRequeue: true,
			expStatus:  &statusDeleting,
		},
		{
			name:   "invalid policy errors without cloud calls",
			policy: "destroy",
			status: statusActive,
			expErr: `invalid deletion policy "destroy"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			runDeleteCase(tt, tc)
		})
	}
}

func TestReconciler_ResourceDelete_NoStatusID(t *testing.T) {
	cases := []deleteCase{
		{name: "delete policy makes no cloud calls", policy: "delete"},
		{name: "retain policy makes no cloud calls", policy: "retain"},
		{name: "invalid policy makes no cloud calls", policy: "destroy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			runDeleteCase(tt, tc)
		})
	}
}

func TestReconciler_ResourceDelete_SendDelete(t *testing.T) {
	cases := []deleteCase{
		{
			name:       "success sets deleting and requeues",
			status:     statusActive,
			deleteResp: deleteAccepted,
			expDelete:  true,
			expRequeue: true,
			expStatus:  &statusDeleting,
		},
		{
			name:      "NotFound means already gone",
			status:    statusActive,
			deleteErr: status.Error(codes.NotFound, "namespace not found"),
			expDelete: true,
		},
		{
			name:   "FailedPrecondition sets stalled",
			status: statusActive,
			deleteErr: status.Error(
				codes.FailedPrecondition, "delete protection is enabled",
			),
			expDelete:  true,
			expErr:     "delete protection is enabled",
			expStalled: "delete protection is enabled",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			runDeleteCase(tt, tc)
		})
	}
}

func TestReconciler_ResourceDelete_DeleteInProgress(t *testing.T) {
	cases := []deleteCase{
		{
			name:          "operation fulfilled",
			status:        statusDeleting,
			asyncOp:       asyncOperation(operationv1.AsyncOperation_STATE_FULFILLED, ""),
			expGetAsyncOp: true,
		},
		{
			name:          "operation pending requeues",
			status:        statusDeleting,
			asyncOp:       asyncOperation(operationv1.AsyncOperation_STATE_PENDING, ""),
			expGetAsyncOp: true,
			expRequeue:    true,
		},
		{
			name:   "operation failed sets delete failed and stalled",
			status: statusDeleting,
			asyncOp: asyncOperation(
				operationv1.AsyncOperation_STATE_FAILED, "nexus endpoints exist",
			),
			expGetAsyncOp: true,
			expErr:        "nexus endpoints exist",
			expTerminal:   true,
			expStatus:     &statusDeleteFailed,
			expStalled:    "nexus endpoints exist",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			runDeleteCase(tt, tc)
		})
	}
}

func TestReconciler_ResourceDelete_DeleteFailed(t *testing.T) {
	runDeleteCase(t, deleteCase{
		name:        "delete failed is not retried",
		status:      statusDeleteFailed,
		expErr:      "previously failed",
		expTerminal: true,
	})
}

func TestReconciler_ResourceDelete_InFlightStates(t *testing.T) {
	cases := []deleteCase{
		{
			name:       "activating requeues without cloud calls",
			status:     statusWithState(tcloudcommon.ResourceStateActivating),
			expRequeue: true,
		},
		{
			name:       "updating requeues without cloud calls",
			status:     statusWithState(tcloudcommon.ResourceStateUpdating),
			expRequeue: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			runDeleteCase(tt, tc)
		})
	}
}

// runDeleteCase calls ResourceDelete for tc against a fake Temporal Cloud
// server and checks the returned resource, the error, and the Cloud calls
// made.
func runDeleteCase(t *testing.T, tc deleteCase) {
	t.Helper()
	require := require.New(t)

	tns := newDeleteTestNamespace(tc.policy, tc.status)
	cloud := startDeleteFakeCloud(t, tc)
	r := newDeleteTestReconciler(cloud, tns)

	original := namespace.NewResource(tns.DeepCopy())
	before := original.DeepCopy()

	got, err := r.ResourceDelete(context.TODO(), original)

	requireNotMutated(t, before, original)
	require.NotNil(got)
	requireDeleteError(t, tc, err)

	gotCR := got.CR().(*tcloudv1alpha1.Namespace)
	expStatus := tc.status
	if tc.expStatus != nil {
		expStatus = *tc.expStatus
	}
	require.Equal(expStatus, gotCR.Status.ResourceStatus)

	requireStalled(t, got, tc.expStalled)
	cloud.requireCalls(t, tc.expDelete, tc.expGetAsyncOp)
}

// newDeleteTestNamespace returns a Namespace CR with the supplied
// deletion-policy annotation (none if empty) and Temporal Cloud status.
func newDeleteTestNamespace(
	policy string,
	rs tcloudcommon.ResourceStatus,
) *tcloudv1alpha1.Namespace {
	tns := &tcloudv1alpha1.Namespace{}
	tns.Name = deleteTestName
	tns.Namespace = kubeNamespace
	if policy != "" {
		tns.Annotations = map[string]string{
			tcloudcommon.AnnotationKeyDeletionPolicy: policy,
		}
	}
	tns.Status.ResourceStatus = rs
	return tns
}

// statusWithState returns statusActive with its state replaced.
func statusWithState(s tcloudcommon.ResourceState) tcloudcommon.ResourceStatus {
	rs := statusActive
	rs.State = s
	return rs
}

// asyncOperation returns the test async operation in the supplied state.
func asyncOperation(
	state operationv1.AsyncOperation_State,
	failureReason string,
) *operationv1.AsyncOperation {
	return &operationv1.AsyncOperation{
		Id:            deleteTestAsyncOpID,
		State:         state,
		FailureReason: failureReason,
	}
}

// deleteFakeCloud is a started fake Temporal Cloud server that records the
// requests it receives. Its handlers run on the server goroutine, so they only
// record under the mutex, and the test goroutine asserts.
type deleteFakeCloud struct {
	client *tcclient.Client

	mu                sync.Mutex
	getNamespace      int
	deleteNamespace   []*tcloudservicev1.DeleteNamespaceRequest
	getAsyncOperation []*tcloudservicev1.GetAsyncOperationRequest
}

// startDeleteFakeCloud starts a fake Temporal Cloud server that returns tc's
// DeleteNamespace and GetAsyncOperation responses, and stops it when the test
// ends.
func startDeleteFakeCloud(t *testing.T, tc deleteCase) *deleteFakeCloud {
	t.Helper()
	require := require.New(t)

	c := &deleteFakeCloud{}
	srv, err := fakeserver.New(
		fakeserver.WithGetNamespace(func(
			context.Context, *tcloudservicev1.GetNamespaceRequest,
		) (*tcloudservicev1.GetNamespaceResponse, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.getNamespace++
			return &tcloudservicev1.GetNamespaceResponse{}, nil
		}),
		fakeserver.WithDeleteNamespace(func(
			_ context.Context, req *tcloudservicev1.DeleteNamespaceRequest,
		) (*tcloudservicev1.DeleteNamespaceResponse, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.deleteNamespace = append(c.deleteNamespace, req)
			if tc.deleteErr != nil {
				return nil, tc.deleteErr
			}
			if tc.deleteResp == nil {
				return &tcloudservicev1.DeleteNamespaceResponse{}, nil
			}
			return tc.deleteResp, nil
		}),
		fakeserver.WithGetAsyncOperation(func(
			_ context.Context, req *tcloudservicev1.GetAsyncOperationRequest,
		) (*tcloudservicev1.GetAsyncOperationResponse, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.getAsyncOperation = append(c.getAsyncOperation, req)
			return &tcloudservicev1.GetAsyncOperationResponse{
				AsyncOperation: tc.asyncOp,
			}, nil
		}),
	)
	require.Nil(err)
	stop := srv.Start()
	t.Cleanup(stop)

	c.client, err = srv.Client()
	require.Nil(err)
	return c
}

// requireCalls checks which Temporal Cloud calls the server received.
// GetNamespace is never expected.
func (c *deleteFakeCloud) requireCalls(
	t *testing.T,
	expDelete bool,
	expGetAsyncOp bool,
) {
	t.Helper()
	require := require.New(t)

	c.mu.Lock()
	defer c.mu.Unlock()
	require.Zero(c.getNamespace)
	if expDelete {
		require.Len(c.deleteNamespace, 1)
		require.Equal(deleteTestID, c.deleteNamespace[0].GetNamespace())
		require.Equal(deleteTestRV, c.deleteNamespace[0].GetResourceVersion())
	} else {
		require.Empty(c.deleteNamespace)
	}
	if expGetAsyncOp {
		require.Len(c.getAsyncOperation, 1)
		require.Equal(
			deleteTestAsyncOpID, c.getAsyncOperation[0].GetAsyncOperationId(),
		)
	} else {
		require.Empty(c.getAsyncOperation)
	}
}

// newDeleteTestReconciler returns a Namespace Reconciler that talks to the
// fake Temporal Cloud server and a fake kube client holding tns.
func newDeleteTestReconciler(
	cloud *deleteFakeCloud,
	tns *tcloudv1alpha1.Namespace,
) *namespace.Reconciler {
	return namespace.NewReconciler(
		namespace.WithDefaultCloudClient(*cloud.client),
		namespace.WithKubeClient(kube.NewFakeClient(tns)),
	)
}

// requireNotMutated checks that ResourceDelete left the resource passed in
// unchanged.
func requireNotMutated(t *testing.T, before, original types.Resource) {
	t.Helper()
	require.Equal(t, before, original, "ResourceDelete mutated original")
}

// requireDeleteError checks err against tc's expected requeue, terminal or
// plain error, or no error.
func requireDeleteError(t *testing.T, tc deleteCase, err error) {
	t.Helper()
	require := require.New(t)

	var requeueAfter *requeue.RequeueNeededAfter
	switch {
	case tc.expRequeue:
		require.True(
			errors.As(err, &requeueAfter),
			"expected requeue error, got %v", err,
		)
		require.Equal(
			requeue.DefaultRequeueAfterDuration, requeueAfter.Duration(),
		)
	case tc.expErr != "":
		require.ErrorContains(err, tc.expErr)
		require.False(
			errors.As(err, &requeueAfter),
			"expected non-requeue error, got %v", err,
		)
		var terminal *ctrlerrors.TerminalError
		require.Equal(
			tc.expTerminal, errors.As(err, &terminal),
			"terminal error mismatch, got %T: %v", err, err,
		)
	default:
		require.NoError(err)
	}
}

// requireStalled checks the resource's Stalled condition. An empty msg means
// no Stalled condition is expected; otherwise Stalled is True with the delete
// failed reason and a message containing msg.
func requireStalled(t *testing.T, got types.Resource, msg string) {
	t.Helper()
	require := require.New(t)

	stalled := condition.Stalled(got)
	if msg == "" {
		require.Nil(stalled)
		return
	}
	require.NotNil(stalled)
	require.Equal(metav1.ConditionTrue, stalled.Status)
	require.Equal(tcloudcommon.ReasonResourceDeleteFailed, stalled.Reason)
	require.Contains(stalled.Message, msg)
}
