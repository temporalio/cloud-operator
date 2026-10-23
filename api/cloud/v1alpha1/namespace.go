package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmprlcorev1 "github.com/temporalio/kube-temporal/api/core/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// Namespace represents an instance of a Temporal Cloud Namespace.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type Namespace struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Namespace.
	// +required
	Spec NamespaceSpec `json:"spec"`
	// +optional
	// Status describes the current status of a Namespace.
	Status NamespaceStatus `json:"status,omitempty"`
}

// NamespaceSpec describes the desired state of a Temporal Cloud Namespace.
//
// Ref: https://github.com/temporalio/cloud-api/blob/main/temporal/api/cloud/namespace/v1/message.proto
// +k8s:openapi-gen=true
// +kubebuilder:validation:XValidation:rule="has(self.project) == has(oldSelf.project) && (!has(self.project) || self.project == oldSelf.project)",message="project cannot be added, changed or removed after creation",fieldPath=".project"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.searchAttributes) || (has(self.searchAttributes) && oldSelf.searchAttributes.all(k, k in self.searchAttributes && self.searchAttributes[k] == oldSelf.searchAttributes[k]))",message="searchAttributes cannot be removed or have their type changed",fieldPath=".searchAttributes"
type NamespaceSpec struct {
	// Project is the identifier of the Temporal Cloud project that the Namespace
	// belongs to.
	// Maps to Temporal Cloud proto `Namespace.project_id`.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Project string `json:"project,omitempty"`
	// Tags are key-value pairs to attach to the Namespace.
	// Maps to Temporal Cloud proto `Namespace.tags`.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// Replicas are the regions where the Namespace is placed.
	// Maps to Temporal Cloud proto `NamespaceSpec.replicas`.
	// +required
	// +listType=map
	// +listMapKey=region
	// +kubebuilder:validation:MinItems=1
	Replicas []ReplicaSpec `json:"replicas"`
	// RetentionDays is the number of days that closed workflow histories are
	// kept.
	// Maps to Temporal Cloud proto `NamespaceSpec.retention_days`.
	// +required
	RetentionDays int32 `json:"retentionDays"`
	// MTLSAuth configures mTLS client authentication.
	// Maps to Temporal Cloud proto `NamespaceSpec.mtls_auth`.
	// +optional
	MTLSAuth *MTLSAuthSpec `json:"mtlsAuth,omitempty"`
	// APIKeyAuth configures API key client authentication.
	// Maps to Temporal Cloud proto `NamespaceSpec.api_key_auth`.
	// +optional
	APIKeyAuth *APIKeyAuthSpec `json:"apiKeyAuth,omitempty"`
	// SearchAttributes are the custom search attributes of the Namespace,
	// keyed by name.
	// Maps to Temporal Cloud proto `NamespaceSpec.search_attributes`.
	// +optional
	SearchAttributes map[string]SearchAttributeType `json:"searchAttributes,omitempty"`
	// CodecServer configures the codec server used by the Temporal Cloud UI.
	// Maps to Temporal Cloud proto `NamespaceSpec.codec_server`.
	// +optional
	CodecServer *CodecServerSpec `json:"codecServer,omitempty"`
	// Lifecycle configures lifecycle settings.
	// Maps to Temporal Cloud proto `NamespaceSpec.lifecycle`.
	// +optional
	Lifecycle *cloudcommon.LifecycleSpec `json:"lifecycle,omitempty"`
	// HighAvailability configures failover behavior.
	// Maps to Temporal Cloud proto `NamespaceSpec.high_availability`.
	// +optional
	HighAvailability *HighAvailabilitySpec `json:"highAvailability,omitempty"`
	// ConnectivityRuleIDs are the IDs of the connectivity rules to apply.
	// Maps to Temporal Cloud proto `NamespaceSpec.connectivity_rule_ids`.
	// +optional
	// +listType=set
	ConnectivityRuleIDs []string `json:"connectivityRuleIDs,omitempty"`
	// Capacity selects the capacity mode.
	// Maps to Temporal Cloud proto `NamespaceSpec.capacity_spec`.
	// +optional
	Capacity *CapacitySpec `json:"capacity,omitempty"`
	// Fairness configures task queue fairness.
	// Maps to Temporal Cloud proto `NamespaceSpec.fairness`.
	// +optional
	Fairness *FairnessSpec `json:"fairness,omitempty"`
	// EncryptionValidation configures payload encryption validation.
	// Maps to Temporal Cloud proto `NamespaceSpec.encryption_validation`.
	// +optional
	EncryptionValidation *EncryptionValidationSpec `json:"encryptionValidation,omitempty"`
	// Description is a human-readable description of the Namespace.
	// Maps to Temporal Cloud proto `NamespaceSpec.description`.
	// +optional
	Description string `json:"description,omitempty"`
}

