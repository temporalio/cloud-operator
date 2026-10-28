package namespace

import (
	"context"
	"errors"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/annotation"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
)

// ResourceDelete deletes the Temporal Cloud Namespace for the supplied
// [types.Resource], unless its deletion policy is retain. It returns nil once
// the Namespace is gone, and an error while the delete is in progress or has
// failed.
//
// It works on a copy of `original` and returns that copy, with any status
// changes (State, AsyncOperationID, conditions), as the latest state.
// `original` is never mutated.
//
// TODO(Mechack): status.id and status.resourceVersion are only set once the
// create path is merged. Until then they're empty, so ResourceDelete treats
// every Namespace as never created. Delete this TODO when it lands.
func (r *Reconciler) ResourceDelete(
	ctx context.Context,
	original types.Resource,
) (types.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.delete")
	defer func() {
		exit(err)
	}()

	latest := original.DeepCopy()
	cr := latest.CR().(*tcloudv1alpha1.Namespace)
	if cr.Status.ID == "" {
		// The user deleted the CR before the operator created a Namespace in
		// Temporal Cloud, so there's nothing to delete.
		return latest, nil
	}

	policy, err := deletionPolicy(latest)
	if err != nil {
		return latest, err
	}
	if policy == tcloudcommon.DeletionPolicyRetain {
		// TODO(Mechack): log that the Namespace is retained because of the
		// retain deletion policy, once ctrlreqlog's log.Info no longer panics
		// (ToContext never stores the request logger).
		return latest, nil
	}

	tcc, err := r.CloudClientFor(ctx, latest)
	if err != nil {
		err = fmt.Errorf("failed getting tcloud client: %w", err)
		return latest, err
	}

	// We may have already called DeleteNamespace for this resource. If so,
	// check whether the delete operation has finished.
	switch {
	case cr.Status.State == tcloudcommon.ResourceStateDeleted:
		return latest, nil
	case cr.Status.State == tcloudcommon.ResourceStateDeleteFailed:
		// A previous delete failed. Don't send another one automatically.
		err = ctrlerrors.NewTerminalError(fmt.Errorf(
			"deleting Temporal Cloud namespace %s previously failed (async operation %s); not retrying",
			cr.Status.ID, cr.Status.AsyncOperationID,
		))
		return latest, err
	case cr.Status.State == tcloudcommon.ResourceStateDeleting &&
		cr.Status.AsyncOperationID != "":
		err = checkDelete(ctx, tcc, latest, cr)
		return latest, err
	case cr.Status.State == tcloudcommon.ResourceStateActivating,
		cr.Status.State == tcloudcommon.ResourceStateUpdating:
		// A create or update on this custom resource is still in flight, and Temporal Cloud rejects a
		// delete until the Namespace is Active. Check again later.
		err = requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud namespace %s to finish %s before deleting",
				cr.Status.ID, cr.Status.State,
			),
			requeue.DefaultRequeueAfterDuration,
		)
		return latest, err
	}

	resp, err := tcc.DeleteNamespace(ctx, &tcloudservicev1.DeleteNamespaceRequest{
		Namespace:       cr.Status.ID,
		ResourceVersion: cr.Status.ResourceVersion,
	})
	if status.Code(err) == codes.NotFound {
		// Already gone.
		err = nil
		return latest, nil
	}
	if err != nil {
		// The gRPC error includes Temporal Cloud's message, for example that
		// deletion protection is enabled.
		err = fmt.Errorf("failed deleting tcloud namespace %s: %w", cr.Status.ID, err)
		condition.SetStalled(
			latest, metav1.ConditionTrue, err.Error(), tcloudcommon.ReasonResourceDeleteFailed,
		)
		return latest, err
	}

	cr.Status.State = tcloudcommon.ResourceStateDeleting
	cr.Status.AsyncOperationID = resp.GetAsyncOperation().GetId()
	err = requeue.NeededAfter(
		fmt.Errorf(
			"deleting tcloud namespace %s (async operation %s)",
			cr.Status.ID, cr.Status.AsyncOperationID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
	return latest, err
}

// checkDelete checks the async operation for a delete already sent to
// Temporal Cloud. It returns nil once the operation is fulfilled, a requeue
// error while it is running, and an error with the Stalled condition set on
// latest if it failed.
func checkDelete(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	latest types.Resource,
	cr *tcloudv1alpha1.Namespace,
) error {
	resp, err := tcc.GetAsyncOperation(ctx, &tcloudservicev1.GetAsyncOperationRequest{
		AsyncOperationId: cr.Status.AsyncOperationID,
	})
	if err != nil {
		return fmt.Errorf(
			"failed getting tcloud async operation %s: %w",
			cr.Status.AsyncOperationID, err,
		)
	}
	op := resp.GetAsyncOperation()
	switch op.GetState() {
	case operationv1.AsyncOperation_STATE_FULFILLED:
		return nil
	case operationv1.AsyncOperation_STATE_FAILED,
		operationv1.AsyncOperation_STATE_REJECTED,
		operationv1.AsyncOperation_STATE_CANCELLED:
		msg := fmt.Sprintf(
			"deleting tcloud namespace %s failed: async operation %s is %s: %s",
			cr.Status.ID, op.GetId(), op.GetState(), op.GetFailureReason(),
		)
		condition.SetStalled(
			latest, metav1.ConditionTrue, msg, tcloudcommon.ReasonResourceDeleteFailed,
		)
		cr.Status.State = tcloudcommon.ResourceStateDeleteFailed
		return ctrlerrors.NewTerminalError(errors.New(msg))
	default:
		// Pending, in progress, or a state this SDK doesn't know.
		return requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud async operation %s (%s)",
				op.GetId(), op.GetState(),
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}
}

// deletionPolicy returns the subject's deletion policy from its
// cloud.temporal.io/deletion-policy annotation. An unset annotation means
// delete. An invalid value is an error and never falls back to delete.
//
// TODO(Mechack): once adoption is supported, the default for an adopted
// Namespace becomes retain. An explicit delete is honored either way.
func deletionPolicy(subject types.Resource) (tcloudcommon.DeletionPolicy, error) {
	v, ok := annotation.Value(subject, tcloudcommon.AnnotationKeyDeletionPolicy)
	if !ok {
		return tcloudcommon.DeletionPolicyDelete, nil
	}
	p := tcloudcommon.DeletionPolicy(v)
	return p, p.Validate()
}
