package types

import (
	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// Deletable describes something that can be deleted from the Temporal Cloud
// API.
type Deletable interface {
	// DeletionPolicy returns the deletion policy for the Deletable.
	DeletionPolicy() tcloudcommon.DeletionPolicy
	// SetDeletionPolicy sets the delation policy for the Deletable.
	SetDeletionPolicy(tcloudcommon.DeletionPolicy)
}
