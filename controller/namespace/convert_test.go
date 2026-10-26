package namespace

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

// fullSpec and fullProtoNamespace describe the same namespace with every
// field set. Pairs of booleans are given different values so that a swapped
// field mapping is caught even if both directions swap it.
func fullSpec() tcloudv1alpha1.NamespaceSpec {
	return tcloudv1alpha1.NamespaceSpec{
		Project:       "proj-1",
		Tags:          map[string]string{"team": "core", "env": "prod"},
		Replicas:      []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}, {Region: "aws-us-west-2"}},
		RetentionDays: 30,
		MTLSAuth: &tcloudv1alpha1.MTLSAuthSpec{
			Enabled:          true,
			AcceptedClientCA: "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----\n",
			CertificateFilters: []tcloudv1alpha1.CertificateFilterSpec{
				{CommonName: "cn", Organization: "org", OrganizationalUnit: "ou", SubjectAlternativeName: "san"},
				{CommonName: "cn2"},
			},
		},
		APIKeyAuth: &tcloudv1alpha1.APIKeyAuthSpec{Enabled: true},
		SearchAttributes: map[string]tcloudv1alpha1.SearchAttributeType{
			"CustomText":    tcloudv1alpha1.SearchAttributeTypeText,
			"CustomKeyword": tcloudv1alpha1.SearchAttributeTypeKeyword,
		},
		CodecServer: &tcloudv1alpha1.CodecServerSpec{
			Endpoint:                      "https://codec.example.com",
			PassAccessToken:               true,
			IncludeCrossOriginCredentials: false,
			CustomErrorMessage: &tcloudv1alpha1.CodecServerCustomErrorMessage{
				Default: &tcloudv1alpha1.CodecServerErrorMessage{
					Message: "codec unavailable",
					Link:    "https://docs.example.com",
				},
			},
		},
		Lifecycle: &cloudcommon.LifecycleSpec{EnableDeleteProtection: true},
		HighAvailability: &tcloudv1alpha1.HighAvailabilitySpec{
			DisableManagedFailover:         true,
			DisablePassivePollerForwarding: false,
		},
		ConnectivityRuleIDs: []string{"rule-1", "rule-2"},
		Capacity: &tcloudv1alpha1.CapacitySpec{
			Provisioned: &tcloudv1alpha1.ProvisionedCapacity{Value: 4},
		},
		Fairness: &tcloudv1alpha1.FairnessSpec{TaskQueueFairnessEnabled: true},
		EncryptionValidation: &tcloudv1alpha1.EncryptionValidationSpec{
			Mode:           tcloudv1alpha1.EncryptionValidationModeWarn,
			MetadataKey:    "encoding",
			MetadataValues: []string{"binary/encrypted"},
			InspectHeader:  true,
			InspectFailure: false,
		},
		Description: "the full namespace",
	}
}

func fullProtoNamespace() *tcloudnamespacev1.Namespace {
	return &tcloudnamespacev1.Namespace{
		Namespace: "my-ns.acct1",
		ProjectId: "proj-1",
		Tags:      map[string]string{"team": "core", "env": "prod"},
		Spec: &tcloudnamespacev1.NamespaceSpec{
			Name: "my-ns",
			Replicas: []*tcloudnamespacev1.ReplicaSpec{
				{Region: "aws-us-east-1"},
				{Region: "aws-us-west-2"},
			},
			RetentionDays: 30,
			MtlsAuth: &tcloudnamespacev1.MtlsAuthSpec{
				Enabled:          true,
				AcceptedClientCa: []byte("-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----\n"),
				CertificateFilters: []*tcloudnamespacev1.CertificateFilterSpec{
					{CommonName: "cn", Organization: "org", OrganizationalUnit: "ou", SubjectAlternativeName: "san"},
					{CommonName: "cn2"},
				},
			},
			ApiKeyAuth: &tcloudnamespacev1.ApiKeyAuthSpec{Enabled: true},
			SearchAttributes: map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{
				"CustomText":    tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT,
				"CustomKeyword": tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD,
			},
			CodecServer: &tcloudnamespacev1.CodecServerSpec{
				Endpoint:                      "https://codec.example.com",
				PassAccessToken:               true,
				IncludeCrossOriginCredentials: false,
				CustomErrorMessage: &tcloudnamespacev1.CodecServerSpec_CustomErrorMessage{
					Default: &tcloudnamespacev1.CodecServerSpec_CustomErrorMessage_ErrorMessage{
						Message: "codec unavailable",
						Link:    "https://docs.example.com",
					},
				},
			},
			Lifecycle: &tcloudnamespacev1.LifecycleSpec{EnableDeleteProtection: true},
			HighAvailability: &tcloudnamespacev1.HighAvailabilitySpec{
				DisableManagedFailover:         true,
				DisablePassivePollerForwarding: false,
			},
			ConnectivityRuleIds: []string{"rule-1", "rule-2"},
			CapacitySpec: &tcloudnamespacev1.CapacitySpec{
				Spec: &tcloudnamespacev1.CapacitySpec_Provisioned_{
					Provisioned: &tcloudnamespacev1.CapacitySpec_Provisioned{Value: 4},
				},
			},
			Fairness: &tcloudnamespacev1.FairnessSpec{TaskQueueFairnessEnabled: true},
			EncryptionValidation: &tcloudnamespacev1.EncryptionValidationSpec{
				Mode:           tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN,
				MetadataKey:    "encoding",
				MetadataValues: []string{"binary/encrypted"},
				InspectHeader:  true,
				InspectFailure: false,
			},
			Description: "the full namespace",
		},
	}
}

