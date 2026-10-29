package v1alpha1

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
)

// fullSpec and fullProtoNamespace describe the same namespace with every
// field set. Pairs of booleans are given different values so that a swapped
// field mapping is caught even if both directions swap it.
func fullSpec() NamespaceSpec {
	return NamespaceSpec{
		Project:       "proj-1",
		Tags:          map[string]string{"team": "core", "env": "prod"},
		Replicas:      []ReplicaSpec{{Region: "aws-us-east-1"}, {Region: "aws-us-west-2"}},
		RetentionDays: 30,
		MTLSAuth: &MTLSAuthSpec{
			Enabled:          true,
			AcceptedClientCA: "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----\n",
			CertificateFilters: []CertificateFilterSpec{
				{CommonName: "cn", Organization: "org", OrganizationalUnit: "ou", SubjectAlternativeName: "san"},
				{CommonName: "cn2"},
			},
		},
		APIKeyAuth: &APIKeyAuthSpec{Enabled: true},
		SearchAttributes: map[string]SearchAttributeType{
			"CustomText":    SearchAttributeTypeText,
			"CustomKeyword": SearchAttributeTypeKeyword,
		},
		CodecServer: &CodecServerSpec{
			Endpoint:                      "https://codec.example.com",
			PassAccessToken:               true,
			IncludeCrossOriginCredentials: false,
			CustomErrorMessage: &CodecServerCustomErrorMessage{
				Default: &CodecServerErrorMessage{
					Message: "codec unavailable",
					Link:    "https://docs.example.com",
				},
			},
		},
		Lifecycle: &cloudcommon.LifecycleSpec{EnableDeleteProtection: true},
		HighAvailability: &HighAvailabilitySpec{
			DisableManagedFailover:         true,
			DisablePassivePollerForwarding: false,
		},
		ConnectivityRuleIDs: []string{"rule-1", "rule-2"},
		Capacity: &CapacitySpec{
			Provisioned: &ProvisionedCapacity{Value: 4},
		},
		Fairness: &FairnessSpec{TaskQueueFairnessEnabled: true},
		EncryptionValidation: &EncryptionValidationSpec{
			Mode:           EncryptionValidationModeWarn,
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

func namespaceWithSpec(spec NamespaceSpec) *Namespace {
	ns := &Namespace{Spec: spec}
	ns.Name = "my-ns"
	return ns
}

func TestNamespaceFromProto_Spec(t *testing.T) {
	minimal := NamespaceSpec{
		Replicas:      []ReplicaSpec{{Region: "aws-us-east-1"}},
		RetentionDays: 7,
	}
	cases := []struct {
		name string
		in   *tcloudnamespacev1.Namespace
		exp  NamespaceSpec
	}{
		{
			name: "nil namespace",
			in:   nil,
			exp:  NamespaceSpec{},
		},
		{
			name: "nil spec keeps outer fields",
			in: &tcloudnamespacev1.Namespace{
				ProjectId: "proj-1",
				Tags:      map[string]string{"a": "1"},
			},
			exp: NamespaceSpec{
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
			var got Namespace
			got.FromProto(tc.in)
			require.Equal(tt, tc.exp, got.Spec)
		})
	}
}

func TestNamespaceFromProto_Status(t *testing.T) {
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
			require := require.New(tt)

			got := namespaceWithSpec(NamespaceSpec{})
			got.FromProto(tc.in)

			require.Equal(tc.exp, got.Status.ResourceStatus)
			// The qualified ID goes to status, never to metadata.name.
			require.Equal("my-ns", got.Name)
		})
	}
}

func TestNamespaceFromProto_CopiesCollections(t *testing.T) {
	require := require.New(t)

	in := fullProtoNamespace()
	var got Namespace
	got.FromProto(in)

	in.Tags["team"] = "changed"
	in.Spec.ConnectivityRuleIds[0] = "changed"
	in.Spec.EncryptionValidation.MetadataValues[0] = "changed"

	require.Equal("core", got.Spec.Tags["team"])
	require.Equal("rule-1", got.Spec.ConnectivityRuleIDs[0])
	require.Equal("binary/encrypted", got.Spec.EncryptionValidation.MetadataValues[0])
}

func TestNamespaceToProto(t *testing.T) {
	// ToProto sets only the fields that are sent to Temporal Cloud, so the
	// qualified ID and status fields of the full proto are left unset.
	full := fullProtoNamespace()
	full.Namespace = ""

	cases := []struct {
		name string
		in   *Namespace
		exp  *tcloudnamespacev1.Namespace
	}{
		{
			name: "unset optional blocks stay nil",
			in: namespaceWithSpec(NamespaceSpec{
				Replicas:      []ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 7,
			}),
			exp: &tcloudnamespacev1.Namespace{
				Spec: &tcloudnamespacev1.NamespaceSpec{
					Name:          "my-ns",
					Replicas:      []*tcloudnamespacev1.ReplicaSpec{{Region: "aws-us-east-1"}},
					RetentionDays: 7,
				},
			},
		},
		{
			name: "every field",
			in:   namespaceWithSpec(fullSpec()),
			exp:  full,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			got := tc.in.ToProto()
			require.True(tt, proto.Equal(tc.exp, got), "expected %v, got %v", tc.exp, got)
		})
	}
}

