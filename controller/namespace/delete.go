package namespace

import (
	"fmt"
	"time"

	"github.com/temporalio/cloud-operator/api/cloud/common"
	"github.com/temporalio/cloud-operator/pkg/deletion"
)

// Reasons for the decisions Decide makes.
const (
	ReasonDeleted              = "Deleted"
	ReasonIdentityUnverified   = "IdentityUnverified"
	ReasonNotCreatedByOperator = "NotCreatedByOperator"
	ReasonDeleteProtected      = "DeleteProtected"
	ReasonNexusEndpointsTarget = "NexusEndpointsTarget"
	ReasonDeleting             = "Deleting"
	ReasonWaiting              = "Waiting"
	ReasonSuspended            = "Suspended"
	ReasonUnknownState         = "UnknownState"
)

// Observed is the live state of a Temporal Cloud Namespace
type Observed struct {
	// Found is false when Temporal Cloud returns NotFound.
	Found bool
	// State is the Namespace's state
	State           common.ResourceState
	ResourceVersion string
	CreatedTime     *time.Time
	// DeleteProtection is the live spec.lifecycle.enableDeleteProtection.
	DeleteProtection bool
	// NexusEndpoints is the number of Nexus endpoints targeting the
	// Namespace.
	NexusEndpoints int
}

// Decide decides what to do with a Namespace whose deletion policy is Delete,
// after deletion.Preflight returned false. It follows Temporal Cloud's delete
// rules: only an Active Namespace that isn't delete-protected, suspended or
// targeted by Nexus endpoints can be deleted.
//
// Namespace IDs are reused once a Namespace is gone, so Decide only deletes a
// Namespace whose creation time matches the one the operator recorded.
func Decide(rec deletion.Record, obs Observed) deletion.Decision {
	switch {
	case !obs.Found, obs.State == common.ResourceStateDeleted:
		return decision(deletion.ActionRelease, ReasonDeleted,
			"namespace %s is deleted in Temporal Cloud", rec.ID)
	case rec.CreatedTime == nil:
		return decision(deletion.ActionFail, ReasonIdentityUnverified,
			"no creation time is recorded for namespace %s, so the operator can't confirm it created it; %s", rec.ID, deletion.RetainHint)
	case !deletion.SameSecond(rec.CreatedTime, obs.CreatedTime):
		return decision(deletion.ActionRelease, ReasonNotCreatedByOperator,
			"namespace %s in Temporal Cloud is not the one the operator created; leaving it in Temporal Cloud", rec.ID)
	}

	switch obs.State {
	case common.ResourceStateActive:
		if obs.DeleteProtection {
			return decision(deletion.ActionBlock, ReasonDeleteProtected,
				"namespace %s has delete protection on; turn it off in Temporal Cloud, or %s", rec.ID, deletion.RetainHint)
		}
		if obs.NexusEndpoints > 0 {
			return decision(deletion.ActionBlock, ReasonNexusEndpointsTarget,
				"Nexus endpoints target namespace %s; delete them first, or %s", rec.ID, deletion.RetainHint)
		}
		return decision(deletion.ActionDelete, ReasonDeleting,
			"deleting namespace %s in Temporal Cloud", rec.ID)
	case common.ResourceStateDeleting,
		common.ResourceStateActivating,
		common.ResourceStateUpdating:
		return decision(deletion.ActionWait, ReasonWaiting,
			"namespace %s is %s in Temporal Cloud; waiting", rec.ID, obs.State)
	case common.ResourceStateSuspended:
		return decision(deletion.ActionBlock, ReasonSuspended,
			"namespace %s is suspended, and Temporal Cloud doesn't delete suspended namespaces; contact Temporal support, or %s", rec.ID, deletion.RetainHint)
	case common.ResourceStateDeleteFailed,
		common.ResourceStateActivationFailed,
		common.ResourceStateUpdateFailed,
		// Temporal Cloud doesn't report Expired for namespaces today.
		common.ResourceStateExpired:
		return decision(deletion.ActionFail, "CloudState"+string(obs.State),
			"namespace %s is %s in Temporal Cloud and can't be deleted; contact Temporal support, or %s", rec.ID, obs.State, deletion.RetainHint)
	default:
		return decision(deletion.ActionWait, ReasonUnknownState,
			"namespace %s has state %q,waiting", rec.ID, obs.State)
	}
}

func decision(
	action deletion.Action,
	reason string,
	format string,
	args ...any,
) deletion.Decision {
	return deletion.Decision{
		Action:  action,
		Reason:  reason,
		Message: fmt.Sprintf(format, args...),
	}
}
