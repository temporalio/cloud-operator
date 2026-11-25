package usergroup

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
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceDelete deletes the Temporal Cloud user group for the supplied
// [types.Resource], unless its deletion policy is retain.
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
		// The CR was deleted before the operator created a user group in
		// Temporal Cloud, so there's nothing to delete.
		//
		// TODO: this misses a user group whose create succeeded but whose ID
		// was never saved, for example because the operator restarted in
		// between. Once crash recovery exists, look that user group up here
		// too, so it isn't left behind in Temporal Cloud.
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

	cr := latest.CR().(*tcloudv1alpha1.UserGroup)
	var done bool
	if done, err = deletion.CheckPrevious(
		ctx, tcc, latest, &cr.Status.Resource, resourceKind,
	); done {
		return latest, err
	}

	// GetUserGroup returns NotFound until a create finishes. Deleting before
	// then would find nothing and remove the CR, and the group created
	// afterwards would be left behind in Temporal Cloud.
	if opID := cr.Status.Resource.AsyncOperationID; cr.Status.Resource.State == "" && opID != "" {
		err = checkCreate(ctx, tcc, latest.ResourceID(), opID)
		if errors.Is(err, cloud.ErrAsyncOperationFailed) {
			// The create failed, so there's no user group to delete.
			err = nil
			return latest, err
		}
		if err != nil {
			return latest, err
		}
	}

	err = deleteUserGroup(ctx, tcc, latest)
	return latest, err
}

// deleteUserGroup deletes the user group in Temporal Cloud and saves the
// delete's async operation in status.
func deleteUserGroup(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	latest types.Resource,
) error {
	cr := latest.CR().(*tcloudv1alpha1.UserGroup)
	resourceID := latest.ResourceID()

	// Read the user group for its current state and resource version.
	getResp, err := tcc.GetUserGroup(ctx, &tcloudservicev1.GetUserGroupRequest{
		GroupId: resourceID,
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed getting tcloud user group %s: %w", resourceID, err)
	}

	var state cloudcommon.ResourceState
	state.FromProto(getResp.GetGroup().GetState())
	if err = deletion.CheckInFlight(&cr.Status.Resource, state, resourceKind, resourceID); err != nil {
		return err
	}

	resp, err := tcc.DeleteUserGroup(ctx, &tcloudservicev1.DeleteUserGroupRequest{
		GroupId:         resourceID,
		ResourceVersion: getResp.GetGroup().GetResourceVersion(),
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		err = fmt.Errorf("failed deleting tcloud user group %s: %w", resourceID, err)
		condition.SetStalled(
			latest, metav1.ConditionTrue, err.Error(), cloudcommon.ReasonResourceDeleteFailed,
		)
		return err
	}

	cr.Status.Resource.State = cloudcommon.ResourceStateDeleting
	cr.Status.Resource.AsyncOperationID = resp.GetAsyncOperation().GetId()
	return requeue.NeededAfter(
		fmt.Errorf(
			"deleting tcloud user group %s (async operation %s)",
			resourceID, cr.Status.Resource.AsyncOperationID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
}
