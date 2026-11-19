package v1alpha1

import (
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmprlcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	"github.com/temporalio/kube-temporal/pkg/kube/transform"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// User represents an instance of a Temporal Cloud User.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.resource.id`
// +kubebuilder:printcolumn:name="Email",type=string,JSONPath=`.spec.email`
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.spec.access.accountAccess.role`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.resource.state`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the User.
	// +required
	Spec UserSpec `json:"spec"`
	// Status describes the current status of the User.
	// +optional
	Status UserStatus `json:"status,omitempty"`
}

// FromProto returns a User from the supplied Temporal Cloud User proto
// message.
func (u User) FromProto(in *tcloudidentityv1.User) *User {
	out := User{
		TypeMeta:   u.TypeMeta,
		ObjectMeta: u.ObjectMeta,
	}
	out.Status.References = *u.Status.References.DeepCopy()

	if in == nil {
		return &out
	}

	var spec UserSpec
	out.Spec = spec.FromProto(in.GetSpec())

	out.Status.Resource.ID = in.GetId()
	out.Status.Resource.Version = in.GetResourceVersion()
	out.Status.Resource.State.FromProto(in.GetState())
	out.Status.Resource.AsyncOperationID = in.GetAsyncOperationId()
	out.Status.Resource.CreatedTime = transform.TimeFromProto(in.GetCreatedTime())
	out.Status.Resource.LastModifiedTime = transform.TimeFromProto(in.GetLastModifiedTime())
	out.Status.Invitation = userInvitationFromProto(in.GetInvitation())
	return &out
}

// UserSpec describes the desired state of a Temporal Cloud User.
//
// Ref: https://github.com/temporalio/cloud-api/blob/v0.24.0/temporal/api/cloud/identity/v1/message.proto
// +k8s:openapi-gen=true
// +kubebuilder:validation:XValidation:rule="!has(self.access.accountAccess) || !has(self.access.accountAccess.role) || self.access.accountAccess.role != 'Owner'",message="the Owner role cannot be assigned to a User",fieldPath=".access.accountAccess.role"
// +kubebuilder:validation:XValidation:rule="!has(self.access.accountAccess) || !has(self.access.accountAccess.role) || self.access.accountAccess.role != 'MetricsRead'",message="the MetricsRead role cannot be assigned to a User",fieldPath=".access.accountAccess.role"
// +kubebuilder:validation:XValidation:rule="!has(self.access.accountAccess) || !has(self.access.accountAccess.role) || !(self.access.accountAccess.role in ['Admin', 'Owner']) || ((!has(self.access.namespaceAccesses) || size(self.access.namespaceAccesses) == 0) && (!has(self.access.projectAccesses) || size(self.access.projectAccesses) == 0))",message="the Admin and Owner roles cannot be combined with namespace or project access",fieldPath=".access"
type UserSpec struct {
	// Email is the email address of the User. Temporal Cloud sends the
	// invitation to it. It must be lowercase, because Temporal Cloud stores
	// emails lowercased, and cannot be changed after creation.
	// Maps to Temporal Cloud proto `UserSpec.email`.
	// +required
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=254
	// +kubebuilder:validation:Format=email
	// +kubebuilder:validation:XValidation:rule="self == self.lowerAscii()",message="email must be lowercase"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="email cannot be changed after creation"
	Email string `json:"email"`
	// Access is the access assigned to the User. It must include an account
	// role.
	// Maps to Temporal Cloud proto `UserSpec.access`.
	// +required
	Access AccountScopedAccess `json:"access"`
}

// FromProto returns a UserSpec from the supplied Temporal Cloud UserSpec
// proto message.
func (s UserSpec) FromProto(in *tcloudidentityv1.UserSpec) UserSpec {
	return UserSpec{
		Email:  in.GetEmail(),
		Access: *accountScopedAccessFromProto(in.GetAccess()),
	}
}

// ToProto returns the Temporal Cloud UserSpec for the spec, with each
// Namespace ref replaced by its ID from `refs`.
func (s UserSpec) ToProto(
	referrerNamespace string,
	refs UserReferences,
) *tcloudidentityv1.UserSpec {
	namespaceID := func(namespace cloudcommon.ResourceReferenceOrID) string {
		if namespace.Ref == nil {
			return namespace.ID
		}
		id, _ := refs.NamespaceID(*namespace.Ref, referrerNamespace)
		return id
	}

	return &tcloudidentityv1.UserSpec{
		Email:  s.Email,
		Access: s.Access.toProto(namespaceID),
	}
}

// UserStatus describes the current status of a Temporal Cloud User.
// +k8s:openapi-gen=true
type UserStatus struct {
	tmprlcorev1.StatusBase `json:",inline"`
	// Resource contains common information about the Temporal Cloud resource
	// that this CR represents.
	// +optional
	Resource cloudcommon.ResourceStatus `json:"resource,omitzero"`
	// References contains the Temporal Cloud resource IDs of related resources.
	// +optional
	References UserReferences `json:"references,omitzero"`
	// Invitation is the open invitation sent to the User, if any. It is unset
	// once the User accepts it.
	// Maps to Temporal Cloud proto `User.invitation`.
	// +optional
	Invitation *UserInvitation `json:"invitation,omitempty"`
}

// UserReferences contains the Temporal Cloud IDs of the custom resources
// referenced by the spec.
type UserReferences struct {
	// Namespaces are the Namespace CRs referenced by the spec that have a
	// Temporal Cloud ID.
	// +optional
	// +listType=map
	// +listMapKey=namespace
	// +listMapKey=name
	Namespaces []NamespaceResolvedReference `json:"namespaces,omitempty"`
}

// NamespaceID returns the Temporal Cloud ID of the Namespace CR that the
// supplied reference points to, and whether it has been resolved.
func (r UserReferences) NamespaceID(
	ref cloudcommon.ResourceReference,
	referrerNamespace string,
) (string, bool) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = referrerNamespace
	}
	for _, resolved := range r.Namespaces {
		if resolved.Namespace == namespace && resolved.Name == ref.Name {
			return resolved.ID, true
		}
	}
	return "", false
}

// UserInvitation describes the invitation Temporal Cloud sent to a User.
// Maps to Temporal Cloud proto `Invitation`.
type UserInvitation struct {
	// CreatedTime is when the invitation was sent.
	// Maps to Temporal Cloud proto `Invitation.created_time`.
	// +optional
	CreatedTime *metav1.Time `json:"createdTime,omitempty"`
	// ExpiredTime is when the invitation expires or expired. Temporal Cloud
	// doesn't change the User when it passes.
	// Maps to Temporal Cloud proto `Invitation.expired_time`.
	// +optional
	ExpiredTime *metav1.Time `json:"expiredTime,omitempty"`
}

// userInvitationFromProto returns the invitation for the supplied Temporal
// Cloud Invitation, or nil if there is none.
func userInvitationFromProto(in *tcloudidentityv1.Invitation) *UserInvitation {
	if in == nil {
		return nil
	}
	return &UserInvitation{
		CreatedTime: transform.TimeFromProto(in.GetCreatedTime()),
		ExpiredTime: transform.TimeFromProto(in.GetExpiredTime()),
	}
}

// UserList is a list of User resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []User `json:"items"`
}
