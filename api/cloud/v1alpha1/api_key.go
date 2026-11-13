package v1alpha1

import (
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmprlcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	"github.com/temporalio/kube-temporal/pkg/kube/transform"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// APIKey represents an instance of a Temporal Cloud API Key.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.resource.id`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.resource.state`
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Expiry",type=date,JSONPath=`.spec.expiryTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type APIKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the API Key.
	// +required
	Spec APIKeySpec `json:"spec"`
	// Status describes the current status of the API Key.
	// +optional
	Status APIKeyStatus `json:"status,omitempty"`
}

// FromProto returns an APIKey from the supplied Temporal Cloud ApiKey proto
// message.
func (k *APIKey) FromProto(in *tcloudidentityv1.ApiKey) *APIKey {
	out := APIKey{
		TypeMeta:   k.TypeMeta,
		ObjectMeta: k.ObjectMeta,
	}
	out.Status.References = *k.Status.References.DeepCopy()

	if in == nil {
		out.Spec.Token = k.Spec.Token.DeepCopy()
		return &out
	}

	var spec APIKeySpec
	out.Spec = spec.FromProto(in.GetSpec())
	out.Spec.Token = k.Spec.Token.DeepCopy()

	out.Status.Resource.ID = in.GetId()
	out.Status.Resource.Version = in.GetResourceVersion()
	out.Status.Resource.State.FromProto(in.GetState())
	out.Status.Resource.AsyncOperationID = in.GetAsyncOperationId()
	out.Status.Resource.CreatedTime = transform.TimeFromProto(in.GetCreatedTime())
	out.Status.Resource.LastModifiedTime = transform.TimeFromProto(in.GetLastModifiedTime())
	return &out
}

