package cloud_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

// Kinds of error the controller treats differently, as returned by errKind.
const (
	kindNil      = "nil"
	kindRequeue  = "requeue"
	kindTerminal = "terminal"
	kindOther    = "other"
)

// errKind classifies err the way the kube-temporal controller does.
func errKind(err error) string {
	var terminal *ctrlerrors.TerminalError
	var requeueAfter *requeue.RequeueNeededAfter
	switch {
	case err == nil:
		return kindNil
	case errors.As(err, &terminal):
		return kindTerminal
	case errors.As(err, &requeueAfter):
		return kindRequeue
	default:
		return kindOther
	}
}

func TestRequireActive(t *testing.T) {
	cases := []struct {
		state   cloudcommon.ResourceState
		expKind string
	}{
		{cloudcommon.ResourceStateActive, kindNil},
		{cloudcommon.ResourceStateActivating, kindRequeue},
		{cloudcommon.ResourceStateUpdating, kindRequeue},
		{cloudcommon.ResourceStateDeleting, kindRequeue},
		{cloudcommon.ResourceStateDeleted, kindRequeue},
		{cloudcommon.ResourceStateActivationFailed, kindTerminal},
		{cloudcommon.ResourceStateUpdateFailed, kindTerminal},
		{cloudcommon.ResourceStateDeleteFailed, kindTerminal},
		{cloudcommon.ResourceStateSuspended, kindTerminal},
		{cloudcommon.ResourceStateExpired, kindTerminal},
		{"SomethingNew", kindRequeue},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			err := cloud.RequireActive("service account", "sa-1", tc.state)
			require.Equal(t, tc.expKind, errKind(err))
			if err != nil {
				require.ErrorContains(t, err, `tcloud service account "sa-1"`)
			}
		})
	}
}

func TestCheckAsyncOperation(t *testing.T) {
	const opID = "op-1"
	opIn := func(s operationv1.AsyncOperation_State) *tcloudservicev1.GetAsyncOperationResponse {
		return &tcloudservicev1.GetAsyncOperationResponse{
			AsyncOperation: &operationv1.AsyncOperation{
				Id:            opID,
				State:         s,
				FailureReason: "quota exceeded",
			},
		}
	}

	cases := []struct {
		name      string
		resp      *tcloudservicev1.GetAsyncOperationResponse
		respErr   error
		expKind   string
		expFailed bool
		expErr    string
	}{
		{
			name:    "fulfilled",
			resp:    opIn(operationv1.AsyncOperation_STATE_FULFILLED),
			expKind: kindNil,
		},
		{
			name:    "pending waits",
			resp:    opIn(operationv1.AsyncOperation_STATE_PENDING),
			expKind: kindRequeue,
		},
		{
			name:    "in progress waits",
			resp:    opIn(operationv1.AsyncOperation_STATE_IN_PROGRESS),
			expKind: kindRequeue,
		},
		{
			name:      "failed is terminal with the reason",
			resp:      opIn(operationv1.AsyncOperation_STATE_FAILED),
			expKind:   kindTerminal,
			expFailed: true,
			expErr:    "quota exceeded",
		},
		{
			name:      "rejected is terminal",
			resp:      opIn(operationv1.AsyncOperation_STATE_REJECTED),
			expKind:   kindTerminal,
			expFailed: true,
		},
		{
			name:      "cancelled is terminal",
			resp:      opIn(operationv1.AsyncOperation_STATE_CANCELLED),
			expKind:   kindTerminal,
			expFailed: true,
		},
		{
			name:    "read error is a plain error",
			respErr: fmt.Errorf("boom"),
			expKind: kindOther,
			expErr:  "failed getting tcloud async operation op-1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)

			srv, err := fakeserver.New(
				fakeserver.OnGetAsyncOperation(
					func(req *tcloudservicev1.GetAsyncOperationRequest) {
						assert.Equal(t, opID, req.GetAsyncOperationId())
					},
					func() (*tcloudservicev1.GetAsyncOperationResponse, error) {
						return tc.resp, tc.respErr
					},
				),
			)
			require.NoError(err)
			t.Cleanup(srv.Start())
			tcc, err := srv.Client()
			require.NoError(err)

			err = cloud.CheckAsyncOperation(context.TODO(), tcc.CloudService(), opID)
			require.Equal(tc.expKind, errKind(err))
			require.Equal(tc.expFailed, errors.Is(err, cloud.ErrAsyncOperationFailed))
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
			}
		})
	}
}

func TestCallError(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		expKind string
	}{
		{
			name:    "invalid argument is terminal",
			err:     status.Error(codes.InvalidArgument, "bad spec"),
			expKind: kindTerminal,
		},
		{
			name:    "already exists requeues",
			err:     status.Error(codes.AlreadyExists, "exists"),
			expKind: kindRequeue,
		},
		{
			name:    "failed precondition requeues",
			err:     status.Error(codes.FailedPrecondition, "stale version"),
			expKind: kindRequeue,
		},
		{
			name:    "aborted requeues",
			err:     status.Error(codes.Aborted, "conflict"),
			expKind: kindRequeue,
		},
		{
			name:    "anything else is a plain error",
			err:     status.Error(codes.Unavailable, "down"),
			expKind: kindOther,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cloud.CallError("creating thing", tc.err)
			require.Equal(t, tc.expKind, errKind(got))
			require.True(t, errors.Is(got, ctrlerrors.ErrResourceSync))
			require.ErrorContains(t, got, "failed creating thing")
			require.ErrorIs(t, got, tc.err)
		})
	}
}