func TestNamespace_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		spec NamespaceSpec
	}{
		{
			name: "minimal",
			spec: NamespaceSpec{
				Replicas:      []ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 7,
			},
		},
		{
			name: "every field",
			spec: fullSpec(),
		},
		{
			name: "on-demand capacity",
			spec: NamespaceSpec{
				Replicas:      []ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 7,
				Capacity:      &CapacitySpec{OnDemand: &OnDemandCapacity{}},
			},
		},
		{
			name: "explicitly empty blocks stay non-nil",
			spec: NamespaceSpec{
				Replicas:             []ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays:        7,
				MTLSAuth:             &MTLSAuthSpec{},
				APIKeyAuth:           &APIKeyAuthSpec{},
				CodecServer:          &CodecServerSpec{CustomErrorMessage: &CodecServerCustomErrorMessage{}},
				Lifecycle:            &cloudcommon.LifecycleSpec{},
				HighAvailability:     &HighAvailabilitySpec{},
				Fairness:             &FairnessSpec{},
				EncryptionValidation: &EncryptionValidationSpec{Mode: EncryptionValidationModeDisabled},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			var got Namespace
			got.FromProto(namespaceWithSpec(tc.spec).ToProto())
			require.Equal(tt, tc.spec, got.Spec)
		})
	}
}

func TestCapacityFromProto(t *testing.T) {
	cases := []struct {
		name string
		in   *tcloudnamespacev1.CapacitySpec
		exp  *CapacitySpec
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
			exp: &CapacitySpec{OnDemand: &OnDemandCapacity{}},
		},
		{
			name: "provisioned truncates fractional TRUs",
			in: &tcloudnamespacev1.CapacitySpec{
				Spec: &tcloudnamespacev1.CapacitySpec_Provisioned_{
					Provisioned: &tcloudnamespacev1.CapacitySpec_Provisioned{Value: 2.9},
				},
			},
			exp: &CapacitySpec{
				Provisioned: &ProvisionedCapacity{Value: 2},
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
		in   *CapacitySpec
		exp  *tcloudnamespacev1.CapacitySpec
	}{
		{
			name: "nil",
		},
		{
			name: "no mode set",
			in:   &CapacitySpec{},
		},
		{
			name: "on demand",
			in:   &CapacitySpec{OnDemand: &OnDemandCapacity{}},
			exp: &tcloudnamespacev1.CapacitySpec{
				Spec: &tcloudnamespacev1.CapacitySpec_OnDemand_{
					OnDemand: &tcloudnamespacev1.CapacitySpec_OnDemand{},
				},
			},
		},
		{
			name: "provisioned",
			in: &CapacitySpec{
				Provisioned: &ProvisionedCapacity{Value: 8},
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
	require.Equal(t, &CodecServerSpec{
		Endpoint:           "https://codec.example.com",
		CustomErrorMessage: &CodecServerCustomErrorMessage{},
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
	require.Nil(replicasToProto([]ReplicaSpec{}))
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
		cr    SearchAttributeType
	}{
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT, SearchAttributeTypeText},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD, SearchAttributeTypeKeyword},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT, SearchAttributeTypeInt},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE, SearchAttributeTypeDouble},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL, SearchAttributeTypeBool},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME, SearchAttributeTypeDatetime},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST, SearchAttributeTypeKeywordList},
	}
	for _, p := range pairs {
		t.Run(string(p.cr), func(tt *testing.T) {
			require := require.New(tt)
			require.Equal(
				map[string]SearchAttributeType{"a": p.cr},
				searchAttributesFromProto(map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{"a": p.proto}),
			)
			require.Equal(
				map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{"a": p.proto},
				searchAttributesToProto(map[string]SearchAttributeType{"a": p.cr}),
			)
		})
	}

	t.Run("unspecified and unknown from proto", func(tt *testing.T) {
		got := searchAttributesFromProto(map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{
			"unspecified": tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED,
			"unknown":     tcloudnamespacev1.NamespaceSpec_SearchAttributeType(99),
		})
		require.Equal(tt, map[string]SearchAttributeType{
			"unspecified": "",
			"unknown":     "99",
		}, got)
	})

	t.Run("unknown to proto", func(tt *testing.T) {
		got := searchAttributesToProto(map[string]SearchAttributeType{"a": "Bogus"})
		require.Equal(tt, map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{
			"a": tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED,
		}, got)
	})
}

func TestEncryptionValidationModeConversion(t *testing.T) {
	pairs := []struct {
		proto tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode
		cr    EncryptionValidationMode
	}{
		{tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DISABLED, EncryptionValidationModeDisabled},
		{tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN, EncryptionValidationModeWarn},
		{tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DENY, EncryptionValidationModeDeny},
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
			EncryptionValidationMode(""),
			encryptionValidationModeFromProto(tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED),
		)
		require.Equal(
			EncryptionValidationMode("99"),
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
