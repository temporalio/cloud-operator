package common_test

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
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/common"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
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

func TestResolveNamespaceID(t *testing.T) {
	const referrerNamespace = "apps"

	newNamespace := func(
		kubeNamespace, name, id string,
		state cloudcommon.ResourceState,
	) *tcloudv1alpha1.Namespace {
		ns := &tcloudv1alpha1.Namespace{}
		ns.Namespace = kubeNamespace
		ns.Name = name
		ns.Status.Resource.ID = id
		ns.Status.Resource.State = state
		return ns
	}
	kc := kube.NewFakeClient(
		newNamespace(referrerNamespace, "payments", "payments.acct1", cloudcommon.ResourceStateActive),
		newNamespace("shared", "billing", "billing.acct1", cloudcommon.ResourceStateActive),
		newNamespace(referrerNamespace, "pending", "", ""),
		newNamespace(referrerNamespace, "activating", "activating.acct1", cloudcommon.ResourceStateActivating),
	)

	cases := []struct {
		name        string
		refOrID     cloudcommon.ResourceReferenceOrID
		exp         string
		expNotReady bool
	}{
		{
			name:    "id is returned as is",
			refOrID: cloudcommon.ResourceReferenceOrID{ID: "legacy.acct1"},
			exp:     "legacy.acct1",
		},
		{
			name: "ref without namespace uses the referrer's",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "payments"},
			},
			exp: "payments.acct1",
		},
		{
			name: "ref with namespace",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Namespace: "shared", Name: "billing"},
			},
			exp: "billing.acct1",
		},
		{
			name: "ref to a missing custom resource is not ready",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "nope"},
			},
			expNotReady: true,
		},
		{
			name: "ref to a custom resource without an ID is not ready",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "pending"},
			},
			expNotReady: true,
		},
		{
			name: "ref to a namespace that is still activating is not ready",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "activating"},
			},
			expNotReady: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := common.ResolveNamespaceID(context.TODO(), kc, referrerNamespace, tc.refOrID)
			if tc.expNotReady {
				require.True(t, errors.Is(err, common.ErrReferenceNotReady), "got %v", err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.exp, got)
		})
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
			err := common.RequireActive("service account", "sa-1", tc.state)
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

			err = common.CheckAsyncOperation(context.TODO(), tcc.CloudService(), opID)
			require.Equal(tc.expKind, errKind(err))
			require.Equal(tc.expFailed, errors.Is(err, common.ErrAsyncOperationFailed))
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
			}
		})
	}
}

func TestCloudCallError(t *testing.T) {
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
			got := common.CloudCallError("creating thing", tc.err)
			require.Equal(t, tc.expKind, errKind(got))
			require.True(t, errors.Is(got, ctrlerrors.ErrResourceSync))
			require.ErrorContains(t, got, "failed creating thing")
			require.ErrorIs(t, got, tc.err)
		})
	}
}

func TestReferenceCheck(t *testing.T) {
	ref := func(namespace, name string) cloudcommon.ResourceReferenceOrID {
		return cloudcommon.ResourceReferenceOrID{
			Ref: &cloudcommon.ResourceReference{Namespace: namespace, Name: name},
		}
	}
	type pending struct {
		path     string
		refOrID  cloudcommon.ResourceReferenceOrID
		resolved string
	}
	type unsupported struct {
		path    string
		refOrID cloudcommon.ResourceReferenceOrID
	}

	cases := []struct {
		name        string
		pending     []pending
		unsupported []unsupported
		expKind     string
		expErr      string
	}{
		{
			name:    "nothing recorded",
			expKind: kindNil,
		},
		{
			name: "ids and resolved refs are fine",
			pending: []pending{
				{path: "spec.a", refOrID: cloudcommon.ResourceReferenceOrID{ID: "a.acct"}},
				{path: "spec.b", refOrID: ref("", "b"), resolved: "b.acct"},
			},
			unsupported: []unsupported{
				{path: "spec.c", refOrID: cloudcommon.ResourceReferenceOrID{ID: "p-1"}},
			},
			expKind: kindNil,
		},
		{
			name: "unresolved ref waits and names it",
			pending: []pending{
				{path: "spec.a", refOrID: ref("", "payments")},
				{path: "spec.b", refOrID: ref("shared", "billing")},
			},
			expKind: kindRequeue,
			expErr:  "spec.a (apps/payments), spec.b (shared/billing)",
		},
		{
			name: "unsupported ref is terminal, even with pending refs",
			pending: []pending{
				{path: "spec.a", refOrID: ref("", "payments")},
			},
			unsupported: []unsupported{
				{path: "spec.project", refOrID: ref("", "payments")},
			},
			expKind: kindTerminal,
			expErr:  "set id instead: spec.project",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := common.NewReferenceCheck("apps")
			for _, p := range tc.pending {
				check.Pending(p.path, p.refOrID, p.resolved)
			}
			for _, u := range tc.unsupported {
				check.Unsupported(u.path, u.refOrID)
			}

			err := check.Err()
			require.Equal(t, tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			}
		})
	}
}
