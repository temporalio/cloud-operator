package namespace

import (
	"context"
	"errors"
	"fmt"
	"time"

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
)

const (
	// ReasonResourceDeleteFailed is the Stalled condition reason when Temporal
	// Cloud failed to delete the Namespace.
	ReasonResourceDeleteFailed = "ResourceDeleteFailed"

	// deletePollInterval is how often to check on a delete in progress.
	deletePollInterval = 15 * time.Second
)

// ResourceDelete deletes the Temporal Cloud Namespace for the supplied
// [types.Resource], unless its deletion policy is retain. It returns nil once
// the Namespace is gone, and an error while the delete is in progress or has
// failed.
//
// TODO(Mechack): this relies on the kube-temporal Controller persisting the
// status set here (State and AsyncOperationID) and treating nil as "remove
// the finalizer" and a requeue error as "keep it and requeue". Today it passes
// desired and drops the result on error. Remove this once that's fixed.
//
// TODO(Mechack): status.id and status.resourceVersion are set by the create path.
func (r *Reconciler) ResourceDelete(
	ctx context.Context,
	desired types.Resource,
) error {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.delete")
	defer func() {
		exit(err)
	}()

	policy, err := deletionPolicy(desired)
	if err != nil {
		return err
	}
	if policy == tcloudcommon.DeletionPolicyRetain {
		return nil
	}

	cr := desired.CR().(*tcloudv1alpha1.Namespace)
	if cr.Status.ID == "" {
		// The operator never created a Namespace in Temporal Cloud.
		return nil
	}

	tcc, err := r.CloudClientFor(ctx, desired)
	if err != nil {
		err = fmt.Errorf("failed getting tcloud client: %w", err)
		return err
	}

	// We may have already called DeleteNamespace for this resource. If so,
	// check whether the delete operation has finished.
	switch {
	case cr.Status.State == tcloudcommon.ResourceStateDeleted:
		return nil
	case cr.Status.State == tcloudcommon.ResourceStateDeleting &&
		cr.Status.AsyncOperationID != "":
		err = checkDelete(ctx, tcc, desired, cr)
		return err
	}

	// TODO(Mechack): uses the CR's last recorded resourceVersion. If it's stale,
	// Temporal Cloud rejects the delete with "resource version mismatch". Switch to
	// the version from a fresh GetNamespace if that becomes a problem.
	resp, err := tcc.DeleteNamespace(ctx, &tcloudservicev1.DeleteNamespaceRequest{
		Namespace:       cr.Status.ID,
		ResourceVersion: cr.Status.ResourceVersion,
	})
	if status.Code(err) == codes.NotFound {
		// Already gone.
		err = nil
		return nil
	}
	if err != nil {
		// The gRPC error includes Temporal Cloud's message, for example that
		// deletion protection is enabled.
		err = fmt.Errorf("failed deleting tcloud namespace %s: %w", cr.Status.ID, err)
		condition.SetStalled(
			desired, metav1.ConditionTrue, err.Error(), ReasonResourceDeleteFailed,
		)
		return err
	}

	cr.Status.State = tcloudcommon.ResourceStateDeleting
	cr.Status.AsyncOperationID = resp.GetAsyncOperation().GetId()
	err = requeue.NeededAfter(
		fmt.Errorf(
			"deleting tcloud namespace %s (async operation %s)",
			cr.Status.ID, cr.Status.AsyncOperationID,
		),
		deletePollInterval,
	)
	return err
}

// checkDelete checks the async operation for a delete already sent to
// Temporal Cloud. It returns nil once the operation is fulfilled, a requeue
// error while it is running, and an error with the Stalled condition set if
// it failed.
func checkDelete(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired types.Resource,
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
			desired, metav1.ConditionTrue, msg, ReasonResourceDeleteFailed,
		)
		return errors.New(msg)
	default:
		// Pending.
		return requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud async operation %s (%s)",
				op.GetId(), op.GetState(),
			),
			deletePollInterval,
		)
	}
}

// deletionPolicy returns the subject's deletion policy from its
// cloud.temporal.io/deletion-policy annotation. An unset annotation means
// delete. Any other value is an error and never falls back to delete.
//
// TODO(Mechack/10/26/2026): once adoption is supported, the default for an adopted
// Namespace becomes retain. An explicit delete is honored either way.
func deletionPolicy(subject types.Resource) (string, error) {
	key := tcloudcommon.AnnotationKeyDeletionPolicy
	v, ok := annotation.Value(subject, key)
	if !ok {
		// annotation.Value reports false when more than one key matches
		// case-insensitively, so check that the key is really absent.
		if annotation.HasKey(subject, key) {
			return "", fmt.Errorf("more than one %s annotation", key)
		}
		return tcloudcommon.DeletionPolicyDelete, nil
	}
	switch v {
	case tcloudcommon.DeletionPolicyDelete, tcloudcommon.DeletionPolicyRetain:
		return v, nil
	default:
		return "", fmt.Errorf(
			"invalid %s annotation %q: must be %q or %q",
			key, v,
			tcloudcommon.DeletionPolicyDelete, tcloudcommon.DeletionPolicyRetain,
		)
	}
}
