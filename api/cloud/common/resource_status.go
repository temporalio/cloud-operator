package common

import (
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceStatus describes the state of a Temporal Cloud resource as reported
// by Temporal Cloud. Not every resource type reports every field. Fields that
// a resource type does not report are left unset.
//
// Ref: https://github.com/temporalio/cloud-api/blob/main/temporal/api/cloud/resource/v1/message.proto
type ResourceStatus struct {
	// ID is the identifier Temporal Cloud uses for the resource. Not set for
	// resources that Temporal Cloud identifies by name.
	// Maps to Temporal Cloud proto `id`, or `Namespace.namespace` for
	// Namespaces.
	// +optional
	ID string `json:"id,omitempty"`
	// ResourceVersion is the version of the resource in Temporal Cloud. It is
	// sent with each update and delete request.
	// Maps to Temporal Cloud proto `resource_version`.
	// +optional
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// State is the state of the resource in Temporal Cloud.
	// Maps to Temporal Cloud proto `state`.
	// +optional
	State ResourceState `json:"state,omitempty"`
	// AsyncOperationID is the ID of the most recent asynchronous operation
	// started on the resource.
	// Maps to Temporal Cloud proto `async_operation_id`.
	// +optional
	AsyncOperationID string `json:"asyncOperationID,omitempty"`
	// CreatedTime is when the resource was created in Temporal Cloud.
	// Maps to Temporal Cloud proto `created_time`.
	// +optional
	CreatedTime *metav1.Time `json:"createdTime,omitempty"`
	// LastModifiedTime is when the resource was last modified in Temporal
	// Cloud.
	// Maps to Temporal Cloud proto `last_modified_time`.
	// +optional
	LastModifiedTime *metav1.Time `json:"lastModifiedTime,omitempty"`
}

// ResourceState is the state of a resource in Temporal Cloud.
// Maps to Temporal Cloud proto `ResourceState`.
// +kubebuilder:validation:Enum=Activating;ActivationFailed;Active;Updating;UpdateFailed;Deleting;DeleteFailed;Deleted;Suspended;Expired
type ResourceState string

// FromProto sets the state from a Temporal Cloud resource state. Unspecified
// and unknown states map to "": the CR field is enum-validated, so an
// unrecognized value would fail the status patch.
func (s *ResourceState) FromProto(in tcloudresourcev1.ResourceState) {
	switch in {
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATING:
		*s = ResourceStateActivating
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATION_FAILED:
		*s = ResourceStateActivationFailed
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE:
		*s = ResourceStateActive
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING:
		*s = ResourceStateUpdating
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATE_FAILED:
		*s = ResourceStateUpdateFailed
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETING:
		*s = ResourceStateDeleting
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETE_FAILED:
		*s = ResourceStateDeleteFailed
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETED:
		*s = ResourceStateDeleted
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_SUSPENDED:
		*s = ResourceStateSuspended
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_EXPIRED:
		*s = ResourceStateExpired
	default:
		*s = ""
	}
}

// ToProto returns the Temporal Cloud resource state for the state. The CR
// field is enum-validated, so unknown states cannot occur; they map to
// unspecified.
func (s ResourceState) ToProto() tcloudresourcev1.ResourceState {
	switch s {
	case ResourceStateActivating:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATING
	case ResourceStateActivationFailed:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATION_FAILED
	case ResourceStateActive:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE
	case ResourceStateUpdating:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING
	case ResourceStateUpdateFailed:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATE_FAILED
	case ResourceStateDeleting:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETING
	case ResourceStateDeleteFailed:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETE_FAILED
	case ResourceStateDeleted:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETED
	case ResourceStateSuspended:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_SUSPENDED
	case ResourceStateExpired:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_EXPIRED
	default:
		return tcloudresourcev1.ResourceState_RESOURCE_STATE_UNSPECIFIED
	}
}

const (
	// ResourceStateActivating means the resource is being activated.
	ResourceStateActivating ResourceState = "Activating"
	// ResourceStateActivationFailed means the resource failed to activate.
	ResourceStateActivationFailed ResourceState = "ActivationFailed"
	// ResourceStateActive means the resource is active and ready to use.
	ResourceStateActive ResourceState = "Active"
	// ResourceStateUpdating means the resource is being updated.
	ResourceStateUpdating ResourceState = "Updating"
	// ResourceStateUpdateFailed means the resource failed to update.
	ResourceStateUpdateFailed ResourceState = "UpdateFailed"
	// ResourceStateDeleting means the resource is being deleted.
	ResourceStateDeleting ResourceState = "Deleting"
	// ResourceStateDeleteFailed means the resource failed to delete.
	ResourceStateDeleteFailed ResourceState = "DeleteFailed"
	// ResourceStateDeleted means the resource has been deleted.
	ResourceStateDeleted ResourceState = "Deleted"
	// ResourceStateSuspended means the resource is suspended and not
	// available for use.
	ResourceStateSuspended ResourceState = "Suspended"
	// ResourceStateExpired means the resource has expired and is no longer
	// available for use.
	ResourceStateExpired ResourceState = "Expired"
)
