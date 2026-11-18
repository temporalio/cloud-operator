package apikey

import (
	"context"
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

	cr := latest.CR().(*tcloudv1alpha1.APIKey)
	var retain bool
	if retain, err = deletion.CheckPolicy(ctx, latest, resourceKind); err != nil {
		return latest, err
	}
	if retain {
		err = r.releaseTokenSecret(ctx, cr)
		return latest, err
	}

	tcc, err := r.CloudClientFor(ctx, latest)
	if err != nil {
		err = fmt.Errorf("failed getting tcloud client: %w", err)
		return latest, err
	}

	var done bool
	if done, err = deletion.CheckPrevious(
		ctx, tcc, latest, &cr.Status.Resource, resourceKind,
	); done {
		return latest, err
	}

	err = deleteAPIKey(ctx, tcc, latest)
	return latest, err
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
	if err = deletion.CheckInFlight(&cr.Status.Resource, state, resourceKind, resourceID); err != nil {
		return err
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
