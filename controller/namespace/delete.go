package namespace

import (
	"context"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/reconciler/deletion"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceDelete deletes the Temporal Cloud Namespace for the supplied
// [types.Resource], unless its deletion policy is retain. It returns nil once
// the Namespace is gone, and an error while the delete is in progress or has
// failed.
//
// It works on a copy of `original` and returns that copy, with any status
// changes (State, AsyncOperationID, conditions), as the latest state.
// `original` is NEVER mutated.
func (r *Reconciler) ResourceDelete(
	ctx context.Context,
	original kttypes.Resource,
) (kttypes.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.delete")
	defer func() {
		exit(err)
	}()

	latest := original.DeepCopy().(types.Resource)

	if !latest.PreviouslyReconciled() {
		// The user deleted the CR before the operator created a Namespace in
		// Temporal Cloud, so there's nothing to delete.
		return latest, nil
	}

	var retain bool
	if retain, err = deletion.CheckPolicy(ctx, latest, resourceKind); err != nil || retain {
		return latest, err
	}

	tcc, err := r.CloudClientFor(ctx, latest)
	if err != nil {
		err = fmt.Errorf("failed getting tcloud client: %w", err)
		return latest, err
	}

	cr := latest.CR().(*tcloudv1alpha1.Namespace)
	var done bool
	if done, err = deletion.CheckPrevious(
		ctx, tcc, latest, &cr.Status.Resource, resourceKind,
	); done {
		return latest, err
	}

	if err = deletion.CheckProtection(latest, cr.Spec.Lifecycle, resourceKind); err != nil {
		return latest, err
	}

	err = deleteNamespace(ctx, tcc, latest)
	return latest, err
}

// deleteNamespace deletes the Namespace in Temporal Cloud and saves the
// delete's async operation in status. If deletion protection is still enabled
// in Temporal Cloud, it turns that off first and requeues instead.
func deleteNamespace(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	latest types.Resource,
) error {
	cr := latest.CR().(*tcloudv1alpha1.Namespace)
	resourceID := latest.ResourceID()

	// Read the Namespace for its current state, deletion protection and
	// resource version.
	getResp, err := tcc.GetNamespace(ctx, &tcloudservicev1.GetNamespaceRequest{
		Namespace: resourceID,
	})
	if status.Code(err) == codes.NotFound {
		// Already gone.
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed getting tcloud namespace %s: %w", resourceID, err)
	}
	tns := getResp.GetNamespace()

	var state cloudcommon.ResourceState
	state.FromProto(tns.GetState())
	if err = deletion.CheckInFlight(&cr.Status.Resource, state, resourceKind, resourceID); err != nil {
		return err
	}

	if tns.GetSpec().GetLifecycle().GetEnableDeleteProtection() {
		// deletion.CheckProtection already confirmed the CR's spec has
		// protection off, so make Temporal Cloud match it before deleting.
		return disableDeleteProtection(ctx, tcc, tns)
	}

	resp, err := tcc.DeleteNamespace(ctx, &tcloudservicev1.DeleteNamespaceRequest{
		Namespace:       resourceID,
		ResourceVersion: tns.GetResourceVersion(),
	})
	if status.Code(err) == codes.NotFound {
		// Already gone.
		return nil
	}
	if err != nil {
		// The gRPC error includes Temporal Cloud's message, for example that
		// deletion protection is enabled.
		err = fmt.Errorf("failed deleting tcloud namespace %s: %w", resourceID, err)
		condition.SetStalled(
			latest, metav1.ConditionTrue, err.Error(), cloudcommon.ReasonResourceDeleteFailed,
		)
		return err
	}

	cr.Status.Resource.State = cloudcommon.ResourceStateDeleting
	cr.Status.Resource.AsyncOperationID = resp.GetAsyncOperation().GetId()
	return requeue.NeededAfter(
		fmt.Errorf(
			"deleting tcloud namespace %s (async operation %s)",
			resourceID, cr.Status.Resource.AsyncOperationID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
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
		return cloud.CallError("turning off tcloud namespace deletion protection", err)
	}
	return requeue.NeededAfter(
		fmt.Errorf(
			"turned off deletion protection on tcloud namespace %s, waiting for update to complete",
			tns.GetNamespace(),
		),
		requeue.DefaultRequeueAfterDuration,
	)
}