// APIKeySpec describes the desired state of a Temporal Cloud API Key.
//
// Ref: https://github.com/temporalio/cloud-api/blob/main/temporal/api/cloud/identity/v1/message.proto
// +k8s:openapi-gen=true
// +kubebuilder:validation:XValidation:rule="self.owner == oldSelf.owner",message="owner cannot be changed after creation",fieldPath=".owner"
// +kubebuilder:validation:XValidation:rule="has(self.token) == has(oldSelf.token) && (!has(self.token) || self.token == oldSelf.token)",message="token cannot be added, changed or removed after creation",fieldPath=".token"
type APIKeySpec struct {
	// DisplayName is the mutable name of the API Key in Temporal Cloud.
	// It does not have to be unique.
	// Maps to Temporal Cloud proto `ApiKeySpec.display_name`.
	// +required
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`
	// Description is a human readable description of the API Key.
	// Maps to Temporal Cloud proto `ApiKeySpec.description`.
	// +optional
	Description string `json:"description,omitempty"`
	// Owner is the identity the API Key authenticates as. It must be a Service
	// Account and cannot be changed after creation.
	// Maps to Temporal Cloud proto `ApiKeySpec.owner_id` and
	// `ApiKeySpec.owner_type`.
	// +required
	Owner APIKeyOwner `json:"owner"`
	// ExpiryTime is when the API Key stops working.
	// Maps to Temporal Cloud proto `ApiKeySpec.expiry_time`.
	// +required
	ExpiryTime metav1.Time `json:"expiryTime"`
	// Disabled turns the API Key off without deleting it.
	// Maps to Temporal Cloud proto `ApiKeySpec.disabled`.
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// TODO(tomba): describe what the operator does with the token location for
	// an adopted API Key, once adoption has been decided for API Keys.

	// Token is where the API Key's token is stored. When the operator creates
	// the API Key, it writes the token there. It cannot be changed after
	// creation.
	// +optional
	Token *APIKeyToken `json:"token,omitempty"`
}

// FromProto returns an APIKeySpec constructed from the supplied Temporal Cloud
// ApiKeySpec.
func (s APIKeySpec) FromProto(in *tcloudidentityv1.ApiKeySpec) APIKeySpec {
	if in == nil {
		return APIKeySpec{}
	}
	out := APIKeySpec{
		DisplayName: in.GetDisplayName(),
		Description: in.GetDescription(),
		Disabled:    in.GetDisabled(),
	}
	if t := transform.TimeFromProto(in.GetExpiryTime()); t != nil {
		out.ExpiryTime = *t
	}
	if in.GetOwnerType() == tcloudidentityv1.OwnerType_OWNER_TYPE_SERVICE_ACCOUNT {
		out.Owner.ServiceAccount = cloudcommon.ResourceReferenceOrID{ID: in.GetOwnerId()}
	}
	return out
}

// ToProto returns the Temporal Cloud ApiKeySpec for the spec. A Service
// Account ref takes its ID from `refs`.
func (s APIKeySpec) ToProto(
	referrerNamespace string,
	refs APIKeyReferences,
) *tcloudidentityv1.ApiKeySpec {
	ownerID := s.Owner.ServiceAccount.ID
	if ownerID == "" && s.Owner.ServiceAccount.Ref != nil {
		ownerID, _ = refs.ServiceAccountID(*s.Owner.ServiceAccount.Ref, referrerNamespace)
	}

	out := &tcloudidentityv1.ApiKeySpec{
		OwnerId:     ownerID,
		OwnerType:   tcloudidentityv1.OwnerType_OWNER_TYPE_SERVICE_ACCOUNT,
		DisplayName: s.DisplayName,
		Description: s.Description,
		Disabled:    s.Disabled,
	}
	if !s.ExpiryTime.IsZero() {
		out.ExpiryTime = timestamppb.New(s.ExpiryTime.Time)
	}
	return out
}

// APIKeyOwner is the identity that an API Key authenticates as.
type APIKeyOwner struct {
	// ServiceAccount is the Service Account that owns the API Key. A user
	// owner is not supported as Temporal Cloud only lets users create API Keys
	// for themselves. Since the operator  is designed to run under a service
	// account, it can only create API Keys owned by a Service Account.
	// +required
	ServiceAccount cloudcommon.ResourceReferenceOrID `json:"serviceAccount"`
}

// APIKeyToken is where an API Key's token is stored.
type APIKeyToken struct {
	// SecretKeyRef is the key of a Kubernetes Secret, in the same Kubernetes
	// Namespace as the API Key.
	// +required
	SecretKeyRef *SecretKeyRef `json:"secretKeyRef"`
}

// SecretKeyRef selects a key of a Kubernetes Secret.
type SecretKeyRef struct {
	// Name is the name of the Secret.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Key is the key in the Secret's data.
	// +optional
	// +kubebuilder:default=token
	Key string `json:"key,omitempty"`
}

// APIKeyStatus describes the current status of a Temporal Cloud API Key.
// +k8s:openapi-gen=true
type APIKeyStatus struct {
	tmprlcorev1.StatusBase `json:",inline"`
	// Resource contains common information about the Temporal Cloud resource
	// that this CR represents.
	// +optional
	Resource cloudcommon.ResourceStatus `json:"resource,omitempty"`
	// References contains the Temporal Cloud IDs of the custom resources
	// referenced by the spec.
	// +optional
	References APIKeyReferences `json:"references,omitempty"`
}

// APIKeyReferences contains the Temporal Cloud IDs of the custom resources
// referenced by an API Key's spec.
type APIKeyReferences struct {
	// ServiceAccount is the owning Service Account CR with its Temporal Cloud
	// ID. It isn't set until that Service Account has an ID.
	// +optional
	ServiceAccount *ServiceAccountResolvedReference `json:"serviceAccount,omitempty"`
}

// ServiceAccountID returns the Temporal Cloud ID of the Service Account CR
// that the supplied reference points to, and whether it has been resolved.
func (r APIKeyReferences) ServiceAccountID(
	ref cloudcommon.ResourceReference,
	referrerNamespace string,
) (string, bool) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = referrerNamespace
	}
	resolved := r.ServiceAccount
	if resolved == nil || resolved.Namespace != namespace || resolved.Name != ref.Name {
		return "", false
	}
	return resolved.ID, true
}

// ServiceAccountResolvedReference contains a Kubernetes Namespace and Name for
// a Temporal Cloud Service Account CR along with its resolved Temporal Cloud
// resource identifier.
type ServiceAccountResolvedReference struct {
	// Namespace is the Kubernetes Namespace containing the Service Account CR.
	// +required
	Namespace string `json:"namespace"`
	// Name is the Kubernetes Name of the Service Account CR.
	// +required
	Name string `json:"name"`
	// ID is the Temporal Cloud resource identifier.
	// +required
	ID string `json:"id"`
}

// APIKeyList is a list of APIKey resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIKeyList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []APIKey `json:"items"`
}