func crWithSpec(spec tcloudv1alpha1.NamespaceSpec) *tcloudv1alpha1.Namespace {
	cr := &tcloudv1alpha1.Namespace{Spec: spec}
	cr.Name = "my-ns"
	return cr
}

func TestNamespaceSpecFromProto(t *testing.T) {
	minimal := tcloudv1alpha1.NamespaceSpec{
		Replicas:      []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
		RetentionDays: 7,
	}
	cases := []struct {
		name string
		in   *tcloudnamespacev1.Namespace
		exp  tcloudv1alpha1.NamespaceSpec
	}{
		{
			name: "nil namespace",
			in:   nil,
			exp:  tcloudv1alpha1.NamespaceSpec{},
		},
		{
			name: "nil spec keeps outer fields",
			in: &tcloudnamespacev1.Namespace{
				ProjectId: "proj-1",
				Tags:      map[string]string{"a": "1"},
			},
			exp: tcloudv1alpha1.NamespaceSpec{
				Project: "proj-1",
				Tags:    map[string]string{"a": "1"},
			},
		},
		{
			name: "unset optional blocks stay nil",
			in: &tcloudnamespacev1.Namespace{
				Spec: &tcloudnamespacev1.NamespaceSpec{
					Name:          "my-ns",
					Replicas:      []*tcloudnamespacev1.ReplicaSpec{{Region: "aws-us-east-1"}},
					RetentionDays: 7,
				},
			},
			exp: minimal,
		},
		{
			name: "every field",
			in:   fullProtoNamespace(),
			exp:  fullSpec(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require.Equal(tt, tc.exp, namespaceSpecFromProto(tc.in))
		})
	}
}

func TestNamespaceSpecFromProto_CopiesCollections(t *testing.T) {
	require := require.New(t)

	in := fullProtoNamespace()
	got := namespaceSpecFromProto(in)

	in.Tags["team"] = "changed"
	in.Spec.ConnectivityRuleIds[0] = "changed"
	in.Spec.EncryptionValidation.MetadataValues[0] = "changed"

	require.Equal("core", got.Tags["team"])
	require.Equal("rule-1", got.ConnectivityRuleIDs[0])
	require.Equal("binary/encrypted", got.EncryptionValidation.MetadataValues[0])
}

func TestNamespaceSpecToProto(t *testing.T) {
	cases := []struct {
		name string
		in   *tcloudv1alpha1.Namespace
		exp  *tcloudnamespacev1.NamespaceSpec
	}{
		{
			name: "unset optional blocks stay nil",
			in: crWithSpec(tcloudv1alpha1.NamespaceSpec{
				Replicas:      []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 7,
			}),
			exp: &tcloudnamespacev1.NamespaceSpec{
				Name:          "my-ns",
				Replicas:      []*tcloudnamespacev1.ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 7,
			},
		},
		{
			// Project and tags have no place in the proto NamespaceSpec and
			// are sent at the request level instead.
			name: "every field",
			in:   crWithSpec(fullSpec()),
			exp:  fullProtoNamespace().GetSpec(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			got := namespaceSpecToProto(tc.in)
			require.True(tt, proto.Equal(tc.exp, got), "expected %v, got %v", tc.exp, got)
		})
	}
}

