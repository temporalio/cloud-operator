package common

import (
	"fmt"
	"strings"
)

const (
	AnnotationKeyPrefixTemporalCloud   = "cloud.temporal.io/"
	AnnotationKeyTemporalCloudHostPort = AnnotationKeyPrefixTemporalCloud + "hostport"

	// AnnotationKeyDeletionPolicy controls what happens to the Temporal Cloud
	// resource when the custom resource that manages it is deleted. Its value
	// is a DeletionPolicy.
	AnnotationKeyDeletionPolicy = AnnotationKeyPrefixTemporalCloud + "deletion-policy"

	// ReasonResourceDeleteFailed is the Stalled condition reason when Temporal
	// Cloud failed to delete the backend resource.
	ReasonResourceDeleteFailed = "ResourceDeleteFailed"
)

// DeletionPolicy describes the expected behaviour of the controller when
// deleting a Temporal Cloud resource associated with a CR.
type DeletionPolicy string

const (
	// DeletionPolicyDelete deletes the Temporal Cloud resource, and removes
	// the custom resource once Temporal Cloud has finished deleting it.
	DeletionPolicyDelete DeletionPolicy = "delete"
	// DeletionPolicyRetain leaves the Temporal Cloud resource in place and
	// only removes the custom resource.
	DeletionPolicyRetain DeletionPolicy = "retain"
)

// Validate returns an error if the DeletionPolicy is not valid.
func (p DeletionPolicy) Validate() error {
	switch p {
	case DeletionPolicyRetain, DeletionPolicyDelete:
		return nil
	default:
		allowed := strings.Join(
			[]string{string(DeletionPolicyRetain), string(DeletionPolicyDelete)},
			", ",
		)
		return fmt.Errorf("invalid deletion policy %q. allowed values: %s", p, allowed)
	}
}
