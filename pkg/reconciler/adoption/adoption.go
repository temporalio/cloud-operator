// Package adoption provides helper functions to adopt an existing Temporal
// Cloud resource.
package adoption

import (
	"fmt"
	"time"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// Wanted returns true if desired asks to adopt an existing resource and
// hasn't been adopted yet.
func Wanted(desired types.Resource) bool {
	return desired.AdoptionPolicy() == cloudcommon.AdoptionPolicyAdopt &&
		!desired.IsAdopted()
}

// Adopt checks whether the target resource on Temporal Cloud can be adopted.
// If so, it returns a copy of latest with adopted-on timestamp set.
// Otherwise, it returns a copy with the CR's previous status so it isn't linked.
func Adopt(
	desired types.Resource,
	latest types.Resource,
	kind string,
	now time.Time,
	check func(latest types.Resource) error,
) (kttypes.Resource, error) {
	foundID := latest.ResourceStatus().ID
	if foundID == "" {
		// The resource may have disappeared after an earlier reconcile saved
		// its ID. Retry without creating a new one.
		return Unchanged(desired), requeue.NeededAfter(
			fmt.Errorf("no tcloud %s with ID %q to adopt", kind, desired.ResourceID()),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	if check != nil {
		if err := check(latest); err != nil {
			return Unchanged(desired), ctrlerrors.NewTerminalError(
				fmt.Errorf("%w: %w", ctrlerrors.ErrResourceSync, err),
			)
		}
	}

	switch state := latest.ResourceState(); state {
	case cloudcommon.ResourceStateActive,
		cloudcommon.ResourceStateActivating,
		cloudcommon.ResourceStateUpdating:
	default:
		return Unchanged(desired), ctrlerrors.NewTerminalError(fmt.Errorf(
			"%w: can't adopt tcloud %s %q in state %q",
			ctrlerrors.ErrResourceSync, kind, foundID, state,
		))
	}

	adopted := latest.DeepCopy().(types.Resource)
	adopted.SetAdoptedOn(now)
	return adopted, nil
}

// Unchanged prevents a failed adoption from saving the found resource's ID.
// It returns a copy of desired with the CR's original status, which
// reconcileComplete saves even when reconciliation fails.
func Unchanged(desired types.Resource) kttypes.Resource {
	out := desired.DeepCopy()
	condition.Clear(out)
	return out
}

// CheckNotRecreated returns a terminal error if desired was adopted, for use
// when its resource is gone from Temporal Cloud. The operator doesn't
// recreate an adopted resource.
func CheckNotRecreated(desired types.Resource, kind string) error {
	if !desired.IsAdopted() {
		return nil
	}
	return ctrlerrors.NewTerminalError(fmt.Errorf(
		"%w: adopted tcloud %s %q no longer exists in Temporal Cloud; "+
			"the operator doesn't recreate an adopted %s, "+
			"delete this CR to stop managing it",
		ctrlerrors.ErrResourceSync, kind, desired.ResourceID(), kind,
	))
}