func TestNamespaceSpec_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		spec tcloudv1alpha1.NamespaceSpec
	}{
		{
			name: "minimal",
			spec: tcloudv1alpha1.NamespaceSpec{
				Replicas:      []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 7,
			},
		},
		{
			name: "every field",
			spec: fullSpec(),
		},
		{
			name: "on-demand capacity",
			spec: tcloudv1alpha1.NamespaceSpec{
				Replicas:      []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 7,
				Capacity:      &tcloudv1alpha1.CapacitySpec{OnDemand: &tcloudv1alpha1.OnDemandCapacity{}},
			},
		},
		{
			name: "explicitly empty blocks stay non-nil",
			spec: tcloudv1alpha1.NamespaceSpec{
				Replicas:             []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays:        7,
				MTLSAuth:             &tcloudv1alpha1.MTLSAuthSpec{},
				APIKeyAuth:           &tcloudv1alpha1.APIKeyAuthSpec{},
				CodecServer:          &tcloudv1alpha1.CodecServerSpec{CustomErrorMessage: &tcloudv1alpha1.CodecServerCustomErrorMessage{}},
				Lifecycle:            &cloudcommon.LifecycleSpec{},
				HighAvailability:     &tcloudv1alpha1.HighAvailabilitySpec{},
				Fairness:             &tcloudv1alpha1.FairnessSpec{},
				EncryptionValidation: &tcloudv1alpha1.EncryptionValidationSpec{Mode: tcloudv1alpha1.EncryptionValidationModeDisabled},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			cr := crWithSpec(tc.spec)
			tns := &tcloudnamespacev1.Namespace{
				ProjectId: cr.Spec.Project,
				Tags:      cr.Spec.Tags,
				Spec:      namespaceSpecToProto(cr),
			}
			require.Equal(tt, tc.spec, namespaceSpecFromProto(tns))
		})
	}
}

func TestCapacityFromProto(t *testing.T) {
	cases := []struct {
		name string
		in   *tcloudnamespacev1.CapacitySpec
		exp  *tcloudv1alpha1.CapacitySpec
	}{
		{
			name: "nil",
		},
		{
			name: "no mode set",
			in:   &tcloudnamespacev1.CapacitySpec{},
		},
		{
			name: "on demand",
			in: &tcloudnamespacev1.CapacitySpec{
				Spec: &tcloudnamespacev1.CapacitySpec_OnDemand_{
					OnDemand: &tcloudnamespacev1.CapacitySpec_OnDemand{},
				},
			},
			exp: &tcloudv1alpha1.CapacitySpec{OnDemand: &tcloudv1alpha1.OnDemandCapacity{}},
		},
		{
			name: "provisioned truncates fractional TRUs",
			in: &tcloudnamespacev1.CapacitySpec{
				Spec: &tcloudnamespacev1.CapacitySpec_Provisioned_{
					Provisioned: &tcloudnamespacev1.CapacitySpec_Provisioned{Value: 2.9},
				},
			},
			exp: &tcloudv1alpha1.CapacitySpec{
				Provisioned: &tcloudv1alpha1.ProvisionedCapacity{Value: 2},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require.Equal(tt, tc.exp, capacityFromProto(tc.in))
		})
	}
}

