package v1alpha1

import (
	"maps"
	"slices"

	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmprlcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	"github.com/temporalio/kube-temporal/pkg/kube/transform"

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

// FromProto sets the spec and status from the supplied Temporal Cloud
// Namespace. Project, tags and the status fields live on the outer Namespace
// message rather than its spec, so they are set here. Object metadata is left
// unchanged.
func (n *Namespace) FromProto(in *tcloudnamespacev1.Namespace) {
	n.Spec.FromProto(in.GetSpec())
	n.Spec.Project = in.GetProjectId()
	n.Spec.Tags = maps.Clone(in.GetTags())

	n.Status.ID = in.GetNamespace()
	n.Status.ResourceVersion = in.GetResourceVersion()
	n.Status.State.FromProto(in.GetState())
	n.Status.AsyncOperationID = in.GetAsyncOperationId()
	n.Status.CreatedTime = transform.TimeFromProto(in.GetCreatedTime())
	n.Status.LastModifiedTime = transform.TimeFromProto(in.GetLastModifiedTime())
}

// ToProto returns the Temporal Cloud Namespace that represents the spec. It
// sets only the fields we send to Temporal Cloud: the spec, project and tags.
// Requests take these as separate fields, so callers pick out the ones they
// need. Status is never sent, so it is left unset.
//
// The namespace name comes from the CR's metadata.name, because the CR has no
// spec.name of its own.
func (n Namespace) ToProto() *tcloudnamespacev1.Namespace {
	spec := n.Spec.toProto()
	spec.Name = n.Name
	return &tcloudnamespacev1.Namespace{
		Spec:      spec,
		ProjectId: n.Spec.Project,
		Tags:      maps.Clone(n.Spec.Tags),
	}
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
	// TODO(Mechack 10/25/2026): Ask #crew K8s how we should handle delete when this field is not set
	// Should the default depend on whether the operator created or adopted the Namespace
	// (for example, Retain for adopted, Delete for created), and should an explicit Delete ever be
	// honored for an adopted Namespace?

	// DeletionPolicy controls what happens to the Temporal Cloud Namespace
	// when this resource is deleted. Retain leaves it in Temporal Cloud.
	// Delete permanently deletes it.
	// +optional
	DeletionPolicy cloudcommon.DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// FromProto sets the spec from the supplied Temporal Cloud NamespaceSpec.
// Project and tags are not part of the Temporal Cloud NamespaceSpec, so they
// are left unchanged; [Namespace.FromProto] sets them.
func (s *NamespaceSpec) FromProto(in *tcloudnamespacev1.NamespaceSpec) {
	s.Replicas = replicasFromProto(in.GetReplicas())
	s.RetentionDays = in.GetRetentionDays()
	s.MTLSAuth = mtlsAuthFromProto(in.GetMtlsAuth())
	s.APIKeyAuth = apiKeyAuthFromProto(in.GetApiKeyAuth())
	s.SearchAttributes = searchAttributesFromProto(in.GetSearchAttributes())
	s.CodecServer = codecServerFromProto(in.GetCodecServer())
	s.Lifecycle = lifecycleFromProto(in.GetLifecycle())
	s.HighAvailability = highAvailabilityFromProto(in.GetHighAvailability())
	s.ConnectivityRuleIDs = slices.Clone(in.GetConnectivityRuleIds())
	s.Capacity = capacityFromProto(in.GetCapacitySpec())
	s.Fairness = fairnessFromProto(in.GetFairness())
	s.EncryptionValidation = encryptionValidationFromProto(in.GetEncryptionValidation())
	s.Description = in.GetDescription()
}

// toProto returns the Temporal Cloud NamespaceSpec that represents the spec,
// without the name. Project and tags are not part of the Temporal Cloud
// NamespaceSpec and are sent separately. Use [Namespace.ToProto], which also
// sets the name.
func (s NamespaceSpec) toProto() *tcloudnamespacev1.NamespaceSpec {
	return &tcloudnamespacev1.NamespaceSpec{
		Replicas:             replicasToProto(s.Replicas),
		RetentionDays:        s.RetentionDays,
		MtlsAuth:             mtlsAuthToProto(s.MTLSAuth),
		ApiKeyAuth:           apiKeyAuthToProto(s.APIKeyAuth),
		SearchAttributes:     searchAttributesToProto(s.SearchAttributes),
		CodecServer:          codecServerToProto(s.CodecServer),
		Lifecycle:            lifecycleToProto(s.Lifecycle),
		HighAvailability:     highAvailabilityToProto(s.HighAvailability),
		ConnectivityRuleIds:  slices.Clone(s.ConnectivityRuleIDs),
		CapacitySpec:         capacityToProto(s.Capacity),
		Fairness:             fairnessToProto(s.Fairness),
		EncryptionValidation: encryptionValidationToProto(s.EncryptionValidation),
		Description:          s.Description,
	}
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
// This struct does not contain a FromProto or ToProto function because there is
// no corresponding NamespaceStatus struct in the Temporal Cloud API.
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

func replicasFromProto(
	in []*tcloudnamespacev1.ReplicaSpec,
) []ReplicaSpec {
	if len(in) == 0 {
		return nil
	}
	out := make([]ReplicaSpec, 0, len(in))
	for _, r := range in {
		out = append(out, ReplicaSpec{Region: r.GetRegion()})
	}
	return out
}

func mtlsAuthFromProto(
	in *tcloudnamespacev1.MtlsAuthSpec,
) *MTLSAuthSpec {
	if in == nil {
		return nil
	}
	out := &MTLSAuthSpec{
		Enabled: in.GetEnabled(),
		// TODO: confirm Temporal Cloud returns raw PEM here, not base64.
		AcceptedClientCA: string(in.GetAcceptedClientCa()),
	}
	for _, f := range in.GetCertificateFilters() {
		out.CertificateFilters = append(
			out.CertificateFilters,
			CertificateFilterSpec{
				CommonName:             f.GetCommonName(),
				Organization:           f.GetOrganization(),
				OrganizationalUnit:     f.GetOrganizationalUnit(),
				SubjectAlternativeName: f.GetSubjectAlternativeName(),
			},
		)
	}
	return out
}

func apiKeyAuthFromProto(
	in *tcloudnamespacev1.ApiKeyAuthSpec,
) *APIKeyAuthSpec {
	if in == nil {
		return nil
	}
	return &APIKeyAuthSpec{Enabled: in.GetEnabled()}
}

func codecServerFromProto(
	in *tcloudnamespacev1.CodecServerSpec,
) *CodecServerSpec {
	if in == nil {
		return nil
	}
	out := &CodecServerSpec{
		Endpoint:                      in.GetEndpoint(),
		PassAccessToken:               in.GetPassAccessToken(),
		IncludeCrossOriginCredentials: in.GetIncludeCrossOriginCredentials(),
	}
	if cem := in.GetCustomErrorMessage(); cem != nil {
		out.CustomErrorMessage = &CodecServerCustomErrorMessage{}
		if def := cem.GetDefault(); def != nil {
			out.CustomErrorMessage.Default = &CodecServerErrorMessage{
				Message: def.GetMessage(),
				Link:    def.GetLink(),
			}
		}
	}
	return out
}

func lifecycleFromProto(
	in *tcloudnamespacev1.LifecycleSpec,
) *cloudcommon.LifecycleSpec {
	if in == nil {
		return nil
	}
	return &cloudcommon.LifecycleSpec{
		EnableDeleteProtection: in.GetEnableDeleteProtection(),
	}
}

func highAvailabilityFromProto(
	in *tcloudnamespacev1.HighAvailabilitySpec,
) *HighAvailabilitySpec {
	if in == nil {
		return nil
	}
	return &HighAvailabilitySpec{
		DisableManagedFailover:         in.GetDisableManagedFailover(),
		DisablePassivePollerForwarding: in.GetDisablePassivePollerForwarding(),
	}
}

func capacityFromProto(
	in *tcloudnamespacev1.CapacitySpec,
) *CapacitySpec {
	switch {
	case in.GetOnDemand() != nil:
		return &CapacitySpec{
			OnDemand: &OnDemandCapacity{},
		}
	case in.GetProvisioned() != nil:
		return &CapacitySpec{
			Provisioned: &ProvisionedCapacity{
				// The proto carries TRUs as a float64; the CR uses whole TRUs.
				Value: int32(in.GetProvisioned().GetValue()),
			},
		}
	default:
		return nil
	}
}

func fairnessFromProto(
	in *tcloudnamespacev1.FairnessSpec,
) *FairnessSpec {
	if in == nil {
		return nil
	}
	return &FairnessSpec{
		TaskQueueFairnessEnabled: in.GetTaskQueueFairnessEnabled(),
	}
}

func encryptionValidationFromProto(
	in *tcloudnamespacev1.EncryptionValidationSpec,
) *EncryptionValidationSpec {
	if in == nil {
		return nil
	}
	return &EncryptionValidationSpec{
		Mode:           encryptionValidationModeFromProto(in.GetMode()),
		MetadataKey:    in.GetMetadataKey(),
		MetadataValues: slices.Clone(in.GetMetadataValues()),
		InspectHeader:  in.GetInspectHeader(),
		InspectFailure: in.GetInspectFailure(),
	}
}

// searchAttributesFromProto converts search attribute types to their CR
// names. Unspecified types map to "". Types this operator does not know about
// keep their proto enum name, so they surface as a spec difference instead of
// being silently dropped.
func searchAttributesFromProto(
	in map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType,
) map[string]SearchAttributeType {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]SearchAttributeType, len(in))
	for name, t := range in {
		switch t {
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED:
			out[name] = ""
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT:
			out[name] = SearchAttributeTypeText
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD:
			out[name] = SearchAttributeTypeKeyword
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT:
			out[name] = SearchAttributeTypeInt
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE:
			out[name] = SearchAttributeTypeDouble
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL:
			out[name] = SearchAttributeTypeBool
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME:
			out[name] = SearchAttributeTypeDatetime
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST:
			out[name] = SearchAttributeTypeKeywordList
		default:
			out[name] = SearchAttributeType(t.String())
		}
	}
	return out
}

// encryptionValidationModeFromProto converts an encryption validation mode to
// its CR name. Unknown modes keep their proto enum name.
func encryptionValidationModeFromProto(
	in tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode,
) EncryptionValidationMode {
	switch in {
	case tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED:
		return ""
	case tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DISABLED:
		return EncryptionValidationModeDisabled
	case tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN:
		return EncryptionValidationModeWarn
	case tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DENY:
		return EncryptionValidationModeDeny
	default:
		return EncryptionValidationMode(in.String())
	}
}

func replicasToProto(
	in []ReplicaSpec,
) []*tcloudnamespacev1.ReplicaSpec {
	if len(in) == 0 {
		return nil
	}
	out := make([]*tcloudnamespacev1.ReplicaSpec, 0, len(in))
	for _, r := range in {
		out = append(out, &tcloudnamespacev1.ReplicaSpec{Region: r.Region})
	}
	return out
}

func mtlsAuthToProto(
	in *MTLSAuthSpec,
) *tcloudnamespacev1.MtlsAuthSpec {
	if in == nil {
		return nil
	}
	out := &tcloudnamespacev1.MtlsAuthSpec{
		Enabled: in.Enabled,
		// TODO: confirm Temporal Cloud expects raw PEM here, not base64.
		AcceptedClientCa: []byte(in.AcceptedClientCA),
	}
	for _, f := range in.CertificateFilters {
		out.CertificateFilters = append(
			out.CertificateFilters,
			&tcloudnamespacev1.CertificateFilterSpec{
				CommonName:             f.CommonName,
				Organization:           f.Organization,
				OrganizationalUnit:     f.OrganizationalUnit,
				SubjectAlternativeName: f.SubjectAlternativeName,
			},
		)
	}
	return out
}

func apiKeyAuthToProto(
	in *APIKeyAuthSpec,
) *tcloudnamespacev1.ApiKeyAuthSpec {
	if in == nil {
		return nil
	}
	return &tcloudnamespacev1.ApiKeyAuthSpec{Enabled: in.Enabled}
}

func codecServerToProto(
	in *CodecServerSpec,
) *tcloudnamespacev1.CodecServerSpec {
	if in == nil {
		return nil
	}
	out := &tcloudnamespacev1.CodecServerSpec{
		Endpoint:                      in.Endpoint,
		PassAccessToken:               in.PassAccessToken,
		IncludeCrossOriginCredentials: in.IncludeCrossOriginCredentials,
	}
	if cem := in.CustomErrorMessage; cem != nil {
		out.CustomErrorMessage = &tcloudnamespacev1.CodecServerSpec_CustomErrorMessage{}
		if def := cem.Default; def != nil {
			out.CustomErrorMessage.Default = &tcloudnamespacev1.CodecServerSpec_CustomErrorMessage_ErrorMessage{
				Message: def.Message,
				Link:    def.Link,
			}
		}
	}
	return out
}

func lifecycleToProto(
	in *cloudcommon.LifecycleSpec,
) *tcloudnamespacev1.LifecycleSpec {
	if in == nil {
		return nil
	}
	return &tcloudnamespacev1.LifecycleSpec{
		EnableDeleteProtection: in.EnableDeleteProtection,
	}
}

func highAvailabilityToProto(
	in *HighAvailabilitySpec,
) *tcloudnamespacev1.HighAvailabilitySpec {
	if in == nil {
		return nil
	}
	return &tcloudnamespacev1.HighAvailabilitySpec{
		DisableManagedFailover:         in.DisableManagedFailover,
		DisablePassivePollerForwarding: in.DisablePassivePollerForwarding,
	}
}

func capacityToProto(
	in *CapacitySpec,
) *tcloudnamespacev1.CapacitySpec {
	switch {
	case in == nil:
		return nil
	case in.OnDemand != nil:
		return &tcloudnamespacev1.CapacitySpec{
			Spec: &tcloudnamespacev1.CapacitySpec_OnDemand_{
				OnDemand: &tcloudnamespacev1.CapacitySpec_OnDemand{},
			},
		}
	case in.Provisioned != nil:
		return &tcloudnamespacev1.CapacitySpec{
			Spec: &tcloudnamespacev1.CapacitySpec_Provisioned_{
				Provisioned: &tcloudnamespacev1.CapacitySpec_Provisioned{
					Value: float64(in.Provisioned.Value),
				},
			},
		}
	default:
		return nil
	}
}

func fairnessToProto(
	in *FairnessSpec,
) *tcloudnamespacev1.FairnessSpec {
	if in == nil {
		return nil
	}
	return &tcloudnamespacev1.FairnessSpec{
		TaskQueueFairnessEnabled: in.TaskQueueFairnessEnabled,
	}
}

func encryptionValidationToProto(
	in *EncryptionValidationSpec,
) *tcloudnamespacev1.EncryptionValidationSpec {
	if in == nil {
		return nil
	}
	return &tcloudnamespacev1.EncryptionValidationSpec{
		Mode:           encryptionValidationModeToProto(in.Mode),
		MetadataKey:    in.MetadataKey,
		MetadataValues: slices.Clone(in.MetadataValues),
		InspectHeader:  in.InspectHeader,
		InspectFailure: in.InspectFailure,
	}
}

// searchAttributesToProto converts search attribute types to their proto
// values. The CR field is enum-validated, so unknown types cannot occur; they
// map to unspecified.
func searchAttributesToProto(
	in map[string]SearchAttributeType,
) map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType, len(in))
	for name, t := range in {
		switch t {
		case SearchAttributeTypeText:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT
		case SearchAttributeTypeKeyword:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD
		case SearchAttributeTypeInt:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT
		case SearchAttributeTypeDouble:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE
		case SearchAttributeTypeBool:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL
		case SearchAttributeTypeDatetime:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME
		case SearchAttributeTypeKeywordList:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST
		default:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED
		}
	}
	return out
}

// encryptionValidationModeToProto converts an encryption validation mode to
// its proto value. The CR field is enum-validated, so unknown modes cannot
// occur; they map to unspecified.
func encryptionValidationModeToProto(
	in EncryptionValidationMode,
) tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode {
	switch in {
	case EncryptionValidationModeDisabled:
		return tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DISABLED
	case EncryptionValidationModeWarn:
		return tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN
	case EncryptionValidationModeDeny:
		return tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DENY
	default:
		return tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED
	}
}
