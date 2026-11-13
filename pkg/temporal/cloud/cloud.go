// Package cloud contains Temporal Cloud logic shared by the controllers.
package cloud

import (
	"context"
	"errors"
	"fmt"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// ErrAsyncOperationFailed is wrapped by the error CheckAsyncOperation returns
// when the operation failed, was rejected or was cancelled.
var ErrAsyncOperationFailed = errors.New("tcloud async operation failed")

// RequireActive returns nil when a Temporal Cloud resource in the supplied
// state accepts new operations, which is only when it is Active.
func RequireActive(
	kind string,
	id string,
	state cloudcommon.ResourceState,
) error {
	switch state {
	case cloudcommon.ResourceStateActive:
		return nil
	case cloudcommon.ResourceStateActivating, cloudcommon.ResourceStateUpdating:
		return requeue.NeededAfter(
			fmt.Errorf("waiting for tcloud %s %q to leave state %s", kind, id, state),
			requeue.DefaultRequeueAfterDuration,
		)
	case cloudcommon.ResourceStateDeleting, cloudcommon.ResourceStateDeleted:
		// The resource was deleted outside of the operator. Once Temporal
		// Cloud reports it as not found, ResourceLatest clears the status
		// and the next sync recreates it.
		return requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud %s %q to finish deleting, state %s",
				kind, id, state,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	case cloudcommon.ResourceStateActivationFailed,
		cloudcommon.ResourceStateUpdateFailed,
		cloudcommon.ResourceStateDeleteFailed,
		cloudcommon.ResourceStateSuspended,
		cloudcommon.ResourceStateExpired:
		return ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: tcloud %s %q is in state %s, contact Temporal Cloud support",
				ctrlerrors.ErrResourceSync, kind, id, state,
			),
		)
	default:
		return requeue.NeededAfter(
			fmt.Errorf("waiting for tcloud %s %q to leave unknown state %q", kind, id, state),
			requeue.DefaultRequeueAfterDuration,
		)
	}
}

// CheckAsyncOperation checks a Temporal Cloud async operation. It returns nil
// once the operation is fulfilled and a requeue error while it is pending or
// in progress.
func CheckAsyncOperation(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	opID string,
) error {
	resp, err := tcc.GetAsyncOperation(ctx, &tcloudservicev1.GetAsyncOperationRequest{
		AsyncOperationId: opID,
	})
	if err != nil {
		return fmt.Errorf("failed getting tcloud async operation %s: %w", opID, err)
	}
	op := resp.GetAsyncOperation()
	switch op.GetState() {
	case operationv1.AsyncOperation_STATE_FULFILLED:
		return nil
	case operationv1.AsyncOperation_STATE_FAILED,
		operationv1.AsyncOperation_STATE_REJECTED,
		operationv1.AsyncOperation_STATE_CANCELLED:
		return ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: async operation %s is %s: %s",
				ErrAsyncOperationFailed, op.GetId(), op.GetState(), op.GetFailureReason(),
			),
		)
	default:
		// Pending, in progress, or a state this SDK doesn't know.
		return requeue.NeededAfter(
			fmt.Errorf("waiting for tcloud async operation %s (%s)", op.GetId(), op.GetState()),
			requeue.DefaultRequeueAfterDuration,
		)
	}
}

// CallError wraps an error returned from a Temporal Cloud call in
// [ctrlerrors.ErrResourceSync] and classifies it for the controller.
func CallError(action string, err error) error {
	wrapped := fmt.Errorf("%w: failed %s: %w", ctrlerrors.ErrResourceSync, action, err)
	switch status.Code(err) {
	case codes.InvalidArgument:
		// The request itself is bad and retrying will not help until the spec
		// changes.
		return ctrlerrors.NewTerminalError(wrapped)
	case codes.AlreadyExists, codes.FailedPrecondition, codes.Aborted:
		// Our view of the resource is stale (e.g. an outdated resource
		// version). The next reconcile re-reads it.
		return requeue.NeededAfter(wrapped, requeue.DefaultRequeueAfterDuration)
	}
	return wrapped
}
