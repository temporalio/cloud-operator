package namespace

import (
	"fmt"
	"strings"
	"time"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// adoptNamespace checks whether the Namespace found by ResourceLatest can be
// adopted. If so, it returns a copy of latest with adopted-on timestamp set.
// Otherwise, it returns a copy with the CR's previous status so it isn't linked.
func adoptNamespace(
	desired types.Resource,
	latest types.Resource,
	now time.Time,
) (kttypes.Resource, error) {
	if latest.ResourceStatus().ID == "" {
		return resetLatestStatus(desired, latest), requeue.NeededAfter(
			fmt.Errorf("no tcloud namespace with ID %q to adopt", desired.ResourceID()),
			requeue.DefaultRequeueAfterDuration,
		)
	}
	ok, reason := resourceCanBeAdopted(desired, latest)
	if !ok {
		return resetLatestStatus(desired, latest), ctrlerrors.NewTerminalError(
			fmt.Errorf("%w: %s", ctrlerrors.ErrResourceSync, reason),
		)
	}
	adopted := latest.DeepCopy().(types.Resource)
	adopted.SetAdoptedOn(now)
	return adopted, nil
}

// resetLatestStatus prevents a failed adoption from saving the found Namespace's ID.
// It returns a copy of latest with the CR's original status, which
// reconcileComplete saves even when reconciliation fails.
func resetLatestStatus(desired, latest types.Resource) kttypes.Resource {
	out := latest.DeepCopy()
	outNS := out.CR().(*tcloudv1alpha1.Namespace)
	desiredNS := desired.CR().(*tcloudv1alpha1.Namespace)
	desiredNS.Status.Resource.DeepCopyInto(&outNS.Status.Resource)
	outNS.Status.References = desiredNS.Status.References
	return out
}

// wantsAdoption returns true if desired asks to adopt an existing Namespace
// and hasn't been adopted yet.
func wantsAdoption(desired types.Resource) bool {
	return desired.AdoptionPolicy() == cloudcommon.AdoptionPolicyAdopt &&
		!desired.IsAdopted()
}

// resourceCanBeAdopted reports whether the Namespace that ResourceLatest
// found can be adopted.
func resourceCanBeAdopted(desired, latest types.Resource) (bool, string) {
	foundID := latest.ResourceStatus().ID

	// A Namespace ID is "<name>.<account ID>", and names can't contain dots.
	name, _, _ := strings.Cut(foundID, ".")
	if crName := desired.CR().(*tcloudv1alpha1.Namespace).Name; name != crName {
		return false, fmt.Sprintf(
			"can't adopt tcloud namespace %q: its name %q doesn't match the CR's name %q",
			foundID, name, crName,
		)
	}

	switch state := latest.ResourceState(); state {
	case cloudcommon.ResourceStateActive,
		cloudcommon.ResourceStateActivating,
		cloudcommon.ResourceStateUpdating:
		return true, ""
	default:
		return false, fmt.Sprintf(
			"can't adopt tcloud namespace %q in state %q", foundID, state,
		)
	}
}
