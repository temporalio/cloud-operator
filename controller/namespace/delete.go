package namespace

import (
	"context"
	"errors"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/types"
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
func (r *Reconciler) ResourceDelete(
	ctx context.Context,
	original kttypes.Resource,
) (kttypes.Resource, error) {
	return r.deleteNamespace(
		ctx,
		original.(types.Resource),
	)
}

// deleteNamespace attempts to delete the supplied resource.
func (r *Reconciler) deleteNamespace(
	ctx context.Context,
	original types.Resource,
) (kttypes.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.delete")
	defer func() {
		exit(err)
	}()

	// Make a copy of the original resource. We may modify and return this
	// copy, and want to make sure we never mutate original.
	updated := original.DeepCopy()
	resourceID := original.ResourceID()

	if !original.PreviouslyReconciled() {
		// The user deleted the CR before the operator created a Namespace in
		// Temporal Cloud, so there's nothing to delete.
		return updated, nil
	}

	dp := original.DeletionPolicy()

	err = dp.Validate()
	if err != nil {
		// Never fall back to delete on an invalid value. Annotation edits
		// don't trigger a reconcile, so requeue to pick up a corrected value.
		msg := fmt.Sprintf(
			"%s. Set the %s annotation to %q or %q",
			err,
			tcloudcommon.AnnotationKeyDeletionPolicy,
			tcloudcommon.DeletionPolicyRetain,
			tcloudcommon.DeletionPolicyDelete,
		)
		condition.SetStalled(
			updated,
			metav1.ConditionTrue,
			msg,
			tcloudcommon.ReasonResourceDeleteFailed,
		)
		err = requeue.NeededAfter(
			errors.New(msg),
			requeue.DefaultRequeueAfterDuration,
		)
		return updated, err
	}

	if dp == tcloudcommon.DeletionPolicyRetain {
		log.Info(
			"retaining Temporal Cloud Namespace because of " +
				"a retain deletion policy",
		)
		return updated, nil
	}

	tcc, err := r.CloudClientFor(ctx, original)
	if err != nil {
		err = fmt.Errorf("failed getting tcloud client: %w", err)
		return updated, err
	}

	resStatus := original.ResourceStatus()
	resState := resStatus.State
	asyncOpID := resStatus.AsyncOperationID

	updatedNS := updated.CR().(*tcloudv1alpha1.Namespace)
	originalNS := original.CR().(*tcloudv1alpha1.Namespace)

	// We may have already called DeleteNamespace for this resource. If so,
	// check whether the delete operation has finished.
	switch {
	case resState == tcloudcommon.ResourceStateDeleted:
		return updated, nil
	case resState == tcloudcommon.ResourceStateDeleteFailed:
		// A previous delete failed. Don't send another one automatically.
		err = ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"deleting Temporal Cloud namespace %s previously failed "+
					"(async operation %s); not retrying",
				resourceID, asyncOpID,
			),
		)
		return updated, err
	case resState == tcloudcommon.ResourceStateDeleting &&
		asyncOpID != "":
		op, err := getAsyncOperation(ctx, tcc, asyncOpID)
		if err != nil {
			return updated, err
		}
		opState := op.GetState()
		switch opState {
		case operationv1.AsyncOperation_STATE_FULFILLED:
			return updated, nil
		case operationv1.AsyncOperation_STATE_FAILED,
			operationv1.AsyncOperation_STATE_REJECTED,
			operationv1.AsyncOperation_STATE_CANCELLED:
			msg := fmt.Sprintf(
				"deleting tcloud namespace %s failed: "+
					"async operation %s is %s: %s",
				resourceID,
				asyncOpID,
				opState,
				op.GetFailureReason(),
			)
			condition.SetStalled(
				updated, metav1.ConditionTrue, msg, tcloudcommon.ReasonResourceDeleteFailed,
			)
			updatedNS.Status.Resource.State = tcloudcommon.ResourceStateDeleteFailed
			return updated, ctrlerrors.NewTerminalError(errors.New(msg))
		default:
			// Pending, in progress, or a state this SDK doesn't know.
			return updated, requeue.NeededAfter(
				fmt.Errorf(
					"waiting for tcloud async operation %s (%s)",
					op.GetId(), op.GetState(),
				),
				requeue.DefaultRequeueAfterDuration,
			)
		}
	case resState == tcloudcommon.ResourceStateActivating,
		resState == tcloudcommon.ResourceStateUpdating:
		// A create or update on this custom resource is still in flight, and Temporal Cloud rejects a
		// delete until the Namespace is Active. Check again later.
		err = requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud namespace %s to finish %s before deleting",
				resourceID, resState,
			),
			requeue.DefaultRequeueAfterDuration,
		)
		return updated, err
	}

	// Temporal Cloud rejects deleting a protected Namespace, so no need to send the
	// delete request. Changing the spec triggers a reconcile, but switching the
	// annotation to retain doesn't, so requeue to pick that up.
	if originalNS.Spec.Lifecycle != nil && originalNS.Spec.Lifecycle.EnableDeleteProtection {
		msg := fmt.Sprintf(
			"tcloud namespace %s has deletion protection enabled; set "+
				"spec.lifecycle.enableDeleteProtection to false to delete it, "+
				"or set the %s annotation to %q to keep it",
			resourceID, tcloudcommon.AnnotationKeyDeletionPolicy,
			tcloudcommon.DeletionPolicyRetain,
		)
		condition.SetStalled(
			updated, metav1.ConditionTrue, msg, tcloudcommon.ReasonResourceDeleteFailed,
		)
		err = requeue.NeededAfter(errors.New(msg), requeue.DefaultRequeueAfterDuration)
		return updated, err
	}

	// Read the namespace so deletion protection in Temporal Cloud can be made
	// to match the CR, and so the delete uses the current resource version.
	getResp, err := tcc.GetNamespace(ctx, &tcloudservicev1.GetNamespaceRequest{
		Namespace: resourceID,
	})
	if status.Code(err) == codes.NotFound {
		// Already gone.
		err = nil
		return updated, nil
	}
	if err != nil {
		err = fmt.Errorf("failed getting tcloud namespace %s: %w", resourceID, err)
		return updated, err
	}
	tns := getResp.GetNamespace()

	if tns.GetSpec().GetLifecycle().GetEnableDeleteProtection() {
		// The CR's spec says protection is off, so make
		// Temporal Cloud match it before deleting.
		err = disableDeleteProtection(ctx, tcc, tns)
		return updated, err
	}

	resp, err := tcc.DeleteNamespace(ctx, &tcloudservicev1.DeleteNamespaceRequest{
		Namespace:       resourceID,
		ResourceVersion: tns.GetResourceVersion(),
	})
	if status.Code(err) == codes.NotFound {
		// Already gone.
		err = nil
		return updated, nil
	}
	if err != nil {
		// The gRPC error includes Temporal Cloud's message, for example that
		// deletion protection is enabled.
		err = fmt.Errorf(
			"failed deleting tcloud namespace %s: %w",
			resourceID, err,
		)
		condition.SetStalled(
			updated, metav1.ConditionTrue, err.Error(), tcloudcommon.ReasonResourceDeleteFailed,
		)
		return updated, err
	}

	updatedNS.Status.Resource.State = tcloudcommon.ResourceStateDeleting
	updatedNS.Status.Resource.AsyncOperationID = resp.GetAsyncOperation().GetId()
	err = requeue.NeededAfter(
		fmt.Errorf(
			"deleting tcloud namespace %s (async operation %s)",
			resourceID, asyncOpID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
	return updated, err
}

// disableDeleteProtection turns off deletion protection on the
// Temporal Cloud Namespace. It sends the Namespace's current spec from
// Temporal Cloud with only that setting changed, so other pending changes to
// the CR's spec aren't pushed right before a delete. On success it returns a
// requeue error, and the next reconcile re-reads the Namespace rather than
// tracking the update's async operation.
func disableDeleteProtection(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	tns *tcloudnamespacev1.Namespace,
) error {
	spec := proto.Clone(tns.GetSpec()).(*tcloudnamespacev1.NamespaceSpec)
	spec.GetLifecycle().EnableDeleteProtection = false
	_, err := tcc.UpdateNamespace(ctx, &tcloudservicev1.UpdateNamespaceRequest{
		Namespace:       tns.GetNamespace(),
		Spec:            spec,
		ResourceVersion: tns.GetResourceVersion(),
	})
	if err != nil {
		return cloudCallError("turning off tcloud namespace deletion protection", err)
	}
	return requeue.NeededAfter(
		fmt.Errorf(
			"turned off deletion protection on tcloud namespace %s, waiting for update to complete",
			tns.GetNamespace(),
		),
		requeue.DefaultRequeueAfterDuration,
	)
}

// getAsyncOperation returns information about the async operationr identified
// by the supplied opID.
func getAsyncOperation(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	opID string,
) (*operationv1.AsyncOperation, error) {
	req := &tcloudservicev1.GetAsyncOperationRequest{
		AsyncOperationId: opID,
	}
	resp, err := tcc.GetAsyncOperation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf(
			"failed getting tcloud async operation %s: %w",
			opID, err,
		)
	}
	op := resp.GetAsyncOperation()
	return op, nil
}
