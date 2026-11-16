package project

import (
	"context"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudprojectv1 "go.temporal.io/cloud-sdk/api/project/v1"
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

// ResourceDelete deletes the Temporal Cloud project for the supplied
// [types.Resource], unless its deletion policy is retain. It returns nil once
// the project is gone, and an error while the delete is in progress or has
// failed.
//
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
		// The CR was deleted before the operator created a project in
		// Temporal Cloud, so there's nothing to delete.
		//
		// TODO: this misses a project whose create succeeded but whose ID was
		// never saved (see ResourceLatest). Once crash recovery exists, look
		// that project up here too, so it isn't left behind in Temporal Cloud.
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

	cr := latest.CR().(*tcloudv1alpha1.Project)
	var done bool
	if done, err = deletion.CheckPrevious(
		ctx, tcc, latest, &cr.Status.Resource, resourceKind,
	); done {
		return latest, err
	}

	if err = deletion.CheckProtection(latest, cr.Spec.Lifecycle, resourceKind); err != nil {
		return latest, err
	}

	err = deleteProject(ctx, tcc, latest)
	return latest, err
}

// deleteProject deletes the project in Temporal Cloud and saves the delete's
// async operation in status. If deletion protection is still enabled in
// Temporal Cloud, it turns that off first and requeues instead.
func deleteProject(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	latest types.Resource,
) error {
	cr := latest.CR().(*tcloudv1alpha1.Project)
	resourceID := latest.ResourceID()

	// Read the project for its current state, deletion protection and
	// resource version.
	getResp, err := tcc.GetProject(ctx, &tcloudservicev1.GetProjectRequest{
		ProjectId: resourceID,
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed getting tcloud project %s: %w", resourceID, err)
	}
	tp := getResp.GetProject()

	var state cloudcommon.ResourceState
	state.FromProto(tp.GetState())
	if err = deletion.CheckInFlight(&cr.Status.Resource, state, resourceKind, resourceID); err != nil {
		return err
	}

	if tp.GetSpec().GetLifecycle().GetEnableDeleteProtection() {
		// deletion.CheckProtection already confirmed the CR's spec has protection
		// off, so make Temporal Cloud match it before deleting.
		return disableDeleteProtection(ctx, tcc, tp)
	}

	resp, err := tcc.DeleteProject(ctx, &tcloudservicev1.DeleteProjectRequest{
		ProjectId:       resourceID,
		ResourceVersion: tp.GetResourceVersion(),
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		// The gRPC error includes Temporal Cloud's message, for example that
		// the project still contains namespaces.
		err = fmt.Errorf("failed deleting tcloud project %s: %w", resourceID, err)
		condition.SetStalled(
			latest, metav1.ConditionTrue, err.Error(), cloudcommon.ReasonResourceDeleteFailed,
		)
		return err
	}

	cr.Status.Resource.State = cloudcommon.ResourceStateDeleting
	cr.Status.Resource.AsyncOperationID = resp.GetAsyncOperation().GetId()
	return requeue.NeededAfter(
		fmt.Errorf(
			"deleting tcloud project %s (async operation %s)",
			resourceID, cr.Status.Resource.AsyncOperationID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
}

// disableDeleteProtection turns off deletion protection on the Temporal Cloud
// project. It sends the project's current spec from Temporal Cloud with only
// that setting changed, so other pending changes to the CR's spec aren't
// pushed right before a delete. On success it returns a requeue error, and the
// next reconcile re-reads the project rather than tracking the update's async
// operation.
func disableDeleteProtection(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	tp *tcloudprojectv1.Project,
) error {
	spec := proto.Clone(tp.GetSpec()).(*tcloudprojectv1.ProjectSpec)
	spec.GetLifecycle().EnableDeleteProtection = false
	_, err := tcc.UpdateProject(ctx, &tcloudservicev1.UpdateProjectRequest{
		ProjectId:       tp.GetId(),
		Spec:            spec,
		ResourceVersion: tp.GetResourceVersion(),
	})
	if err != nil {
		return cloud.CallError("turning off tcloud project deletion protection", err)
	}
	return requeue.NeededAfter(
		fmt.Errorf(
			"turned off deletion protection on tcloud project %s, waiting for update to complete",
			tp.GetId(),
		),
		requeue.DefaultRequeueAfterDuration,
	)
}
