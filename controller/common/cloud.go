// Package common contains reconciler logic shared by the controllers.
package common

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

var (
	// ErrReferenceNotReady is wrapped by the error ResolveID returns when the
	// referenced custom resource doesn't exist or isn't ready in Temporal
	// Cloud yet.
	ErrReferenceNotReady = errors.New("referenced resource not ready")

	// ErrAsyncOperationFailed is wrapped by the error CheckAsyncOperation
	// returns when the operation failed, was rejected or was cancelled.
	ErrAsyncOperationFailed = errors.New("tcloud async operation failed")
)

// ResolveID returns the Temporal Cloud ID that `refOrID` refers to. `cloudID`
// returns the ID of `obj`, or an empty string when it isn't ready yet.
func ResolveID[T client.Object](
	ctx context.Context,
	kr client.Reader,
	referrerNamespace string,
	refOrID cloudcommon.ResourceReferenceOrID,
	obj T,
	cloudID func(T) string,
) (string, error) {
	if refOrID.Ref == nil {
		return refOrID.ID, nil
	}

	key := client.ObjectKey{Namespace: refOrID.Ref.Namespace, Name: refOrID.Ref.Name}
	if key.Namespace == "" {
		key.Namespace = referrerNamespace
	}
	if kr == nil {
		return "", fmt.Errorf("failed reading %s: nil kube reader", key)
	}

	err := kr.Get(ctx, key, obj)
	if apierrors.IsNotFound(err) {
		return "", fmt.Errorf("%w: %s not found", ErrReferenceNotReady, key)
	}
	if err != nil {
		return "", fmt.Errorf("failed reading %s: %w", key, err)
	}

	id := cloudID(obj)
	if id == "" {
		return "", fmt.Errorf("%w: %s isn't ready in Temporal Cloud yet", ErrReferenceNotReady, key)
	}
	return id, nil
}

// ResolveNamespaceID returns the Temporal Cloud ID of the Namespace that refOrID
// refers to, once that Namespace is Active.
func ResolveNamespaceID(
	ctx context.Context,
	kr client.Reader,
	referrerNamespace string,
	refOrID cloudcommon.ResourceReferenceOrID,
) (string, error) {
	return ResolveID(
		ctx, kr, referrerNamespace, refOrID,
		&tcloudv1alpha1.Namespace{},
		func(ns *tcloudv1alpha1.Namespace) string {
			// Temporal Cloud rejects a service account for a namespace that is
			// still activating.
			if ns.Status.Resource.State != cloudcommon.ResourceStateActive {
				return ""
			}
			return ns.Status.Resource.ID
		},
	)
}

// ReferenceCheck collects the references in a spec that can't be sent to
// Temporal Cloud yet.
type ReferenceCheck struct {
	referrerNamespace string
	pending           []string
	unsupported       []string
}

// NewReferenceCheck returns a ReferenceCheck for a custom resource in the
// supplied namespace.
func NewReferenceCheck(referrerNamespace string) *ReferenceCheck {
	return &ReferenceCheck{referrerNamespace: referrerNamespace}
}

// Pending records the reference at path when it is a ref that has no
// resolved ID yet.
func (c *ReferenceCheck) Pending(
	path string,
	refOrID cloudcommon.ResourceReferenceOrID,
	resolved string,
) {
	if refOrID.Ref == nil || resolved != "" {
		return
	}
	key := refOrID.Ref.Key(c.referrerNamespace)
	c.pending = append(c.pending, fmt.Sprintf("%s (%s)", path, key))
}

// Unsupported records the reference at path when it is a ref to a kind that
// can't be referenced yet.
//
// TODO(tomba): remove when all resource types have been implemented.
func (c *ReferenceCheck) Unsupported(path string, refOrID cloudcommon.ResourceReferenceOrID) {
	if refOrID.Ref != nil {
		c.unsupported = append(c.unsupported, path)
	}
}

// Err returns a terminal error for unsupported references, a requeue error
// for pending ones, or nil.
func (c *ReferenceCheck) Err() error {
	if len(c.unsupported) > 0 {
		return ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: references to these custom resources aren't supported yet, "+
					"set id instead: %s",
				ctrlerrors.ErrResourceSync, strings.Join(c.unsupported, ", "),
			),
		)
	}
	if len(c.pending) > 0 {
		return requeue.NeededAfter(
			fmt.Errorf(
				"waiting for referenced custom resources to be ready in Temporal Cloud: %s",
				strings.Join(c.pending, ", "),
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}
	return nil
}

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

// CloudCallError wraps an error returned from a Temporal Cloud call in
// [ctrlerrors.ErrResourceSync] and classifies it for the controller.
func CloudCallError(action string, err error) error {
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
