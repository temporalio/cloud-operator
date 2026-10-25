package common

// +kubebuilder:validation:Enum=Retain;Delete
type DeletionPolicy string

const (
	// DeletionPolicyRetain leaves the Temporal Cloud resource in place and
	// only removes the custom resource
	DeletionPolicyRetain DeletionPolicy = "Retain"

	// DeletionPolicyDelete deletes the Temporal Cloud resource, and removes
	// the custom resource after Temporal Cloud finishes deleting it.
	DeletionPolicyDelete DeletionPolicy = "Delete"
)
