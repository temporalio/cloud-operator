package namespace

import (
	"context"
	"errors"
	"fmt"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
)

// checkProjectMove checks the in-progress move recorded in
// updated.Status.ProjectMove. It requeues while the move runs, clears the
// record and requeues once the move succeeds, and records the failure and
// returns a terminal error once the move fails.
func checkProjectMove(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	resourceID string,
	updated *tcloudv1alpha1.Namespace, // mutated by this function
) error {
	move := updated.Status.ProjectMove
	err := cloud.CheckAsyncOperation(ctx, tcc, move.AsyncOperationID)
	if errors.Is(err, cloud.ErrAsyncOperationFailed) {
		err = fmt.Errorf(
			"moving tcloud namespace %q to project %q failed: %w",
			resourceID, move.DestinationProject, err,
		)
		move.Failure = err.Error()
		return err
	}
	if err != nil {
		return fmt.Errorf(
			"waiting for tcloud namespace %q to move to project %q: %w",
			resourceID, move.DestinationProject, err,
		)
	}

	updated.Status.ProjectMove = nil
	// Requeue so the next reconcile reads the Namespace in its new project.
	return requeue.NeededAfter(
		fmt.Errorf("moved tcloud namespace %q to project %q", resourceID, move.DestinationProject),
		requeue.DefaultRequeueAfterDuration,
	)
}

// checkMoveAllowed returns a terminal error when the Namespace can't move to
// the project `moveTo` until the user changes the spec.
func checkMoveAllowed(
	resourceID string,
	moveTo string,
	desired *tcloudv1alpha1.Namespace,
	latest *tcloudv1alpha1.Namespace,
) error {
	// Retrying on every reconcile would start a move that fails the same way
	// each time, so a failed move waits for a spec change.
	if m := latest.Status.ProjectMove; m != nil && m.Failure != "" &&
		m.DestinationProject == moveTo && m.Generation == desired.Generation {
		return ctrlerrors.NewTerminalError(fmt.Errorf(
			"%w: %s (async operation %s), change the spec to retry",
			ctrlerrors.ErrResourceSync, m.Failure, m.AsyncOperationID,
		))
	}
	// Connectivity rules belong to a project, so the move would need rules
	// in the destination project, which the operator doesn't manage yet.
	if len(desired.Spec.ConnectivityRuleIDs) > 0 {
		return ctrlerrors.NewTerminalError(fmt.Errorf(
			"%w: tcloud namespace %q can't move to project %q while "+
				"spec.connectivityRuleIDs is set, remove the connectivity rules first",
			ctrlerrors.ErrResourceSync, resourceID, moveTo,
		))
	}
	return nil
}

// moveNamespace starts moving the Namespace to the project `moveTo` and
// records the move in updated.Status.ProjectMove.
func moveNamespace(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	resourceID string,
	moveTo string,
	desired *tcloudv1alpha1.Namespace,
	latest *tcloudv1alpha1.Namespace,
	updated *tcloudv1alpha1.Namespace, // mutated by this function
) error {
	resp, err := tcc.MoveNamespaceToProject(ctx, &tcloudservicev1.MoveNamespaceToProjectRequest{
		Namespace:               resourceID,
		DestinationProjectId:    moveTo,
		ExpectedSourceProjectId: latest.Status.References.Project,
		ResourceVersion:         latest.Status.Resource.Version,
	})
	if err != nil {
		// Temporal Cloud returns PermissionDenied when projects aren't enabled
		// for the account or the API key can't move the Namespace. Retrying
		// won't help until someone changes that.
		if status.Code(err) == codes.PermissionDenied {
			return ctrlerrors.NewTerminalError(fmt.Errorf(
				"%w: failed moving tcloud namespace %q to project %q: %w",
				ctrlerrors.ErrResourceSync, resourceID, moveTo, err,
			))
		}
		return cloud.CallError("moving tcloud namespace", err)
	}

	opID := resp.GetAsyncOperation().GetId()
	updated.Status.ProjectMove = &tcloudv1alpha1.NamespaceProjectMove{
		DestinationProject: moveTo,
		AsyncOperationID:   opID,
		Generation:         desired.Generation,
	}
	return requeue.NeededAfter(
		fmt.Errorf(
			"moving tcloud namespace %q to project %q (async operation %s)",
			resourceID, moveTo, opID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
}
