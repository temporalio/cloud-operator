package v1alpha1

import (
	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tmprlcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	"github.com/temporalio/kube-temporal/pkg/kube/transform"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserGroup represents an instance of a Temporal Cloud User Group.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.resource.id`
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.spec.access.accountAccess.role`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.resource.state`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type UserGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the User Group.
	// +required
	Spec UserGroupSpec `json:"spec"`
	// Status describes the current status of the User Group.
	// +optional
	Status UserGroupStatus `json:"status,omitempty"`
}

// UserGroupSpec describes the desired state of a Temporal Cloud User Group.
//
// Ref: https://github.com/temporalio/cloud-api/blob/v0.24.0/temporal/api/cloud/identity/v1/message.proto
// +k8s:openapi-gen=true
// +kubebuilder:validation:XValidation:rule="has(self.googleGroup) != has(self.cloudGroup)",message="exactly one of googleGroup or cloudGroup must be set"
// +kubebuilder:validation:XValidation:rule="has(self.googleGroup) == has(oldSelf.googleGroup) && has(self.cloudGroup) == has(oldSelf.cloudGroup)",message="the group type cannot be changed after creation"
// +kubebuilder:validation:XValidation:rule="!has(self.cloudGroup) || self.displayName.matches('^[a-zA-Z0-9_-]{3,64}$')",message="the displayName of a cloudGroup must be 3 to 64 letters, digits, hyphens or underscores",fieldPath=".displayName"
// +kubebuilder:validation:XValidation:rule="!has(self.access) || !has(self.access.accountAccess) || !has(self.access.accountAccess.role) || self.access.accountAccess.role != 'Owner'",message="the Owner role cannot be assigned to a User Group",fieldPath=".access.accountAccess.role"
// +kubebuilder:validation:XValidation:rule="!has(self.access) || !has(self.access.accountAccess) || !has(self.access.accountAccess.role) || self.access.accountAccess.role != 'MetricsRead'",message="the MetricsRead role cannot be assigned to a User Group",fieldPath=".access.accountAccess.role"
// +kubebuilder:validation:XValidation:rule="!has(self.access) || !has(self.access.accountAccess) || !has(self.access.accountAccess.role) || self.access.accountAccess.role != 'Admin' || ((!has(self.access.namespaceAccesses) || size(self.access.namespaceAccesses) == 0) && (!has(self.access.projectAccesses) || size(self.access.projectAccesses) == 0))",message="the Admin role cannot be combined with namespace or project access",fieldPath=".access"
type UserGroupSpec struct {
	// DisplayName is the name of the User Group in Temporal Cloud. It can be
	// changed. Temporal Cloud requires the name of a cloudGroup
	//(a user group where tcloud itself keeps the member list ) to be unique within the account.
	// Maps to Temporal Cloud proto `UserGroupSpec.display_name`.
	// +required
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`
	// Access is the access given to the members of the User Group.
	// Maps to Temporal Cloud proto `UserGroupSpec.access`.
	// +optional
	Access AccountScopedAccess `json:"access,omitzero"`
	// GoogleGroup makes this a User Group whose members come from a Google
	// Workspace group.
	// Maps to Temporal Cloud proto `UserGroupSpec.google_group`.
	// +optional
	GoogleGroup *GoogleGroupSpec `json:"googleGroup,omitempty"`
	// CloudGroup makes this a User Group whose members are managed in
	// Temporal Cloud.
	// Maps to Temporal Cloud proto `UserGroupSpec.cloud_group`.
	// +optional
	CloudGroup *CloudGroupSpec `json:"cloudGroup,omitempty"`
}

// GoogleGroupSpec describes the Google Workspace group of a User Group.
// Maps to Temporal Cloud proto `GoogleGroupSpec`.
type GoogleGroupSpec struct {
	// EmailAddress is the email address of the Google group. It must be
	// lowercase and can't be changed after creation, because Temporal Cloud
	// rejects both.
	// Maps to Temporal Cloud proto `GoogleGroupSpec.email_address`.
	// +required
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=254
	// +kubebuilder:validation:Format=email
	// +kubebuilder:validation:XValidation:rule="self == self.lowerAscii()",message="emailAddress must be lowercase"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="emailAddress cannot be changed after creation"
	EmailAddress string `json:"emailAddress"`
}

// CloudGroupSpec describes a User Group whose members are managed in
// Temporal Cloud. It has no fields.
// Maps to Temporal Cloud proto `CloudGroupSpec`.
type CloudGroupSpec struct{}

// UserGroupStatus describes the current status of a Temporal Cloud User Group.
// +k8s:openapi-gen=true
type UserGroupStatus struct {
	tmprlcorev1.StatusBase `json:",inline"`
	// Resource contains common information about the Temporal Cloud resource
	// that this CR represents.
	// +optional
	Resource cloudcommon.ResourceStatus `json:"resource,omitzero"`
	// References contains the Temporal Cloud IDs of resources the spec refers to.
	// +optional
	References UserGroupReferences `json:"references,omitzero"`
}

// UserGroupReferences contains the Temporal Cloud IDs of the custom resources
// referenced by the spec.
type UserGroupReferences struct {
	// Namespaces are the Namespace CRs referenced by the spec that have a
	// Temporal Cloud ID.
	// +optional
	// +listType=map
	// +listMapKey=namespace
	// +listMapKey=name
	Namespaces []NamespaceResolvedReference `json:"namespaces,omitempty"`
}

// UserGroupList is a list of UserGroup resources.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UserGroupList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []UserGroup `json:"items"`
}

// FromProto returns a UserGroup from the supplied Temporal Cloud UserGroup
// proto message.
func (g UserGroup) FromProto(in *tcloudidentityv1.UserGroup) *UserGroup {
	out := UserGroup{
		TypeMeta:   g.TypeMeta,
		ObjectMeta: g.ObjectMeta,
	}
	out.Status.References = *g.Status.References.DeepCopy()

	if in == nil {
		return &out
	}

	var spec UserGroupSpec
	out.Spec = spec.FromProto(in.GetSpec())

	out.Status.Resource.ID = in.GetId()
	out.Status.Resource.Version = in.GetResourceVersion()
	out.Status.Resource.State.FromProto(in.GetState())
	out.Status.Resource.AsyncOperationID = in.GetAsyncOperationId()
	out.Status.Resource.CreatedTime = transform.TimeFromProto(in.GetCreatedTime())
	out.Status.Resource.LastModifiedTime = transform.TimeFromProto(in.GetLastModifiedTime())
	return &out
}

// FromProto returns a UserGroupSpec from the supplied Temporal Cloud
// UserGroupSpec proto message. A SCIM group has neither GoogleGroup or
// CloudGroup set.
func (s UserGroupSpec) FromProto(in *tcloudidentityv1.UserGroupSpec) UserGroupSpec {
	out := UserGroupSpec{
		DisplayName: in.GetDisplayName(),
		Access:      *accountScopedAccessFromProto(in.GetAccess()),
	}
	switch {
	case in.GetGoogleGroup() != nil:
		out.GoogleGroup = &GoogleGroupSpec{
			EmailAddress: in.GetGoogleGroup().GetEmailAddress(),
		}
	case in.GetCloudGroup() != nil:
		out.CloudGroup = &CloudGroupSpec{}
	}
	return out
}
