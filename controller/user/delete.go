package user

import (
	"context"
	"errors"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/reconciler/deletion"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceDelete deletes the Temporal Cloud user for the supplied
// User, unless its deletion policy is retain.
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
		// The CR was deleted before the operator created a user in Temporal
		// Cloud, so there's nothing to delete.
		//
		// TODO: this misses a user whose create succeeded but whose ID was
		// never saved, for example because the operator restarted in between.
		// Emails are unique in Temporal Cloud, so that user could be looked up
		// by email here.
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

	cr := latest.CR().(*tcloudv1alpha1.User)
	var done bool
	if done, err = deletion.CheckPrevious(
		ctx, tcc, latest, &cr.Status.Resource, resourceKind,
	); done {
		return latest, err
	}

	err = deleteUser(ctx, tcc, latest)
	return latest, err
}

// deleteUser deletes the user in Temporal Cloud and saves the delete's async
// operation in status.
func deleteUser(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	latest types.Resource,
) error {
	cr := latest.CR().(*tcloudv1alpha1.User)
	resourceID := latest.ResourceID()

	// Read the user for its current state and resource version.
	getResp, err := tcc.GetUser(ctx, &tcloudservicev1.GetUserRequest{
		UserId: resourceID,
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed getting tcloud user %s: %w", resourceID, err)
	}

	var state cloudcommon.ResourceState
	state.FromProto(getResp.GetUser().GetState())
	if err = deletion.CheckInFlight(&cr.Status.Resource, state, resourceKind, resourceID); err != nil {
		return err
	}

	resp, err := tcc.DeleteUser(ctx, &tcloudservicev1.DeleteUserRequest{
		UserId:          resourceID,
		ResourceVersion: getResp.GetUser().GetResourceVersion(),
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if status.Code(err) == codes.InvalidArgument {
		// Temporal Cloud refuses to delete an account Owner, a SCIM user or the
		// ONLY account Admin, and retrying doesn't change that. Annotation edits
		// don't trigger a reconcile, so requeue to pick up a retain policy.
		msg := fmt.Sprintf(
			"failed deleting tcloud user %s: %s. Set the %s annotation to %q "+
				"to keep the user in Temporal Cloud and remove this CR",
			resourceID, status.Convert(err).Message(),
			cloudcommon.AnnotationKeyDeletionPolicy, cloudcommon.DeletionPolicyRetain,
		)
		condition.SetStalled(
			latest, metav1.ConditionTrue, msg, cloudcommon.ReasonResourceDeleteFailed,
		)
		return requeue.NeededAfter(errors.New(msg), requeue.DefaultRequeueAfterDuration)
	}
	if err != nil {
		err = fmt.Errorf("failed deleting tcloud user %s: %w", resourceID, err)
		condition.SetStalled(
			latest, metav1.ConditionTrue, err.Error(), cloudcommon.ReasonResourceDeleteFailed,
		)
		return err
	}

	cr.Status.Resource.State = cloudcommon.ResourceStateDeleting
	cr.Status.Resource.AsyncOperationID = resp.GetAsyncOperation().GetId()
	return requeue.NeededAfter(
		fmt.Errorf(
			"deleting tcloud user %s (async operation %s)",
			resourceID, cr.Status.Resource.AsyncOperationID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
}
