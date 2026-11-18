package apikey

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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceDelete deletes the Temporal Cloud API key for the supplied
// [types.Resource], unless its deletion policy is retain. It returns nil once
// the API key is gone, and an error while the delete is in progress or has
// failed.
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
		// The CR was deleted before the operator created an API key in
		// Temporal Cloud, so there's nothing to delete.
		return latest, nil
	}

	var retain bool
	if retain, err = checkDeletionPolicy(ctx, latest); err != nil {
		return latest, err
	}
	if retain {
		err = r.releaseTokenSecret(ctx, latest.CR().(*tcloudv1alpha1.APIKey))
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

	err = deleteAPIKey(ctx, tcc, latest)
	return latest, err
}

// checkDeletionPolicy reports whether the deletion policy keeps the API key in
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
			"retaining Temporal Cloud api key because of the retain deletion policy",
			slog.String("api_key_id", latest.ResourceID()),
			slog.String("deletion_policy", string(policy)),
		)
		return true, nil
	}
	return false, nil
}

// checkPreviousDelete reports whether the API key's state means no new delete
// should be sent, and returns the result to use instead.
func checkPreviousDelete(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	latest types.Resource,
) (bool, error) {
	cr := latest.CR().(*tcloudv1alpha1.APIKey)
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
				"deleting Temporal Cloud api key %s previously failed "+
					"(async operation %s); not retrying",
				resourceID, asyncOpID,
			),
		)
	case resourceState == cloudcommon.ResourceStateDeleting && asyncOpID != "":
		err := cloud.CheckAsyncOperation(ctx, tcc, asyncOpID)
		if errors.Is(err, cloud.ErrAsyncOperationFailed) {
			err = fmt.Errorf("deleting tcloud api key %s failed: %w", resourceID, err)
			condition.SetStalled(
				latest, metav1.ConditionTrue, err.Error(), cloudcommon.ReasonResourceDeleteFailed,
			)
			cr.Status.Resource.State = cloudcommon.ResourceStateDeleteFailed
		}
		return true, err
	}
	return false, nil
}

// deleteAPIKey deletes the API key in Temporal Cloud and saves the delete's
// async operation in status.
func deleteAPIKey(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	latest types.Resource,
) error {
	cr := latest.CR().(*tcloudv1alpha1.APIKey)
	resourceID := latest.ResourceID()

	// Read the API key for its current state and resource version.
	getResp, err := tcc.GetApiKey(ctx, &tcloudservicev1.GetApiKeyRequest{
		KeyId: resourceID,
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed getting tcloud api key %s: %w", resourceID, err)
	}

	var state cloudcommon.ResourceState
	state.FromProto(getResp.GetApiKey().GetState())
	if state == cloudcommon.ResourceStateActivating || state == cloudcommon.ResourceStateUpdating {
		// Temporal Cloud only accepts the delete once a create or update has
		// finished.
		cr.Status.Resource.State = state
		return requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud api key %s to finish %s before deleting",
				resourceID, state,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	resp, err := tcc.DeleteApiKey(ctx, &tcloudservicev1.DeleteApiKeyRequest{
		KeyId:           resourceID,
		ResourceVersion: getResp.GetApiKey().GetResourceVersion(),
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		err = fmt.Errorf("failed deleting tcloud api key %s: %w", resourceID, err)
		condition.SetStalled(
			latest, metav1.ConditionTrue, err.Error(), cloudcommon.ReasonResourceDeleteFailed,
		)
		return err
	}

	cr.Status.Resource.State = cloudcommon.ResourceStateDeleting
	cr.Status.Resource.AsyncOperationID = resp.GetAsyncOperation().GetId()
	return requeue.NeededAfter(
		fmt.Errorf(
			"deleting tcloud api key %s (async operation %s)",
			resourceID, cr.Status.Resource.AsyncOperationID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
}