func TestCapacityToProto(t *testing.T) {
	cases := []struct {
		name string
		in   *tcloudv1alpha1.CapacitySpec
		exp  *tcloudnamespacev1.CapacitySpec
	}{
		{
			name: "nil",
		},
		{
			name: "no mode set",
			in:   &tcloudv1alpha1.CapacitySpec{},
		},
		{
			name: "on demand",
			in:   &tcloudv1alpha1.CapacitySpec{OnDemand: &tcloudv1alpha1.OnDemandCapacity{}},
			exp: &tcloudnamespacev1.CapacitySpec{
				Spec: &tcloudnamespacev1.CapacitySpec_OnDemand_{
					OnDemand: &tcloudnamespacev1.CapacitySpec_OnDemand{},
				},
			},
		},
		{
			name: "provisioned",
			in: &tcloudv1alpha1.CapacitySpec{
				Provisioned: &tcloudv1alpha1.ProvisionedCapacity{Value: 8},
			},
			exp: &tcloudnamespacev1.CapacitySpec{
				Spec: &tcloudnamespacev1.CapacitySpec_Provisioned_{
					Provisioned: &tcloudnamespacev1.CapacitySpec_Provisioned{Value: 8},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			got := capacityToProto(tc.in)
			require.True(tt, proto.Equal(tc.exp, got), "expected %v, got %v", tc.exp, got)
		})
	}
}

func TestCodecServerFromProto_CustomErrorMessageWithoutDefault(t *testing.T) {
	got := codecServerFromProto(&tcloudnamespacev1.CodecServerSpec{
		Endpoint:           "https://codec.example.com",
		CustomErrorMessage: &tcloudnamespacev1.CodecServerSpec_CustomErrorMessage{},
	})
	require.Equal(t, &tcloudv1alpha1.CodecServerSpec{
		Endpoint:           "https://codec.example.com",
		CustomErrorMessage: &tcloudv1alpha1.CodecServerCustomErrorMessage{},
	}, got)
}

func TestSubMessageConverters_Nil(t *testing.T) {
	require := require.New(t)

	require.Nil(replicasFromProto(nil))
	require.Nil(replicasFromProto([]*tcloudnamespacev1.ReplicaSpec{}))
	require.Nil(mtlsAuthFromProto(nil))
	require.Nil(apiKeyAuthFromProto(nil))
	require.Nil(codecServerFromProto(nil))
	require.Nil(lifecycleFromProto(nil))
	require.Nil(highAvailabilityFromProto(nil))
	require.Nil(fairnessFromProto(nil))
	require.Nil(encryptionValidationFromProto(nil))
	require.Nil(searchAttributesFromProto(nil))

	require.Nil(replicasToProto(nil))
	require.Nil(replicasToProto([]tcloudv1alpha1.ReplicaSpec{}))
	require.Nil(mtlsAuthToProto(nil))
	require.Nil(apiKeyAuthToProto(nil))
	require.Nil(codecServerToProto(nil))
	require.Nil(lifecycleToProto(nil))
	require.Nil(highAvailabilityToProto(nil))
	require.Nil(fairnessToProto(nil))
	require.Nil(encryptionValidationToProto(nil))
	require.Nil(searchAttributesToProto(nil))
}

func TestSearchAttributeTypeConversion(t *testing.T) {
	pairs := []struct {
		proto tcloudnamespacev1.NamespaceSpec_SearchAttributeType
		cr    tcloudv1alpha1.SearchAttributeType
	}{
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT, tcloudv1alpha1.SearchAttributeTypeText},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD, tcloudv1alpha1.SearchAttributeTypeKeyword},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT, tcloudv1alpha1.SearchAttributeTypeInt},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE, tcloudv1alpha1.SearchAttributeTypeDouble},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL, tcloudv1alpha1.SearchAttributeTypeBool},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME, tcloudv1alpha1.SearchAttributeTypeDatetime},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST, tcloudv1alpha1.SearchAttributeTypeKeywordList},
	}
	for _, p := range pairs {
		t.Run(string(p.cr), func(tt *testing.T) {
			require := require.New(tt)
			require.Equal(
				map[string]tcloudv1alpha1.SearchAttributeType{"a": p.cr},
				searchAttributesFromProto(map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{"a": p.proto}),
			)
			require.Equal(
				map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{"a": p.proto},
				searchAttributesToProto(map[string]tcloudv1alpha1.SearchAttributeType{"a": p.cr}),
			)
		})
	}

	t.Run("unspecified and unknown from proto", func(tt *testing.T) {
		got := searchAttributesFromProto(map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{
			"unspecified": tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED,
			"unknown":     tcloudnamespacev1.NamespaceSpec_SearchAttributeType(99),
		})
		require.Equal(tt, map[string]tcloudv1alpha1.SearchAttributeType{
			"unspecified": "",
			"unknown":     "99",
		}, got)
	})

	t.Run("unknown to proto", func(tt *testing.T) {
		got := searchAttributesToProto(map[string]tcloudv1alpha1.SearchAttributeType{"a": "Bogus"})
		require.Equal(tt, map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{
			"a": tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED,
		}, got)
	})
}