// ReplicaSpec places a replica of the Namespace in a region.
type ReplicaSpec struct {
	// Region is the ID of the region to place the replica in.
	// Maps to Temporal Cloud proto `ReplicaSpec.region`.
	// +required
	Region string `json:"region"`
}

// MTLSAuthSpec configures mTLS client authentication for a Namespace.
type MTLSAuthSpec struct {
	// Enabled turns mTLS authentication on or off.
	// Maps to Temporal Cloud proto `MtlsAuthSpec.enabled`.
	// +required
	Enabled bool `json:"enabled"`
	// AcceptedClientCA is the PEM-encoded CA certificate, or chain, that
	// client certificates must be issued by.
	// Maps to Temporal Cloud proto `MtlsAuthSpec.accepted_client_ca`.
	// +optional
	AcceptedClientCA string `json:"acceptedClientCA,omitempty"`
	// CertificateFilters restrict client certificates by subject.
	// Maps to Temporal Cloud proto `MtlsAuthSpec.certificate_filters`.
	// +optional
	CertificateFilters []CertificateFilterSpec `json:"certificateFilters,omitempty"`
}

// CertificateFilterSpec matches client certificates by subject fields.
type CertificateFilterSpec struct {
	// CommonName is the certificate's common name.
	// Maps to Temporal Cloud proto `CertificateFilterSpec.common_name`.
	// +optional
	CommonName string `json:"commonName,omitempty"`
	// Organization is the certificate's organization.
	// Maps to Temporal Cloud proto `CertificateFilterSpec.organization`.
	// +optional
	Organization string `json:"organization,omitempty"`
	// OrganizationalUnit is the certificate's organizational unit.
	// Maps to Temporal Cloud proto `CertificateFilterSpec.organizational_unit`.
	// +optional
	OrganizationalUnit string `json:"organizationalUnit,omitempty"`
	// SubjectAlternativeName is the certificate's subject alternative name.
	// Maps to Temporal Cloud proto `CertificateFilterSpec.subject_alternative_name`.
	// +optional
	SubjectAlternativeName string `json:"subjectAlternativeName,omitempty"`
}

// APIKeyAuthSpec configures API key client authentication for a Namespace.
type APIKeyAuthSpec struct {
	// Enabled turns API key authentication on or off.
	// Maps to Temporal Cloud proto `ApiKeyAuthSpec.enabled`.
	// +required
	Enabled bool `json:"enabled"`
}

// SearchAttributeType is the type of a custom search attribute.
// +kubebuilder:validation:Enum=Text;Keyword;Int;Double;Bool;Datetime;KeywordList
type SearchAttributeType string

const (
	SearchAttributeTypeText        SearchAttributeType = "Text"
	SearchAttributeTypeKeyword     SearchAttributeType = "Keyword"
	SearchAttributeTypeInt         SearchAttributeType = "Int"
	SearchAttributeTypeDouble      SearchAttributeType = "Double"
	SearchAttributeTypeBool        SearchAttributeType = "Bool"
	SearchAttributeTypeDatetime    SearchAttributeType = "Datetime"
	SearchAttributeTypeKeywordList SearchAttributeType = "KeywordList"
)

// CodecServerSpec configures the codec server used by the Temporal Cloud UI.
type CodecServerSpec struct {
	// Endpoint is the URL of the codec server.
	// Maps to Temporal Cloud proto `CodecServerSpec.endpoint`.
	// +required
	Endpoint string `json:"endpoint"`
	// PassAccessToken passes the user's access token to the endpoint.
	// Maps to Temporal Cloud proto `CodecServerSpec.pass_access_token`.
	// +optional
	PassAccessToken bool `json:"passAccessToken,omitempty"`
	// IncludeCrossOriginCredentials includes cross-origin credentials in
	// requests to the endpoint.
	// Maps to Temporal Cloud proto `CodecServerSpec.include_cross_origin_credentials`.
	// +optional
	IncludeCrossOriginCredentials bool `json:"includeCrossOriginCredentials,omitempty"`
	// CustomErrorMessage is shown in the UI when the codec server returns an
	// error.
	// Maps to Temporal Cloud proto `CodecServerSpec.custom_error_message`.
	// +optional
	CustomErrorMessage *CodecServerCustomErrorMessage `json:"customErrorMessage,omitempty"`
}

// CodecServerCustomErrorMessage configures the messages shown for codec
// server errors.
type CodecServerCustomErrorMessage struct {
	// Default is shown for any codec server error.
	// Maps to Temporal Cloud proto `CodecServerSpec.CustomErrorMessage.default`.
	// +optional
	Default *CodecServerErrorMessage `json:"default,omitempty"`
}

