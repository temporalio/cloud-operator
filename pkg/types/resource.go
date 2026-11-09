package types

import (
	kttypes "github.com/temporalio/kube-temporal/pkg/types"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// Resource wraps the upstream kube-temporal interface of the same name and
// adds Temporal Cloud Operator-specific methods for handling adoption and
// understanding common Temporal Cloud resource lifecycle.
type Resource interface {
	kttypes.Resource
	Adoptable
	Deletable

	// PreviouslyReconciled returns true if the Resource has previously been
	// successfully reconciled and had the Status.Resource.ID field set to a
	// non-empty value.
	PreviouslyReconciled() bool

	// ResourceID returns the Temporal Cloud resource identifier for the
	// Resource. If the Resource has been previously reconciled, returns
	// Status.Resource.ID, otherwise returns the value of the
	// `cloud.temporal.io/resource-id` annotation, if any.
	ResourceID() string

	// ResourceStatus returns the common ResourceStatus struct associated with
	// the Resource.
	ResourceStatus() tcloudcommon.ResourceStatus

	// ResourceState returns the common ResourceStatus.State field.
	ResourceState() tcloudcommon.ResourceState
}