func TestEncryptionValidationModeConversion(t *testing.T) {
	pairs := []struct {
		proto tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode
		cr    tcloudv1alpha1.EncryptionValidationMode
	}{
		{tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DISABLED, tcloudv1alpha1.EncryptionValidationModeDisabled},
		{tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN, tcloudv1alpha1.EncryptionValidationModeWarn},
		{tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DENY, tcloudv1alpha1.EncryptionValidationModeDeny},
	}
	for _, p := range pairs {
		t.Run(string(p.cr), func(tt *testing.T) {
			require := require.New(tt)
			require.Equal(p.cr, encryptionValidationModeFromProto(p.proto))
			require.Equal(p.proto, encryptionValidationModeToProto(p.cr))
		})
	}

	t.Run("unspecified and unknown from proto", func(tt *testing.T) {
		require := require.New(tt)
		require.Equal(
			tcloudv1alpha1.EncryptionValidationMode(""),
			encryptionValidationModeFromProto(tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED),
		)
		require.Equal(
			tcloudv1alpha1.EncryptionValidationMode("99"),
			encryptionValidationModeFromProto(tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode(99)),
		)
	})

	t.Run("empty and unknown to proto", func(tt *testing.T) {
		require := require.New(tt)
		require.Equal(
			tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED,
			encryptionValidationModeToProto(""),
		)
		require.Equal(
			tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED,
			encryptionValidationModeToProto("Bogus"),
		)
	})
}

func TestResourceStateFromProto(t *testing.T) {
	cases := []struct {
		in  tcloudresourcev1.ResourceState
		exp cloudcommon.ResourceState
	}{
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_UNSPECIFIED, ""},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATING, cloudcommon.ResourceStateActivating},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATION_FAILED, cloudcommon.ResourceStateActivationFailed},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE, cloudcommon.ResourceStateActive},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING, cloudcommon.ResourceStateUpdating},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATE_FAILED, cloudcommon.ResourceStateUpdateFailed},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETING, cloudcommon.ResourceStateDeleting},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETE_FAILED, cloudcommon.ResourceStateDeleteFailed},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETED, cloudcommon.ResourceStateDeleted},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_SUSPENDED, cloudcommon.ResourceStateSuspended},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_EXPIRED, cloudcommon.ResourceStateExpired},
		// Unknown states must map to "" so the enum-validated status patch
		// is not rejected.
		{tcloudresourcev1.ResourceState(99), ""},
	}
	for _, tc := range cases {
		t.Run(tc.in.String(), func(tt *testing.T) {
			require.Equal(tt, tc.exp, resourceStateFromProto(tc.in))
		})
	}
}

func TestResourceStatusFromProto(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)
	modified := time.Date(2026, 2, 3, 4, 5, 6, 7, time.UTC)

	cases := []struct {
		name string
		in   *tcloudnamespacev1.Namespace
		exp  cloudcommon.ResourceStatus
	}{
		{
			name: "nil namespace",
		},
		{
			name: "every field",
			in: &tcloudnamespacev1.Namespace{
				Namespace:        "my-ns.acct1",
				ResourceVersion:  "rv-1",
				State:            tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
				AsyncOperationId: "op-1",
				CreatedTime:      timestamppb.New(created),
				LastModifiedTime: timestamppb.New(modified),
			},
			exp: cloudcommon.ResourceStatus{
				ID:               "my-ns.acct1",
				ResourceVersion:  "rv-1",
				State:            cloudcommon.ResourceStateActive,
				AsyncOperationID: "op-1",
				CreatedTime:      &metav1.Time{Time: created},
				LastModifiedTime: &metav1.Time{Time: modified},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require.Equal(tt, tc.exp, resourceStatusFromProto(tc.in))
		})
	}
}

func TestTimeFromProto(t *testing.T) {
	require := require.New(t)

	require.Nil(timeFromProto(nil))

	ts := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)
	require.Equal(&metav1.Time{Time: ts}, timeFromProto(timestamppb.New(ts)))
}