// CodecServerErrorMessage is a message shown for codec server errors.
type CodecServerErrorMessage struct {
	// Message is the text to show.
	// Maps to Temporal Cloud proto `CodecServerSpec.CustomErrorMessage.ErrorMessage.message`.
	// +optional
	Message string `json:"message,omitempty"`
	// Link is a URL shown next to the message.
	// Maps to Temporal Cloud proto `CodecServerSpec.CustomErrorMessage.ErrorMessage.link`.
	// +optional
	Link string `json:"link,omitempty"`
}

// HighAvailabilitySpec configures failover behavior for a Namespace.
type HighAvailabilitySpec struct {
	// DisableManagedFailover turns off failover managed by Temporal Cloud.
	// Maps to Temporal Cloud proto `HighAvailabilitySpec.disable_managed_failover`.
	// +optional
	DisableManagedFailover bool `json:"disableManagedFailover,omitempty"`
	// DisablePassivePollerForwarding turns off forwarding of polls from
	// passive replicas to the active replica.
	// Maps to Temporal Cloud proto `HighAvailabilitySpec.disable_passive_poller_forwarding`.
	// +optional
	DisablePassivePollerForwarding bool `json:"disablePassivePollerForwarding,omitempty"`
}

// CapacitySpec selects the capacity mode of a Namespace.
// +kubebuilder:validation:XValidation:rule="has(self.onDemand) != has(self.provisioned)",message="exactly one of onDemand or provisioned must be set"
type CapacitySpec struct {
	// OnDemand selects the on-demand capacity mode.
	// Maps to Temporal Cloud proto `CapacitySpec.on_demand`.
	// +optional
	OnDemand *OnDemandCapacity `json:"onDemand,omitempty"`
	// Provisioned selects the provisioned capacity mode.
	// Maps to Temporal Cloud proto `CapacitySpec.provisioned`.
	// +optional
	Provisioned *ProvisionedCapacity `json:"provisioned,omitempty"`
}

// OnDemandCapacity selects the on-demand capacity mode.
type OnDemandCapacity struct{}

// ProvisionedCapacity selects the provisioned capacity mode.
type ProvisionedCapacity struct {
	// Value is the capacity in Temporal Resource Units (TRUs).
	// Maps to Temporal Cloud proto `CapacitySpec.Provisioned.value`.
	// +required
	Value int32 `json:"value"`
}

// FairnessSpec configures task queue fairness for a Namespace.
type FairnessSpec struct {
	// TaskQueueFairnessEnabled turns on task queue fairness.
	// Maps to Temporal Cloud proto `FairnessSpec.task_queue_fairness_enabled`.
	// +optional
	TaskQueueFairnessEnabled bool `json:"taskQueueFairnessEnabled,omitempty"`
}

// EncryptionValidationSpec configures payload encryption validation for a
// Namespace.
type EncryptionValidationSpec struct {
	// Mode is the payload encryption validation mode.
	// Maps to Temporal Cloud proto `EncryptionValidationSpec.mode`.
	// +required
	Mode EncryptionValidationMode `json:"mode"`
	// MetadataKey is the payload metadata key that identifies encrypted
	// payloads.
	// Maps to Temporal Cloud proto `EncryptionValidationSpec.metadata_key`.
	// +optional
	MetadataKey string `json:"metadataKey,omitempty"`
	// MetadataValues are the payload metadata values that identify encrypted
	// payloads.
	// Maps to Temporal Cloud proto `EncryptionValidationSpec.metadata_values`.
	// +optional
	MetadataValues []string `json:"metadataValues,omitempty"`
	// InspectHeader includes workflow headers in validation.
	// Maps to Temporal Cloud proto `EncryptionValidationSpec.inspect_header`.
	// +optional
	InspectHeader bool `json:"inspectHeader,omitempty"`
	// InspectFailure includes failure messages and stack traces in
	// validation.
	// Maps to Temporal Cloud proto `EncryptionValidationSpec.inspect_failure`.
	// +optional
	InspectFailure bool `json:"inspectFailure,omitempty"`
}

// EncryptionValidationMode is the payload encryption validation mode.
// +kubebuilder:validation:Enum=Disabled;Warn;Deny
type EncryptionValidationMode string

const (
	// EncryptionValidationModeDisabled turns validation off.
	EncryptionValidationModeDisabled EncryptionValidationMode = "Disabled"
	// EncryptionValidationModeWarn allows unencrypted payloads and emits
	// warning metrics.
	EncryptionValidationModeWarn EncryptionValidationMode = "Warn"
	// EncryptionValidationModeDeny rejects unencrypted payloads.
	EncryptionValidationModeDeny EncryptionValidationMode = "Deny"
)

// NamespaceStatus describes the current status of a Temporal Cloud namespace.
// +k8s:openapi-gen=true
type NamespaceStatus struct {
	tmprlcorev1.StatusBase     `json:",inline"`
	cloudcommon.ResourceStatus `json:",inline"`
}

// NamespaceList is a list of Namespace resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Namespace `json:"items"`
}
