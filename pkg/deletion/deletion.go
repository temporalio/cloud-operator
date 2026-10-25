// Package deletion provides shared logic for handling deleted custom resources.
// It makes no API calls; each resource uses live Temporal Cloud state to decide what to do.
package deletion

import (
	"fmt"
	"time"

	"github.com/temporalio/cloud-operator/api/cloud/common"
)

// Action is what to do next with a custom resource that is being deleted.
type Action int

const (
	// ActionRelease means the finalizer may be removed. Temporal Cloud is
	// left as it is.
	ActionRelease Action = iota + 1
	// ActionWait means Temporal Cloud is busy with the resource.
	ActionWait
	// ActionBlock means a person must unblock the delete.
	ActionBlock
	// ActionFail means the resource can't be deleted; stop until
	// the custom resource changes.
	ActionFail
	// ActionDelete means send the delete request to Temporal Cloud.
	ActionDelete
)

// String returns the Action's name.
func (a Action) String() string {
	switch a {
	case ActionRelease:
		return "Release"
	case ActionWait:
		return "Wait"
	case ActionBlock:
		return "Block"
	case ActionFail:
		return "Fail"
	case ActionDelete:
		return "Delete"
	default:
		return fmt.Sprintf("Action(%d)", int(a))
	}
}

// Decision is what to do next, and why.
type Decision struct {
	Action Action
	// Reason is a condition reason.
	Reason string
	// Message explains the decision to the user.
	Message string
}

// Record is what the operator recorded about the Temporal Cloud resource it
// created.
type Record struct {
	// ID is the resource's Temporal Cloud ID. Only the create path sets it, so
	// a non-empty ID means the operator created the resource.
	ID string
	// CreatedTime is the creation time Temporal Cloud reported for the
	// resource.
	CreatedTime *time.Time
}

// Reasons for the decisions Preflight makes.
const (
	ReasonInvalidDeletionPolicy = "InvalidDeletionPolicy"
	ReasonRetained              = "Retained"
	ReasonNeverCreated          = "NeverCreated"
)

// RetainHint tells the user how to remove a custom resource whose Temporal
// Cloud resource can't be deleted.
const RetainHint = "set spec.deletionPolicy to Retain to remove this resource and leave it in Temporal Cloud"

// Resolve returns the effective deletion policy
func Resolve(policy, def common.DeletionPolicy) (common.DeletionPolicy, error) {
	p := policy
	if p == "" {
		p = def
	}
	switch p {
	case common.DeletionPolicyRetain, common.DeletionPolicyDelete:
		return p, nil
	default:
		return "", fmt.Errorf("unknown deletion policy %q", p)
	}
}

// Preflight decides from the custom resource alone, without reading Temporal
// Cloud. It returns true when the returned Decision is final. When it returns
// false, the caller reads the resource from Temporal Cloud and decides with
// the resource's own rules.
func Preflight(
	policy common.DeletionPolicy,
	def common.DeletionPolicy,
	rec Record,
) (Decision, bool) {
	resolved, err := Resolve(policy, def)
	if err != nil {
		return Decision{
			Action:  ActionFail,
			Reason:  ReasonInvalidDeletionPolicy,
			Message: fmt.Sprintf("%v; set spec.deletionPolicy to Retain or Delete", err),
		}, true
	}
	if resolved == common.DeletionPolicyRetain {
		return Decision{
			Action:  ActionRelease,
			Reason:  ReasonRetained,
			Message: "deletion policy is Retain; leaving the resource in Temporal Cloud",
		}, true
	}
	if rec.ID == "" {
		return Decision{
			Action:  ActionRelease,
			Reason:  ReasonNeverCreated,
			Message: "the operator never created this resource in Temporal Cloud; nothing to delete",
		}, true
	}
	return Decision{}, false
}

// SameSecond reports whether recorded and observed are the same time to the
// second. Recorded times are stored as metav1.Time, which keeps whole seconds
// only. A nil time never matches.
func SameSecond(recorded, observed *time.Time) bool {
	if recorded == nil || observed == nil {
		return false
	}
	return recorded.Truncate(time.Second).Equal(observed.Truncate(time.Second))
}
