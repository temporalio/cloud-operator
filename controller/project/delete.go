package project

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
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
	if retain, err = checkDeletionPolicy(ctx, latest); err != nil || retain {
		return latest, err
	}

	tcc, err := r.CloudClientFor(ctx, latest)
	if err != nil {
		err = fmt.Errorf("failed getting tcloud client: %w", err)
		return latest, err
	}

	var handled bool
	if handled, err = checkPreviousDelete(ctx, tcc, latest); handled {
		return latest, err
	}

	if err = checkDeleteProtection(latest); err != nil {
		return latest, err
	}

	err = deleteProject(ctx, tcc, latest)
	return latest, err
}

// checkDeletionPolicy reports whether the deletion policy keeps the project in
// Temporal Cloud. An invalid policy marks the CR as stalled and returns a
// requeue error.
func checkDeletionPolicy(ctx context.Context, latest types.Resource) (bool, error) {
	policy := latest.DeletionPolicy()
	if err := policy.Validate(); err != nil {
		// Never fall back to delete on an invalid value. Annotation edits
		// don't trigger a reconcile, so requeue to pick up a corrected value.
		msg := fmt.Sprintf(
			"%s. Set the %s annotation to %q or %q",
			err, cloudcommon.AnnotationKeyDeletionPolicy,
			cloudcommon.DeletionPolicyRetain, cloudcommon.DeletionPolicyDelete,
		)
		condition.SetStalled(
			latest, metav1.ConditionTrue, msg, cloudcommon.ReasonResourceDeleteFailed,
		)
		return false, requeue.NeededAfter(errors.New(msg), requeue.DefaultRequeueAfterDuration)
	}
	if policy == cloudcommon.DeletionPolicyRetain {
		ctrlreqlog.FromContext(ctx).Info(
			"retaining Temporal Cloud project because of the retain deletion policy",
			slog.String("project_id", latest.ResourceID()),
			slog.String("deletion_policy", string(policy)),
		)
		return true, nil
	}
	return false, nil
}

// checkPreviousDelete reports whether the project's state means no new delete
// should be sent, and returns the result to use instead.
func checkPreviousDelete(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	latest types.Resource,
) (bool, error) {
	cr := latest.CR().(*tcloudv1alpha1.Project)
	resourceID := latest.ResourceID()
	resourceState := cr.Status.Resource.State
	asyncOpID := cr.Status.Resource.AsyncOperationID

	switch {
	case resourceState == cloudcommon.ResourceStateDeleted:
		return true, nil
	case resourceState == cloudcommon.ResourceStateDeleteFailed:
		// A previous delete failed. Don't send another one automatically.
		return true, ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"deleting Temporal Cloud project %s previously failed "+
					"(async operation %s); not retrying",
				resourceID, asyncOpID,
			),
		)
	case resourceState == cloudcommon.ResourceStateDeleting && asyncOpID != "":
		err := cloud.CheckAsyncOperation(ctx, tcc, asyncOpID)
		if errors.Is(err, cloud.ErrAsyncOperationFailed) {
			err = fmt.Errorf("deleting tcloud project %s failed: %w", resourceID, err)
			condition.SetStalled(
				latest, metav1.ConditionTrue, err.Error(), cloudcommon.ReasonResourceDeleteFailed,
			)
			cr.Status.Resource.State = cloudcommon.ResourceStateDeleteFailed
		}
		return true, err
	}
	return false, nil
}

// checkDeleteProtection returns an error when the CR's spec enables deletion
// protection, because Temporal Cloud rejects deleting a protected project.
// Changing the spec triggers a reconcile, but switching the annotation to
// retain doesn't, so the error requeues to pick that up.
func checkDeleteProtection(latest types.Resource) error {
	cr := latest.CR().(*tcloudv1alpha1.Project)
	if cr.Spec.Lifecycle == nil || !cr.Spec.Lifecycle.EnableDeleteProtection {
		return nil
	}
	msg := fmt.Sprintf(
		"tcloud project %s has deletion protection enabled; set "+
			"spec.lifecycle.enableDeleteProtection to false to delete it, "+
			"or set the %s annotation to %q to keep it",
		latest.ResourceID(), cloudcommon.AnnotationKeyDeletionPolicy,
		cloudcommon.DeletionPolicyRetain,
	)
	condition.SetStalled(
		latest, metav1.ConditionTrue, msg, cloudcommon.ReasonResourceDeleteFailed,
	)
	return requeue.NeededAfter(errors.New(msg), requeue.DefaultRequeueAfterDuration)
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
	if state == cloudcommon.ResourceStateActivating || state == cloudcommon.ResourceStateUpdating {
		// A create or update is still in flight. Wait till the project is
		// Active again for the delete request to be accepted in Temporal Cloud.
		cr.Status.Resource.State = state
		return requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud project %s to finish %s before deleting",
				resourceID, state,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	if tp.GetSpec().GetLifecycle().GetEnableDeleteProtection() {
		// checkDeleteProtection already confirmed the CR's spec has protection
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
