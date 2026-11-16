// Package deletion holds the checks that every controller runs before
// deleting a resource in Temporal Cloud.
//
// Each function takes a kind, such as "namespace" or "service account", that
// names the resource in messages.
package deletion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// CheckPolicy reports whether the resource's deletion policy keeps it in
// Temporal Cloud. An invalid policy marks the CR as stalled and returns a
// requeue error.
func CheckPolicy(
	ctx context.Context,
	res types.Resource,
	kind string,
) (bool, error) {
	policy := res.DeletionPolicy()
	if err := policy.Validate(); err != nil {
		// Never fall back to delete on an invalid value. Annotation edits
		// don't trigger a reconcile, so requeue to pick up a corrected value.
		msg := fmt.Sprintf(
			"%s. Set the %s annotation to %q or %q",
			err, cloudcommon.AnnotationKeyDeletionPolicy,
			cloudcommon.DeletionPolicyRetain, cloudcommon.DeletionPolicyDelete,
		)
		condition.SetStalled(
			res, metav1.ConditionTrue, msg, cloudcommon.ReasonResourceDeleteFailed,
		)
		return false, requeue.NeededAfter(errors.New(msg), requeue.DefaultRequeueAfterDuration)
	}
	if policy == cloudcommon.DeletionPolicyRetain {
		ctrlreqlog.FromContext(ctx).Info(
			fmt.Sprintf("retaining Temporal Cloud %s because of the retain deletion policy", kind),
			slog.String("kind", kind),
			slog.String("resource_id", res.ResourceID()),
			slog.String("deletion_policy", string(policy)),
		)
		return true, nil
	}
	return false, nil
}

// CheckPrevious reports whether an earlier delete means no new delete should
// be sent, and returns the result to use instead. When the earlier delete's
// async operation failed, it sets st.State to DeleteFailed and marks the CR
// as stalled.
//
// st must point into the CR that res wraps, for example
// &cr.Status.Resource.
func CheckPrevious(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	res types.Resource,
	st *cloudcommon.ResourceStatus,
	kind string,
) (bool, error) {
	resourceID := res.ResourceID()

	switch {
	case st.State == cloudcommon.ResourceStateDeleted:
		return true, nil
	case st.State == cloudcommon.ResourceStateDeleteFailed:
		// A previous delete failed. Don't send another one automatically.
		return true, ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"deleting Temporal Cloud %s %s previously failed "+
					"(async operation %s); not retrying",
				kind, resourceID, st.AsyncOperationID,
			),
		)
	case st.State == cloudcommon.ResourceStateDeleting && st.AsyncOperationID != "":
		err := cloud.CheckAsyncOperation(ctx, tcc, st.AsyncOperationID)
		if errors.Is(err, cloud.ErrAsyncOperationFailed) {
			err = fmt.Errorf("deleting tcloud %s %s failed: %w", kind, resourceID, err)
			condition.SetStalled(
				res, metav1.ConditionTrue, err.Error(), cloudcommon.ReasonResourceDeleteFailed,
			)
			st.State = cloudcommon.ResourceStateDeleteFailed
		}
		return true, err
	}
	return false, nil
}

// CheckProtection returns an error when the CR's spec enables deletion
// protection, because Temporal Cloud rejects deleting a protected resource.
// lc is the CR's spec.lifecycle and may be nil. Changing the spec triggers a
// reconcile, but switching the annotation to retain doesn't, so the error
// requeues to pick that up.
func CheckProtection(
	res types.Resource,
	lc *cloudcommon.LifecycleSpec,
	kind string,
) error {
	if lc == nil || !lc.EnableDeleteProtection {
		return nil
	}
	msg := fmt.Sprintf(
		"tcloud %s %s has deletion protection enabled; set "+
			"spec.lifecycle.enableDeleteProtection to false to delete it, "+
			"or set the %s annotation to %q to keep it",
		kind, res.ResourceID(), cloudcommon.AnnotationKeyDeletionPolicy,
		cloudcommon.DeletionPolicyRetain,
	)
	condition.SetStalled(
		res, metav1.ConditionTrue, msg, cloudcommon.ReasonResourceDeleteFailed,
	)
	return requeue.NeededAfter(errors.New(msg), requeue.DefaultRequeueAfterDuration)
}

// CheckInFlight returns a requeue error when state, as just read from Temporal
// Cloud, shows a create or update still in flight. Temporal Cloud rejects a
// delete until the resource is Active again. It saves state to st.State so
// the CR's status shows what the delete is waiting on.
//
// Use the state from a fresh read, not the CR's status: the CR passed to
// ResourceDelete holds the status from the last reconcile, which can be
// stale.
func CheckInFlight(
	st *cloudcommon.ResourceStatus,
	state cloudcommon.ResourceState,
	kind string,
	resourceID string,
) error {
	if state != cloudcommon.ResourceStateActivating && state != cloudcommon.ResourceStateUpdating {
		return nil
	}
	st.State = state
	return requeue.NeededAfter(
		fmt.Errorf(
			"waiting for tcloud %s %s to finish %s before deleting",
			kind, resourceID, state,
		),
		requeue.DefaultRequeueAfterDuration,
	)
}
