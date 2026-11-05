package common

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
)

const (
	AnnotationKeyPrefixTemporalCloud   = "cloud.temporal.io/"
	AnnotationKeyTemporalCloudHostPort = AnnotationKeyPrefixTemporalCloud + "hostport"

	// AnnotationKeyDeletionPolicy controls what happens to the Temporal Cloud
	// resource when the custom resource that manages it is deleted. Its value
	// is a DeletionPolicy.
	AnnotationKeyDeletionPolicy = AnnotationKeyPrefixTemporalCloud + "deletion-policy"

	// AnnotationKeyAdoptionPolicy asks the controller to adopt an existing
	// Temporal Cloud resource instead of creating a new one. Its value is an
	// AdoptionPolicy.
	AnnotationKeyAdoptionPolicy = AnnotationKeyPrefixTemporalCloud + "adoption-policy"

	// ReasonResourceDeleteFailed is the Stalled condition reason when Temporal
	// Cloud failed to delete the backend resource.
	ReasonResourceDeleteFailed = "ResourceDeleteFailed"
)

// DeletionPolicy describes the expected behaviour of the controller when
// deleting a Temporal Cloud resource associated with a CR.
type DeletionPolicy string

// AdoptionPolicy describes how the controller adopts an existing Temporal
// Cloud resource.
type AdoptionPolicy string

// ResourceSource describes where the Temporal Cloud resource managed by a CR
// came from.
// +kubebuilder:validation:Enum=Adopted
type ResourceSource string

const (
	// DeletionPolicyDelete deletes the Temporal Cloud resource, and removes
	// the custom resource once Temporal Cloud has finished deleting it.
	DeletionPolicyDelete DeletionPolicy = "delete"
	// DeletionPolicyRetain leaves the Temporal Cloud resource in place and
	// only removes the custom resource.
	DeletionPolicyRetain DeletionPolicy = "retain"
	// AdoptionPolicyAdopt imports an existing Temporal Cloud resource as is.
	AdoptionPolicyAdopt AdoptionPolicy = "adopt"

	// ResourceSourceAdopted means the controller adopted a Temporal Cloud
	// resource that already existed.
	ResourceSourceAdopted ResourceSource = "Adopted"
)

// validDeletionPolicies are the valid DeletionPolicy values.
var validDeletionPolicies = []string{
	string(DeletionPolicyRetain),
	string(DeletionPolicyDelete),
}

// validAdoptionPolicies are the valid AdoptionPolicy values.
var validAdoptionPolicies = []string{
	string(AdoptionPolicyAdopt),
}

// Validate returns an error if the DeletionPolicy is not one of the valid
// values.
func (p DeletionPolicy) Validate() error {
	if ContainsFold(string(p), validDeletionPolicies) {
		return nil
	}
	allowed := strings.Join(validDeletionPolicies, ", ")
	return fmt.Errorf("invalid deletion policy %q. allowed values: %s", p, allowed)
}

func (p AdoptionPolicy) Validate() error {
	if ContainsFold(string(p), validAdoptionPolicies) {
		return nil
	}
	allowed := strings.Join(validAdoptionPolicies, ", ")
	return fmt.Errorf("invalid adoption policy %q. allowed values: %s", p, allowed)

}

// ContainsFold returns true if the subject matches one of the
// supplied valid values.
func ContainsFold(subject string, validValues []string) bool {
	return lo.ContainsBy(validValues, func(v string) bool {
		return strings.EqualFold(subject, v)
	})
}
