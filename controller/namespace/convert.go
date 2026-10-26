package namespace

import (
	"maps"
	"slices"

	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

// namespaceSpecFromProto returns the [tcloudv1alpha1.NamespaceSpec] that
// represents the supplied Temporal Cloud Namespace.
//
// The whole Namespace is taken, rather than only its spec, because some CR
// spec fields (Project, Tags) live on the outer Namespace message.
func namespaceSpecFromProto(
	tns *tcloudnamespacev1.Namespace,
) tcloudv1alpha1.NamespaceSpec {
	spec := tns.GetSpec()
	return tcloudv1alpha1.NamespaceSpec{
		Project:              tns.GetProjectId(),
		Tags:                 maps.Clone(tns.GetTags()),
		Replicas:             replicasFromProto(spec.GetReplicas()),
		RetentionDays:        spec.GetRetentionDays(),
		MTLSAuth:             mtlsAuthFromProto(spec.GetMtlsAuth()),
		APIKeyAuth:           apiKeyAuthFromProto(spec.GetApiKeyAuth()),
		SearchAttributes:     searchAttributesFromProto(spec.GetSearchAttributes()),
		CodecServer:          codecServerFromProto(spec.GetCodecServer()),
		Lifecycle:            lifecycleFromProto(spec.GetLifecycle()),
		HighAvailability:     highAvailabilityFromProto(spec.GetHighAvailability()),
		ConnectivityRuleIDs:  slices.Clone(spec.GetConnectivityRuleIds()),
		Capacity:             capacityFromProto(spec.GetCapacitySpec()),
		Fairness:             fairnessFromProto(spec.GetFairness()),
		EncryptionValidation: encryptionValidationFromProto(spec.GetEncryptionValidation()),
		Description:          spec.GetDescription(),
	}
}

// resourceStatusFromProto returns the [cloudcommon.ResourceStatus] that
// represents the supplied Temporal Cloud Namespace.
func resourceStatusFromProto(
	tns *tcloudnamespacev1.Namespace,
) cloudcommon.ResourceStatus {
	return cloudcommon.ResourceStatus{
		ID:               tns.GetNamespace(),
		ResourceVersion:  tns.GetResourceVersion(),
		State:            resourceStateFromProto(tns.GetState()),
		AsyncOperationID: tns.GetAsyncOperationId(),
		CreatedTime:      timeFromProto(tns.GetCreatedTime()),
		LastModifiedTime: timeFromProto(tns.GetLastModifiedTime()),
	}
}

func replicasFromProto(
	in []*tcloudnamespacev1.ReplicaSpec,
) []tcloudv1alpha1.ReplicaSpec {
	if len(in) == 0 {
		return nil
	}
	out := make([]tcloudv1alpha1.ReplicaSpec, 0, len(in))
	for _, r := range in {
		out = append(out, tcloudv1alpha1.ReplicaSpec{Region: r.GetRegion()})
	}
	return out
}

func mtlsAuthFromProto(
	in *tcloudnamespacev1.MtlsAuthSpec,
) *tcloudv1alpha1.MTLSAuthSpec {
	if in == nil {
		return nil
	}
	out := &tcloudv1alpha1.MTLSAuthSpec{
		Enabled: in.GetEnabled(),
		// TODO: confirm Temporal Cloud returns raw PEM here, not base64.
		AcceptedClientCA: string(in.GetAcceptedClientCa()),
	}
	for _, f := range in.GetCertificateFilters() {
		out.CertificateFilters = append(
			out.CertificateFilters,
			tcloudv1alpha1.CertificateFilterSpec{
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
) *tcloudv1alpha1.APIKeyAuthSpec {
	if in == nil {
		return nil
	}
	return &tcloudv1alpha1.APIKeyAuthSpec{Enabled: in.GetEnabled()}
}

func codecServerFromProto(
	in *tcloudnamespacev1.CodecServerSpec,
) *tcloudv1alpha1.CodecServerSpec {
	if in == nil {
		return nil
	}
	out := &tcloudv1alpha1.CodecServerSpec{
		Endpoint:                      in.GetEndpoint(),
		PassAccessToken:               in.GetPassAccessToken(),
		IncludeCrossOriginCredentials: in.GetIncludeCrossOriginCredentials(),
	}
	if cem := in.GetCustomErrorMessage(); cem != nil {
		out.CustomErrorMessage = &tcloudv1alpha1.CodecServerCustomErrorMessage{}
		if def := cem.GetDefault(); def != nil {
			out.CustomErrorMessage.Default = &tcloudv1alpha1.CodecServerErrorMessage{
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
) *tcloudv1alpha1.HighAvailabilitySpec {
	if in == nil {
		return nil
	}
	return &tcloudv1alpha1.HighAvailabilitySpec{
		DisableManagedFailover:         in.GetDisableManagedFailover(),
		DisablePassivePollerForwarding: in.GetDisablePassivePollerForwarding(),
	}
}

func capacityFromProto(
	in *tcloudnamespacev1.CapacitySpec,
) *tcloudv1alpha1.CapacitySpec {
	switch {
	case in.GetOnDemand() != nil:
		return &tcloudv1alpha1.CapacitySpec{
			OnDemand: &tcloudv1alpha1.OnDemandCapacity{},
		}
	case in.GetProvisioned() != nil:
		return &tcloudv1alpha1.CapacitySpec{
			Provisioned: &tcloudv1alpha1.ProvisionedCapacity{
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
) *tcloudv1alpha1.FairnessSpec {
	if in == nil {
		return nil
	}
	return &tcloudv1alpha1.FairnessSpec{
		TaskQueueFairnessEnabled: in.GetTaskQueueFairnessEnabled(),
	}
}

func encryptionValidationFromProto(
	in *tcloudnamespacev1.EncryptionValidationSpec,
) *tcloudv1alpha1.EncryptionValidationSpec {
	if in == nil {
		return nil
	}
	return &tcloudv1alpha1.EncryptionValidationSpec{
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
) map[string]tcloudv1alpha1.SearchAttributeType {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]tcloudv1alpha1.SearchAttributeType, len(in))
	for name, t := range in {
		switch t {
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED:
			out[name] = ""
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT:
			out[name] = tcloudv1alpha1.SearchAttributeTypeText
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD:
			out[name] = tcloudv1alpha1.SearchAttributeTypeKeyword
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT:
			out[name] = tcloudv1alpha1.SearchAttributeTypeInt
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE:
			out[name] = tcloudv1alpha1.SearchAttributeTypeDouble
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL:
			out[name] = tcloudv1alpha1.SearchAttributeTypeBool
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME:
			out[name] = tcloudv1alpha1.SearchAttributeTypeDatetime
		case tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST:
			out[name] = tcloudv1alpha1.SearchAttributeTypeKeywordList
		default:
			out[name] = tcloudv1alpha1.SearchAttributeType(t.String())
		}
	}
	return out
}

// encryptionValidationModeFromProto converts an encryption validation mode to
// its CR name. Unknown modes keep their proto enum name.
func encryptionValidationModeFromProto(
	in tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode,
) tcloudv1alpha1.EncryptionValidationMode {
	switch in {
	case tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED:
		return ""
	case tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DISABLED:
		return tcloudv1alpha1.EncryptionValidationModeDisabled
	case tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN:
		return tcloudv1alpha1.EncryptionValidationModeWarn
	case tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DENY:
		return tcloudv1alpha1.EncryptionValidationModeDeny
	default:
		return tcloudv1alpha1.EncryptionValidationMode(in.String())
	}
}

// resourceStateFromProto converts a Temporal Cloud resource state to its CR
// name. Unspecified and unknown states map to "": the CR field is
// enum-validated, so an unrecognized value would fail the status patch.
func resourceStateFromProto(
	in tcloudresourcev1.ResourceState,
) cloudcommon.ResourceState {
	switch in {
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATING:
		return cloudcommon.ResourceStateActivating
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATION_FAILED:
		return cloudcommon.ResourceStateActivationFailed
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE:
		return cloudcommon.ResourceStateActive
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING:
		return cloudcommon.ResourceStateUpdating
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATE_FAILED:
		return cloudcommon.ResourceStateUpdateFailed
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETING:
		return cloudcommon.ResourceStateDeleting
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETE_FAILED:
		return cloudcommon.ResourceStateDeleteFailed
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETED:
		return cloudcommon.ResourceStateDeleted
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_SUSPENDED:
		return cloudcommon.ResourceStateSuspended
	case tcloudresourcev1.ResourceState_RESOURCE_STATE_EXPIRED:
		return cloudcommon.ResourceStateExpired
	default:
		return ""
	}
}

func timeFromProto(ts *timestamppb.Timestamp) *metav1.Time {
	if ts == nil {
		return nil
	}
	t := metav1.NewTime(ts.AsTime())
	return &t
}

// namespaceSpecToProto returns the Temporal Cloud NamespaceSpec that
// represents the supplied Namespace CR. The namespace name comes from the CR's
// metadata.name. Project and tags are not part of the Temporal Cloud
// NamespaceSpec and are sent separately.
func namespaceSpecToProto(
	cr *tcloudv1alpha1.Namespace,
) *tcloudnamespacev1.NamespaceSpec {
	spec := &cr.Spec
	return &tcloudnamespacev1.NamespaceSpec{
		Name:                 cr.Name,
		Replicas:             replicasToProto(spec.Replicas),
		RetentionDays:        spec.RetentionDays,
		MtlsAuth:             mtlsAuthToProto(spec.MTLSAuth),
		ApiKeyAuth:           apiKeyAuthToProto(spec.APIKeyAuth),
		SearchAttributes:     searchAttributesToProto(spec.SearchAttributes),
		CodecServer:          codecServerToProto(spec.CodecServer),
		Lifecycle:            lifecycleToProto(spec.Lifecycle),
		HighAvailability:     highAvailabilityToProto(spec.HighAvailability),
		ConnectivityRuleIds:  slices.Clone(spec.ConnectivityRuleIDs),
		CapacitySpec:         capacityToProto(spec.Capacity),
		Fairness:             fairnessToProto(spec.Fairness),
		EncryptionValidation: encryptionValidationToProto(spec.EncryptionValidation),
		Description:          spec.Description,
	}
}

func replicasToProto(
	in []tcloudv1alpha1.ReplicaSpec,
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
	in *tcloudv1alpha1.MTLSAuthSpec,
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
	in *tcloudv1alpha1.APIKeyAuthSpec,
) *tcloudnamespacev1.ApiKeyAuthSpec {
	if in == nil {
		return nil
	}
	return &tcloudnamespacev1.ApiKeyAuthSpec{Enabled: in.Enabled}
}

func codecServerToProto(
	in *tcloudv1alpha1.CodecServerSpec,
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
	in *tcloudv1alpha1.HighAvailabilitySpec,
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
	in *tcloudv1alpha1.CapacitySpec,
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
	in *tcloudv1alpha1.FairnessSpec,
) *tcloudnamespacev1.FairnessSpec {
	if in == nil {
		return nil
	}
	return &tcloudnamespacev1.FairnessSpec{
		TaskQueueFairnessEnabled: in.TaskQueueFairnessEnabled,
	}
}

func encryptionValidationToProto(
	in *tcloudv1alpha1.EncryptionValidationSpec,
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
	in map[string]tcloudv1alpha1.SearchAttributeType,
) map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType, len(in))
	for name, t := range in {
		switch t {
		case tcloudv1alpha1.SearchAttributeTypeText:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT
		case tcloudv1alpha1.SearchAttributeTypeKeyword:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD
		case tcloudv1alpha1.SearchAttributeTypeInt:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT
		case tcloudv1alpha1.SearchAttributeTypeDouble:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE
		case tcloudv1alpha1.SearchAttributeTypeBool:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL
		case tcloudv1alpha1.SearchAttributeTypeDatetime:
			out[name] = tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME
		case tcloudv1alpha1.SearchAttributeTypeKeywordList:
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
	in tcloudv1alpha1.EncryptionValidationMode,
) tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode {
	switch in {
	case tcloudv1alpha1.EncryptionValidationModeDisabled:
		return tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DISABLED
	case tcloudv1alpha1.EncryptionValidationModeWarn:
		return tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN
	case tcloudv1alpha1.EncryptionValidationModeDeny:
		return tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DENY
	default:
		return tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED
	}
}
