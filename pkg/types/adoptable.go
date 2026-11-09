package types

import (
	"time"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// Adoptable describes something that can be adopted by the Temporal Cloud
// Operator.
//
// A resource being adopted by TCO means that TCO has taken over responsibility
// for managing the lifecycle of an existing resource.
type Adoptable interface {
	// IsAdopted returns true if the resource was adopted by TCO.
	IsAdopted() bool

	// AdoptedOn returns the time that TCO adopted the resource. Returns nil
	// when the resource has not been adopted.
	//
	// Returns an error if the resource adopted-on timestamp was found but was
	// not a valid RFC 3339 UTC timestamp string.
	AdoptedOn() (*time.Time, error)
	// SetAdoptedOn sets the resource's adopted on timestamp.
	SetAdoptedOn(time.Time)

	// AdoptionPolicy returns the adoption policy on the resource.
	AdoptionPolicy() tcloudcommon.AdoptionPolicy
	// SetAdoptionPolicy sets the resource's adoption policy annotation.
	SetAdoptionPolicy(tcloudcommon.AdoptionPolicy)
}
