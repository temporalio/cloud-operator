package common

const (
	AnnotationKeyPrefixTemporalCloud   = "cloud.temporal.io/"
	AnnotationKeyTemporalCloudHostPort = AnnotationKeyPrefixTemporalCloud + "hostport"

	AnnotationKeyDeletionPolicy = AnnotationKeyPrefixTemporalCloud + "deletion-policy"
	// DeletionPolicyDelete deletes the Temporal Cloud resource, and removes
	// the custom resource once Temporal Cloud has finished deleting it.
	DeletionPolicyDelete = "delete"
	// DeletionPolicyRetain leaves the Temporal Cloud resource in place and
	// only removes the custom resource.
	DeletionPolicyRetain = "retain"
)
