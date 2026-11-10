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
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.id`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Region",type=string,JSONPath=`.spec.replicas[0].region`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Namespace struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Namespace. It may be omitted only
	// when adopting an existing Temporal Cloud Namespace, i.e. when the
	// cloud.temporal.io/adoption-policy annotation is "adopt".
	// +optional
	Spec NamespaceSpec `json:"spec,omitzero"`
	// +optional
	// Status describes the current status of a Namespace.
	Status NamespaceStatus `json:"status,omitempty"`
}

// FromProto returns a Namespace from the supplied Temporal Cloud Namespace
// proto message.
//
// Project, tags and the status fields live on the outer Namespace message
// rather than its spec, so they are set here. Object metadata is left
// unchanged.
func (n Namespace) FromProto(in *tcloudnamespacev1.Namespace) *Namespace {
	out := Namespace{
		TypeMeta:   n.TypeMeta,
		ObjectMeta: n.ObjectMeta,
	}
	out.Spec.Project = n.Spec.Project
	out.Spec.Tags = maps.Clone(n.Spec.Tags)

	if in == nil {
		return &out
	}

	var spec NamespaceSpec
	out.Spec = spec.FromProto(in.GetSpec())

	// We use Status.Cloud.References.Project to save the project identifier
	// returned from Temporal Cloud. We *never* mutate Spec.Project once it has
	// been set.
	out.Spec.Project = n.Spec.Project

	// But we *do* update the Spec.Tags to contain the collection of tags
	// returned from Temporal Cloud.
	out.Spec.Tags = maps.Clone(in.GetTags())

	out.Status.Resource.ID = in.GetNamespace()
	out.Status.Resource.Version = in.GetResourceVersion()
	out.Status.Resource.State.FromProto(in.GetState())
	out.Status.Resource.AsyncOperationID = in.GetAsyncOperationId()
	out.Status.Resource.CreatedTime = transform.TimeFromProto(in.GetCreatedTime())
	out.Status.Resource.LastModifiedTime = transform.TimeFromProto(in.GetLastModifiedTime())

	out.Status.References.Project = in.GetProjectId()
	return &out
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
		ProjectId: n.Status.References.Project,
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
	// Project contains *either* a Temporal Cloud Project identifier string
	// *or* a ProjectReference that points to a cloud.temporal.io/Project.
	// +optional
	Project *cloudcommon.ResourceReferenceOrID `json:"project,omitempty"`
	// Tags are key-value pairs to attach to the Namespace.
	// Maps to Temporal Cloud proto `Namespace.tags`.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// Replicas are the regions where the Namespace is placed.
	// Maps to Temporal Cloud proto `NamespaceSpec.replicas`.
	// Required and is enforced by the temporal-cloud-namespace-validation admission policy.
	// +optional
	// +listType=map
	// +listMapKey=region
	// +kubebuilder:validation:MinItems=1
	Replicas []ReplicaSpec `json:"replicas,omitempty"`
	// RetentionDays is the number of days that closed workflow histories are
	// kept.
	// Maps to Temporal Cloud proto `NamespaceSpec.retention_days`.
	// Required and is enforced by the temporal-cloud-namespace-validation admission policy.
	// +optional
	RetentionDays int32 `json:"retentionDays,omitempty"`
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

// FromProto returns a NamespaceSpec constructed from the supplied Temporal
// Cloud NamespaceSpec.
func (s NamespaceSpec) FromProto(in *tcloudnamespacev1.NamespaceSpec) NamespaceSpec {
	if in == nil {
		return NamespaceSpec{}
	}

	out := NamespaceSpec{
		Description:         in.GetDescription(),
		Replicas:            ReplicasFromProto(in.GetReplicas()),
		RetentionDays:       in.GetRetentionDays(),
		SearchAttributes:    SearchAttributesFromProto(in.GetSearchAttributes()),
		Lifecycle:           NamespaceLifecycleFromProto(in.GetLifecycle()),
		ConnectivityRuleIDs: slices.Clone(in.GetConnectivityRuleIds()),
	}

	var mtlsAuthSpec MTLSAuthSpec
	out.MTLSAuth = mtlsAuthSpec.FromProto(in.GetMtlsAuth())

	var apiKeyAuthSpec APIKeyAuthSpec
	out.APIKeyAuth = apiKeyAuthSpec.FromProto(in.GetApiKeyAuth())

	var codecSpec CodecServerSpec
	out.CodecServer = codecSpec.FromProto(in.GetCodecServer())

	var haSpec HighAvailabilitySpec
	out.HighAvailability = haSpec.FromProto(in.GetHighAvailability())

	var capSpec CapacitySpec
	out.Capacity = capSpec.FromProto(in.GetCapacitySpec())

	var fairSpec FairnessSpec
	out.Fairness = fairSpec.FromProto(in.GetFairness())

	var encValSpec EncryptionValidationSpec
	out.EncryptionValidation = encValSpec.FromProto(in.GetEncryptionValidation())

	return out
}

// toProto returns the Temporal Cloud NamespaceSpec that represents the spec,
// without the name. Project and tags are not part of the Temporal Cloud
// NamespaceSpec and are sent separately. Use [Namespace.ToProto], which also
// sets the name.
func (s NamespaceSpec) toProto() *tcloudnamespacev1.NamespaceSpec {
	return &tcloudnamespacev1.NamespaceSpec{
		Replicas:             ReplicasToProto(s.Replicas),
		RetentionDays:        s.RetentionDays,
		MtlsAuth:             s.MTLSAuth.ToProto(),
		ApiKeyAuth:           s.APIKeyAuth.ToProto(),
		SearchAttributes:     SearchAttributesToProto(s.SearchAttributes),
		CodecServer:          s.CodecServer.ToProto(),
		Lifecycle:            NamespaceLifecycleToProto(s.Lifecycle),
		HighAvailability:     s.HighAvailability.ToProto(),
		ConnectivityRuleIds:  slices.Clone(s.ConnectivityRuleIDs),
		CapacitySpec:         s.Capacity.ToProto(),
		Fairness:             s.Fairness.ToProto(),
		EncryptionValidation: s.EncryptionValidation.ToProto(),
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

// FromProto sets the replica from the supplied Temporal Cloud ReplicaSpec.
func (r *ReplicaSpec) FromProto(in *tcloudnamespacev1.ReplicaSpec) {
	*r = ReplicaSpec{Region: in.GetRegion()}
}

// ToProto returns the Temporal Cloud ReplicaSpec that represents the replica.
// It returns nil when r is nil.
func (r *ReplicaSpec) ToProto() *tcloudnamespacev1.ReplicaSpec {
	if r == nil {
		return nil
	}
	return &tcloudnamespacev1.ReplicaSpec{Region: r.Region}
}

// ReplicasFromProto converts the supplied Temporal Cloud replicas to their CR
// form, returning nil when there are none.
//
// It is a function rather than a FromProto method because NamespaceSpec.Replicas
// is a plain []ReplicaSpec, and Go does not allow methods on unnamed slice
// types. Each replica is still converted with [ReplicaSpec.FromProto].
func ReplicasFromProto(
	in []*tcloudnamespacev1.ReplicaSpec,
) []ReplicaSpec {
	if len(in) == 0 {
		return nil
	}
	out := make([]ReplicaSpec, len(in))
	for i, r := range in {
		out[i].FromProto(r)
	}
	return out
}

// ReplicasToProto converts the supplied replicas to their Temporal Cloud form,
// returning nil when there are none.
//
// It is a function rather than a ToProto method because NamespaceSpec.Replicas
// is a plain []ReplicaSpec, and Go does not allow methods on unnamed slice
// types. Each replica is still converted with [ReplicaSpec.ToProto].
func ReplicasToProto(
	in []ReplicaSpec,
) []*tcloudnamespacev1.ReplicaSpec {
	if len(in) == 0 {
		return nil
	}
	out := make([]*tcloudnamespacev1.ReplicaSpec, 0, len(in))
	for _, r := range in {
		out = append(out, r.ToProto())
	}
	return out
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

// FromProto returns an MTLSAuthSpec constructed from the supplied Temporal
// Cloud MtlsAuthSpec.
func (m MTLSAuthSpec) FromProto(in *tcloudnamespacev1.MtlsAuthSpec) *MTLSAuthSpec {
	if in == nil {
		return nil
	}

	out := MTLSAuthSpec{
		Enabled: in.GetEnabled(),
		// TODO: confirm Temporal Cloud returns raw PEM here, not base64.
		AcceptedClientCA: string(in.GetAcceptedClientCa()),
	}
	for _, f := range in.GetCertificateFilters() {
		var filter CertificateFilterSpec
		filter.FromProto(f)
		out.CertificateFilters = append(out.CertificateFilters, filter)
	}
	return &out
}

// ToProto returns the Temporal Cloud MtlsAuthSpec that represents the spec. It
// returns nil when m is nil, so it can be called on an unset optional field.
func (m *MTLSAuthSpec) ToProto() *tcloudnamespacev1.MtlsAuthSpec {
	if m == nil {
		return nil
	}
	out := &tcloudnamespacev1.MtlsAuthSpec{
		Enabled: m.Enabled,
		// TODO: confirm Temporal Cloud expects raw PEM here, not base64.
		AcceptedClientCa: []byte(m.AcceptedClientCA),
	}
	for _, f := range m.CertificateFilters {
		out.CertificateFilters = append(out.CertificateFilters, f.ToProto())
	}
	return out
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

// FromProto sets the filter from the supplied Temporal Cloud
// CertificateFilterSpec.
func (f *CertificateFilterSpec) FromProto(in *tcloudnamespacev1.CertificateFilterSpec) {
	*f = CertificateFilterSpec{
		CommonName:             in.GetCommonName(),
		Organization:           in.GetOrganization(),
		OrganizationalUnit:     in.GetOrganizationalUnit(),
		SubjectAlternativeName: in.GetSubjectAlternativeName(),
	}
}

// ToProto returns the Temporal Cloud CertificateFilterSpec that represents the
// filter. It returns nil when f is nil.
func (f *CertificateFilterSpec) ToProto() *tcloudnamespacev1.CertificateFilterSpec {
	if f == nil {
		return nil
	}
	return &tcloudnamespacev1.CertificateFilterSpec{
		CommonName:             f.CommonName,
		Organization:           f.Organization,
		OrganizationalUnit:     f.OrganizationalUnit,
		SubjectAlternativeName: f.SubjectAlternativeName,
	}
}

// APIKeyAuthSpec configures API key client authentication for a Namespace.
type APIKeyAuthSpec struct {
	// Enabled turns API key authentication on or off.
	// Maps to Temporal Cloud proto `ApiKeyAuthSpec.enabled`.
	// +required
	Enabled bool `json:"enabled"`
}

// FromProto returns an APIKeyAuthSpec from the supplied Temporal Cloud ApiKeyAuthSpec.
func (a APIKeyAuthSpec) FromProto(in *tcloudnamespacev1.ApiKeyAuthSpec) *APIKeyAuthSpec {
	if in == nil {
		return nil
	}

	return &APIKeyAuthSpec{
		Enabled: in.GetEnabled(),
	}
}

// ToProto returns the Temporal Cloud ApiKeyAuthSpec that represents the spec.
// It returns nil when a is nil, so it can be called on an unset optional
// field.
func (a *APIKeyAuthSpec) ToProto() *tcloudnamespacev1.ApiKeyAuthSpec {
	if a == nil {
		return nil
	}
	return &tcloudnamespacev1.ApiKeyAuthSpec{Enabled: a.Enabled}
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

// FromProto sets the type from the supplied Temporal Cloud search attribute
// type. Unspecified types map to "". Types this operator does not know about
// keep their proto enum name, so they surface as a spec difference instead of
// being silently dropped.
func (t *SearchAttributeType) FromProto(in tcloudnamespacev1.NamespaceSpec_SearchAttributeType) {
	switch in {
	case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED:
		*t = ""
	case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT:
		*t = SearchAttributeTypeText
	case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD:
		*t = SearchAttributeTypeKeyword
	case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT:
		*t = SearchAttributeTypeInt
	case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE:
		*t = SearchAttributeTypeDouble
	case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL:
		*t = SearchAttributeTypeBool
	case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME:
		*t = SearchAttributeTypeDatetime
	case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST:
		*t = SearchAttributeTypeKeywordList
	default:
		*t = SearchAttributeType(in.String())
	}
}

// ToProto returns the Temporal Cloud search attribute type that represents the
// type. The CR field is enum-validated, so unknown types cannot occur; they
// map to unspecified.
func (t SearchAttributeType) ToProto() tcloudnamespacev1.NamespaceSpec_SearchAttributeType {
	switch t {
	case SearchAttributeTypeText:
		return tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT
	case SearchAttributeTypeKeyword:
		return tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD
	case SearchAttributeTypeInt:
		return tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT
	case SearchAttributeTypeDouble:
		return tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE
	case SearchAttributeTypeBool:
		return tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL
	case SearchAttributeTypeDatetime:
		return tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME
	case SearchAttributeTypeKeywordList:
		return tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST
	default:
		return tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED
	}
}

// SearchAttributesFromProto converts the supplied Temporal Cloud search
// attribute types, keyed by name, to their CR form, returning nil when there
// are none.
//
// It is a function rather than a FromProto method because
// NamespaceSpec.SearchAttributes is a plain map[string]SearchAttributeType, and
// Go does not allow methods on unnamed map types. Each type is still converted
// with [SearchAttributeType.FromProto].
func SearchAttributesFromProto(
	in map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType,
) map[string]SearchAttributeType {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]SearchAttributeType, len(in))
	for name, t := range in {
		var st SearchAttributeType
		st.FromProto(t)
		out[name] = st
	}
	return out
}

// SearchAttributesToProto converts the supplied search attribute types, keyed
// by name, to their Temporal Cloud form, returning nil when there are none.
//
// It is a function rather than a ToProto method because
// NamespaceSpec.SearchAttributes is a plain map[string]SearchAttributeType, and
// Go does not allow methods on unnamed map types. Each type is still converted
// with [SearchAttributeType.ToProto].
func SearchAttributesToProto(
	in map[string]SearchAttributeType,
) map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType, len(in))
	for name, t := range in {
		out[name] = t.ToProto()
	}
	return out
}

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

// FromProto returns a CodecServerSpec from the supplied Temporal Cloud
// CodecServerSpec.
func (c CodecServerSpec) FromProto(in *tcloudnamespacev1.CodecServerSpec) *CodecServerSpec {
	if in == nil {
		return nil
	}
	out := CodecServerSpec{
		Endpoint:                      in.GetEndpoint(),
		PassAccessToken:               in.GetPassAccessToken(),
		IncludeCrossOriginCredentials: in.GetIncludeCrossOriginCredentials(),
	}

	var cem CodecServerCustomErrorMessage
	out.CustomErrorMessage = cem.FromProto(in.GetCustomErrorMessage())

	return &out
}

// ToProto returns the Temporal Cloud CodecServerSpec that represents the spec.
// It returns nil when c is nil, so it can be called on an unset optional
// field.
func (c *CodecServerSpec) ToProto() *tcloudnamespacev1.CodecServerSpec {
	if c == nil {
		return nil
	}
	return &tcloudnamespacev1.CodecServerSpec{
		Endpoint:                      c.Endpoint,
		PassAccessToken:               c.PassAccessToken,
		IncludeCrossOriginCredentials: c.IncludeCrossOriginCredentials,
		CustomErrorMessage:            c.CustomErrorMessage.ToProto(),
	}
}

// CodecServerCustomErrorMessage configures the messages shown for codec
// server errors.
type CodecServerCustomErrorMessage struct {
	// Default is shown for any codec server error.
	// Maps to Temporal Cloud proto `CodecServerSpec.CustomErrorMessage.default`.
	// +optional
	Default *CodecServerErrorMessage `json:"default,omitempty"`
}

// FromProto returns a CodecServerCustomErrorMessage from the supplied Temporal
// Cloud CodecServerSpec_CustomErrorMessage.
func (c CodecServerCustomErrorMessage) FromProto(
	in *tcloudnamespacev1.CodecServerSpec_CustomErrorMessage,
) *CodecServerCustomErrorMessage {
	if in == nil {
		return nil
	}

	out := CodecServerCustomErrorMessage{}

	var def CodecServerErrorMessage
	out.Default = def.FromProto(in.GetDefault())

	return &out
}

// ToProto returns the Temporal Cloud CodecServerSpec_CustomErrorMessage that
// represents the custom error message. It returns nil when c is nil, so it can
// be called on an unset optional field.
func (c *CodecServerCustomErrorMessage) ToProto() *tcloudnamespacev1.CodecServerSpec_CustomErrorMessage {
	if c == nil {
		return nil
	}
	return &tcloudnamespacev1.CodecServerSpec_CustomErrorMessage{
		Default: c.Default.ToProto(),
	}
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

// FromProto returns a CodecServerErrorMessage constructed from the supplied
// Temporal Cloud CodecServerSpec_CustomErrorMessage_ErrorMessage.
func (e CodecServerErrorMessage) FromProto(
	in *tcloudnamespacev1.CodecServerSpec_CustomErrorMessage_ErrorMessage,
) *CodecServerErrorMessage {
	if in == nil {
		return nil
	}
	return &CodecServerErrorMessage{
		Message: in.GetMessage(),
		Link:    in.GetLink(),
	}
}

// ToProto returns the Temporal Cloud
// CodecServerSpec_CustomErrorMessage_ErrorMessage that represents the error
// message. It returns nil when e is nil, so it can be called on an unset
// optional field.
func (e *CodecServerErrorMessage) ToProto() *tcloudnamespacev1.CodecServerSpec_CustomErrorMessage_ErrorMessage {
	if e == nil {
		return nil
	}
	return &tcloudnamespacev1.CodecServerSpec_CustomErrorMessage_ErrorMessage{
		Message: e.Message,
		Link:    e.Link,
	}
}

// NamespaceLifecycleFromProto converts the supplied Temporal Cloud namespace
// LifecycleSpec to its CR form, returning nil when it is unset.
//
// It is a function rather than a FromProto method because
// [cloudcommon.LifecycleSpec] is shared across resources and lives in another
// package. Temporal Cloud has a separate LifecycleSpec proto per resource
// (namespace, project), and a method could only accept one of them, which
// would tie the shared type to namespaces.
func NamespaceLifecycleFromProto(
	in *tcloudnamespacev1.LifecycleSpec,
) *cloudcommon.LifecycleSpec {
	if in == nil {
		return nil
	}
	return &cloudcommon.LifecycleSpec{
		EnableDeleteProtection: in.GetEnableDeleteProtection(),
	}
}

// NamespaceLifecycleToProto converts the supplied lifecycle settings to a
// Temporal Cloud namespace LifecycleSpec, returning nil when in is nil.
//
// It is a function rather than a ToProto method because
// [cloudcommon.LifecycleSpec] is shared across resources and lives in another
// package. Temporal Cloud has a separate LifecycleSpec proto per resource
// (namespace, project), and a method could only return one of them, which
// would tie the shared type to namespaces.
func NamespaceLifecycleToProto(
	in *cloudcommon.LifecycleSpec,
) *tcloudnamespacev1.LifecycleSpec {
	if in == nil {
		return nil
	}
	return &tcloudnamespacev1.LifecycleSpec{
		EnableDeleteProtection: in.EnableDeleteProtection,
	}
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

// FromProto returns a HighAvailabilitySpec created from the supplied Temporal
// Cloud HighAvailabilitySpec.
func (h HighAvailabilitySpec) FromProto(in *tcloudnamespacev1.HighAvailabilitySpec) *HighAvailabilitySpec {
	if in == nil {
		return nil
	}
	return &HighAvailabilitySpec{
		DisableManagedFailover:         in.GetDisableManagedFailover(),
		DisablePassivePollerForwarding: in.GetDisablePassivePollerForwarding(),
	}
}

// ToProto returns the Temporal Cloud HighAvailabilitySpec that represents the
// spec. It returns nil when h is nil, so it can be called on an unset optional
// field.
func (h *HighAvailabilitySpec) ToProto() *tcloudnamespacev1.HighAvailabilitySpec {
	if h == nil {
		return nil
	}
	return &tcloudnamespacev1.HighAvailabilitySpec{
		DisableManagedFailover:         h.DisableManagedFailover,
		DisablePassivePollerForwarding: h.DisablePassivePollerForwarding,
	}
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

// FromProto returns a CapacitySpec created from the supplied Temporal Cloud
// CapacitySpec.
func (c CapacitySpec) FromProto(in *tcloudnamespacev1.CapacitySpec) *CapacitySpec {
	if in == nil {
		return nil
	}
	// A capacity spec with no mode set carries nothing the CR can represent, so
	// it is treated as unset.
	if in.GetOnDemand() == nil && in.GetProvisioned() == nil {
		return nil
	}
	out := CapacitySpec{}

	var odCap OnDemandCapacity
	out.OnDemand = odCap.FromProto(in.GetOnDemand())

	var provCap ProvisionedCapacity
	out.Provisioned = provCap.FromProto(in.GetProvisioned())

	return &out
}

// ToProto returns the Temporal Cloud CapacitySpec that represents the spec. It
// returns nil when c is nil or sets no capacity mode, so it can be called on an
// unset optional field.
func (c *CapacitySpec) ToProto() *tcloudnamespacev1.CapacitySpec {
	switch {
	case c == nil:
		return nil
	case c.OnDemand != nil:
		return &tcloudnamespacev1.CapacitySpec{
			Spec: &tcloudnamespacev1.CapacitySpec_OnDemand_{
				OnDemand: c.OnDemand.ToProto(),
			},
		}
	case c.Provisioned != nil:
		return &tcloudnamespacev1.CapacitySpec{
			Spec: &tcloudnamespacev1.CapacitySpec_Provisioned_{
				Provisioned: c.Provisioned.ToProto(),
			},
		}
	default:
		return nil
	}
}

// OnDemandCapacity selects the on-demand capacity mode.
type OnDemandCapacity struct{}

// FromProto returns an OnDemandCapacity from the supplied Temporal Cloud
// CapacitySpec_OnDemand.
func (o OnDemandCapacity) FromProto(in *tcloudnamespacev1.CapacitySpec_OnDemand) *OnDemandCapacity {
	if in == nil {
		return nil
	}
	return &OnDemandCapacity{}
}

// ToProto returns the Temporal Cloud CapacitySpec_OnDemand that represents the
// capacity. It returns nil when o is nil.
func (o *OnDemandCapacity) ToProto() *tcloudnamespacev1.CapacitySpec_OnDemand {
	if o == nil {
		return nil
	}
	return &tcloudnamespacev1.CapacitySpec_OnDemand{}
}

// ProvisionedCapacity selects the provisioned capacity mode.
type ProvisionedCapacity struct {
	// Value is the capacity in Temporal Resource Units (TRUs).
	// Maps to Temporal Cloud proto `CapacitySpec.Provisioned.value`.
	// +required
	Value int32 `json:"value"`
}

// FromProto returns the ProvisionedCapacity from the supplied Temporal Cloud
// CapacitySpec_Provisioned.
func (p ProvisionedCapacity) FromProto(in *tcloudnamespacev1.CapacitySpec_Provisioned) *ProvisionedCapacity {
	if in == nil {
		return nil
	}
	return &ProvisionedCapacity{
		// The proto carries TRUs as a float64; the CR uses whole TRUs.
		Value: int32(in.GetValue()),
	}
}

// ToProto returns the Temporal Cloud CapacitySpec_Provisioned that represents
// the capacity. It returns nil when p is nil.
func (p *ProvisionedCapacity) ToProto() *tcloudnamespacev1.CapacitySpec_Provisioned {
	if p == nil {
		return nil
	}
	return &tcloudnamespacev1.CapacitySpec_Provisioned{Value: float64(p.Value)}
}

// FairnessSpec configures task queue fairness for a Namespace.
type FairnessSpec struct {
	// TaskQueueFairnessEnabled turns on task queue fairness.
	// Maps to Temporal Cloud proto `FairnessSpec.task_queue_fairness_enabled`.
	// +optional
	TaskQueueFairnessEnabled bool `json:"taskQueueFairnessEnabled,omitempty"`
}

// FromProto returns a FairnessSpec from the supplied Temporal Cloud
// FairnessSpec.
func (f FairnessSpec) FromProto(in *tcloudnamespacev1.FairnessSpec) *FairnessSpec {
	if in == nil {
		return nil
	}
	return &FairnessSpec{
		TaskQueueFairnessEnabled: in.GetTaskQueueFairnessEnabled(),
	}
}

// ToProto returns the Temporal Cloud FairnessSpec that represents the spec. It
// returns nil when f is nil, so it can be called on an unset optional field.
func (f *FairnessSpec) ToProto() *tcloudnamespacev1.FairnessSpec {
	if f == nil {
		return nil
	}
	return &tcloudnamespacev1.FairnessSpec{
		TaskQueueFairnessEnabled: f.TaskQueueFairnessEnabled,
	}
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

// FromProto returns an EncryptionValidationSpec from the supplied Temporal
// Cloud EncryptionValidationSpec.
func (e EncryptionValidationSpec) FromProto(in *tcloudnamespacev1.EncryptionValidationSpec) *EncryptionValidationSpec {
	if in == nil {
		return nil
	}
	out := EncryptionValidationSpec{
		MetadataKey:    in.GetMetadataKey(),
		MetadataValues: slices.Clone(in.GetMetadataValues()),
		InspectHeader:  in.GetInspectHeader(),
		InspectFailure: in.GetInspectFailure(),
	}

	var mode EncryptionValidationMode
	out.Mode = mode.FromProto(in.GetMode())

	return &out
}

// ToProto returns the Temporal Cloud EncryptionValidationSpec that represents
// the spec. It returns nil when e is nil, so it can be called on an unset
// optional field.
func (e *EncryptionValidationSpec) ToProto() *tcloudnamespacev1.EncryptionValidationSpec {
	if e == nil {
		return nil
	}
	return &tcloudnamespacev1.EncryptionValidationSpec{
		Mode:           e.Mode.ToProto(),
		MetadataKey:    e.MetadataKey,
		MetadataValues: slices.Clone(e.MetadataValues),
		InspectHeader:  e.InspectHeader,
		InspectFailure: e.InspectFailure,
	}
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

// FromProto returns the EncryptionValidationMode from the supplied Temporal
// Cloud encryption validation mode.
func (m EncryptionValidationMode) FromProto(
	in tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode,
) EncryptionValidationMode {
	switch in {
	case tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED:
		return EncryptionValidationMode("")
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

// ToProto returns the Temporal Cloud encryption validation mode that
// represents the mode. The CR field is enum-validated, so unknown modes cannot
// occur; they map to unspecified.
func (m EncryptionValidationMode) ToProto() tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode {
	switch m {
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

// NamespaceStatus describes the current status of a Temporal Cloud namespace.
// This struct does not contain a FromProto or ToProto function because there is
// no corresponding NamespaceStatus struct in the Temporal Cloud API.
// +k8s:openapi-gen=true
type NamespaceStatus struct {
	tmprlcorev1.StatusBase `json:",inline"`
	// Resource contains common information about the Temporal Cloud resource
	// that this CR represents.
	Resource cloudcommon.ResourceStatus `json:"resource"`
	// References is an optional field containing the Temporal Cloud resource
	// IDs for any related resources.
	References NamespaceReferences `json:"references,omitempty"`
}

// NamespaceReferences is an optional field containing the
// Temporal Cloud resource IDs for any related resources.
type NamespaceReferences struct {
	// Project is the Temporal Cloud resource identifier for the Project
	// containing this Temporal Namespace.
	Project string `json:"project,omitempty"`
	// ConnectivityRules is the set of Temporal Cloud resource identifiers for
	// any Namespace ConnectivityRules associated with this Temporal Namespace
	ConnectivityRules []string `json:"connectivityRules,omitempty"`
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
