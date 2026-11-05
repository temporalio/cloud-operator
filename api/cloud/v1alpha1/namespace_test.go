package v1alpha1_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	"github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

// fullSpec and fullProtoNamespace describe the same namespace with every
// field set. Pairs of booleans are given different values so that a swapped
// field mapping is caught even if both directions swap it.
func fullSpec() v1alpha1.NamespaceSpec {
	return v1alpha1.NamespaceSpec{
		Project:       "proj-1",
		Tags:          map[string]string{"team": "core", "env": "prod"},
		Replicas:      []v1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}, {Region: "aws-us-west-2"}},
		RetentionDays: 30,
		MTLSAuth: &v1alpha1.MTLSAuthSpec{
			Enabled:          true,
			AcceptedClientCA: "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----\n",
			CertificateFilters: []v1alpha1.CertificateFilterSpec{
				{CommonName: "cn", Organization: "org", OrganizationalUnit: "ou", SubjectAlternativeName: "san"},
				{CommonName: "cn2"},
			},
		},
		APIKeyAuth: &v1alpha1.APIKeyAuthSpec{Enabled: true},
		SearchAttributes: map[string]v1alpha1.SearchAttributeType{
			"CustomText":    v1alpha1.SearchAttributeTypeText,
			"CustomKeyword": v1alpha1.SearchAttributeTypeKeyword,
		},
		CodecServer: &v1alpha1.CodecServerSpec{
			Endpoint:                      "https://codec.example.com",
			PassAccessToken:               true,
			IncludeCrossOriginCredentials: false,
			CustomErrorMessage: &v1alpha1.CodecServerCustomErrorMessage{
				Default: &v1alpha1.CodecServerErrorMessage{
					Message: "codec unavailable",
					Link:    "https://docs.example.com",
				},
			},
		},
		Lifecycle: &cloudcommon.LifecycleSpec{EnableDeleteProtection: true},
		HighAvailability: &v1alpha1.HighAvailabilitySpec{
			DisableManagedFailover:         true,
			DisablePassivePollerForwarding: false,
		},
		ConnectivityRuleIDs: []string{"rule-1", "rule-2"},
		Capacity: &v1alpha1.CapacitySpec{
			Provisioned: &v1alpha1.ProvisionedCapacity{Value: 4},
		},
		Fairness: &v1alpha1.FairnessSpec{TaskQueueFairnessEnabled: true},
		EncryptionValidation: &v1alpha1.EncryptionValidationSpec{
			Mode:           v1alpha1.EncryptionValidationModeWarn,
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

func namespaceWithSpec(spec v1alpha1.NamespaceSpec) *v1alpha1.Namespace {
	ns := &v1alpha1.Namespace{Spec: spec}
	ns.Name = "my-ns"
	return ns
}

func TestNamespaceFromProto_Spec(t *testing.T) {
	minimal := v1alpha1.NamespaceSpec{
		Replicas:      []v1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
		RetentionDays: 7,
	}
	cases := []struct {
		name string
		in   *tcloudnamespacev1.Namespace
		exp  v1alpha1.NamespaceSpec
	}{
		{
			name: "nil namespace",
			in:   nil,
			exp:  v1alpha1.NamespaceSpec{},
		},
		{
			name: "nil spec keeps outer fields",
			in: &tcloudnamespacev1.Namespace{
				ProjectId: "proj-1",
				Tags:      map[string]string{"a": "1"},
			},
			exp: v1alpha1.NamespaceSpec{
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
			var got v1alpha1.Namespace
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

			got := namespaceWithSpec(v1alpha1.NamespaceSpec{})
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
	var got v1alpha1.Namespace
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
		in   *v1alpha1.Namespace
		exp  *tcloudnamespacev1.Namespace
	}{
		{
			name: "unset optional blocks stay nil",
			in: namespaceWithSpec(v1alpha1.NamespaceSpec{
				Replicas:      []v1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
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
		spec v1alpha1.NamespaceSpec
	}{
		{
			name: "minimal",
			spec: v1alpha1.NamespaceSpec{
				Replicas:      []v1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 7,
			},
		},
		{
			name: "every field",
			spec: fullSpec(),
		},
		{
			name: "on-demand capacity",
			spec: v1alpha1.NamespaceSpec{
				Replicas:      []v1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 7,
				Capacity:      &v1alpha1.CapacitySpec{OnDemand: &v1alpha1.OnDemandCapacity{}},
			},
		},
		{
			name: "explicitly empty blocks stay non-nil",
			spec: v1alpha1.NamespaceSpec{
				Replicas:             []v1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays:        7,
				MTLSAuth:             &v1alpha1.MTLSAuthSpec{},
				APIKeyAuth:           &v1alpha1.APIKeyAuthSpec{},
				CodecServer:          &v1alpha1.CodecServerSpec{CustomErrorMessage: &v1alpha1.CodecServerCustomErrorMessage{}},
				Lifecycle:            &cloudcommon.LifecycleSpec{},
				HighAvailability:     &v1alpha1.HighAvailabilitySpec{},
				Fairness:             &v1alpha1.FairnessSpec{},
				EncryptionValidation: &v1alpha1.EncryptionValidationSpec{Mode: v1alpha1.EncryptionValidationModeDisabled},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			var got v1alpha1.Namespace
			got.FromProto(namespaceWithSpec(tc.spec).ToProto())
			require.Equal(tt, tc.spec, got.Spec)
		})
	}
}

// TestNamespace_AdoptRoundTrip checks that a spec filled in from Temporal
// Cloud for the adopt policy reads back from the API server unchanged and maps
// back to Temporal Cloud's spec, so adopting makes no update. NamespaceSpec
// has no CRD defaults, so the API server's round trip is a JSON one.
func TestNamespace_AdoptRoundTrip(t *testing.T) {
	cloud := fullProtoNamespace()

	var adopted v1alpha1.Namespace
	adopted.FromProto(cloud)
	adopted.Spec.Project = "" // a name-only CR has no project

	data, err := json.Marshal(adopted.Spec)
	require.NoError(t, err)
	var stored v1alpha1.NamespaceSpec
	require.NoError(t, json.Unmarshal(data, &stored))
	require.Equal(t, adopted.Spec, stored)

	got := namespaceWithSpec(stored).ToProto().GetSpec()
	require.True(
		t, proto.Equal(cloud.GetSpec(), got),
		"want %v, got %v", cloud.GetSpec(), got,
	)
}

func TestCapacityFromProto(t *testing.T) {
	cases := []struct {
		name string
		in   *tcloudnamespacev1.CapacitySpec
		exp  *v1alpha1.CapacitySpec
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
			exp: &v1alpha1.CapacitySpec{OnDemand: &v1alpha1.OnDemandCapacity{}},
		},
		{
			name: "provisioned truncates fractional TRUs",
			in: &tcloudnamespacev1.CapacitySpec{
				Spec: &tcloudnamespacev1.CapacitySpec_Provisioned_{
					Provisioned: &tcloudnamespacev1.CapacitySpec_Provisioned{Value: 2.9},
				},
			},
			exp: &v1alpha1.CapacitySpec{
				Provisioned: &v1alpha1.ProvisionedCapacity{Value: 2},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			// Go through v1alpha1.NamespaceSpec, which decides when Capacity is nil.
			var got v1alpha1.NamespaceSpec
			got.FromProto(&tcloudnamespacev1.NamespaceSpec{CapacitySpec: tc.in})
			require.Equal(tt, tc.exp, got.Capacity)
		})
	}
}

func TestCapacityToProto(t *testing.T) {
	cases := []struct {
		name string
		in   *v1alpha1.CapacitySpec
		exp  *tcloudnamespacev1.CapacitySpec
	}{
		{
			name: "nil",
		},
		{
			name: "no mode set",
			in:   &v1alpha1.CapacitySpec{},
		},
		{
			name: "on demand",
			in:   &v1alpha1.CapacitySpec{OnDemand: &v1alpha1.OnDemandCapacity{}},
			exp: &tcloudnamespacev1.CapacitySpec{
				Spec: &tcloudnamespacev1.CapacitySpec_OnDemand_{
					OnDemand: &tcloudnamespacev1.CapacitySpec_OnDemand{},
				},
			},
		},
		{
			name: "provisioned",
			in: &v1alpha1.CapacitySpec{
				Provisioned: &v1alpha1.ProvisionedCapacity{Value: 8},
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
			got := tc.in.ToProto()
			require.True(tt, proto.Equal(tc.exp, got), "expected %v, got %v", tc.exp, got)
		})
	}
}

func TestCodecServerFromProto_CustomErrorMessageWithoutDefault(t *testing.T) {
	var got v1alpha1.CodecServerSpec
	got.FromProto(&tcloudnamespacev1.CodecServerSpec{
		Endpoint:           "https://codec.example.com",
		CustomErrorMessage: &tcloudnamespacev1.CodecServerSpec_CustomErrorMessage{},
	})
	require.Equal(t, v1alpha1.CodecServerSpec{
		Endpoint:           "https://codec.example.com",
		CustomErrorMessage: &v1alpha1.CodecServerCustomErrorMessage{},
	}, got)
}

func TestSubMessageConverters_Nil(t *testing.T) {
	require := require.New(t)

	require.Nil(v1alpha1.ReplicasFromProto(nil))
	require.Nil(v1alpha1.ReplicasFromProto([]*tcloudnamespacev1.ReplicaSpec{}))
	require.Nil(v1alpha1.LifecycleFromProto(nil))
	require.Nil(v1alpha1.SearchAttributesFromProto(nil))

	require.Nil(v1alpha1.ReplicasToProto(nil))
	require.Nil(v1alpha1.ReplicasToProto([]v1alpha1.ReplicaSpec{}))
	require.Nil(v1alpha1.LifecycleToProto(nil))
	require.Nil(v1alpha1.SearchAttributesToProto(nil))

	// ToProto methods are called on optional fields, so they must accept a nil
	// receiver. Nil handling for FromProto is the caller's job and is covered
	// by TestNamespaceFromProto_Spec.
	require.Nil((*v1alpha1.ReplicaSpec)(nil).ToProto())
	require.Nil((*v1alpha1.MTLSAuthSpec)(nil).ToProto())
	require.Nil((*v1alpha1.CertificateFilterSpec)(nil).ToProto())
	require.Nil((*v1alpha1.APIKeyAuthSpec)(nil).ToProto())
	require.Nil((*v1alpha1.CodecServerSpec)(nil).ToProto())
	require.Nil((*v1alpha1.CodecServerCustomErrorMessage)(nil).ToProto())
	require.Nil((*v1alpha1.CodecServerErrorMessage)(nil).ToProto())
	require.Nil((*v1alpha1.HighAvailabilitySpec)(nil).ToProto())
	require.Nil((*v1alpha1.CapacitySpec)(nil).ToProto())
	require.Nil((*v1alpha1.OnDemandCapacity)(nil).ToProto())
	require.Nil((*v1alpha1.ProvisionedCapacity)(nil).ToProto())
	require.Nil((*v1alpha1.FairnessSpec)(nil).ToProto())
	require.Nil((*v1alpha1.EncryptionValidationSpec)(nil).ToProto())
}

func TestSearchAttributeTypeConversion(t *testing.T) {
	pairs := []struct {
		proto tcloudnamespacev1.NamespaceSpec_SearchAttributeType
		cr    v1alpha1.SearchAttributeType
	}{
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_TEXT, v1alpha1.SearchAttributeTypeText},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD, v1alpha1.SearchAttributeTypeKeyword},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_INT, v1alpha1.SearchAttributeTypeInt},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DOUBLE, v1alpha1.SearchAttributeTypeDouble},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_BOOL, v1alpha1.SearchAttributeTypeBool},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_DATETIME, v1alpha1.SearchAttributeTypeDatetime},
		{tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD_LIST, v1alpha1.SearchAttributeTypeKeywordList},
	}
	for _, p := range pairs {
		t.Run(string(p.cr), func(tt *testing.T) {
			require := require.New(tt)
			require.Equal(
				map[string]v1alpha1.SearchAttributeType{"a": p.cr},
				v1alpha1.SearchAttributesFromProto(map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{"a": p.proto}),
			)
			require.Equal(
				map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{"a": p.proto},
				v1alpha1.SearchAttributesToProto(map[string]v1alpha1.SearchAttributeType{"a": p.cr}),
			)
		})
	}

	t.Run("unspecified and unknown from proto", func(tt *testing.T) {
		got := v1alpha1.SearchAttributesFromProto(map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{
			"unspecified": tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED,
			"unknown":     tcloudnamespacev1.NamespaceSpec_SearchAttributeType(99),
		})
		require.Equal(tt, map[string]v1alpha1.SearchAttributeType{
			"unspecified": "",
			"unknown":     "99",
		}, got)
	})

	t.Run("unknown to proto", func(tt *testing.T) {
		got := v1alpha1.SearchAttributesToProto(map[string]v1alpha1.SearchAttributeType{"a": "Bogus"})
		require.Equal(tt, map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{
			"a": tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_UNSPECIFIED,
		}, got)
	})
}

func TestEncryptionValidationModeConversion(t *testing.T) {
	fromProto := func(in tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode) v1alpha1.EncryptionValidationMode {
		var m v1alpha1.EncryptionValidationMode
		m.FromProto(in)
		return m
	}

	pairs := []struct {
		proto tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode
		cr    v1alpha1.EncryptionValidationMode
	}{
		{tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DISABLED, v1alpha1.EncryptionValidationModeDisabled},
		{tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_WARN, v1alpha1.EncryptionValidationModeWarn},
		{tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_DENY, v1alpha1.EncryptionValidationModeDeny},
	}
	for _, p := range pairs {
		t.Run(string(p.cr), func(tt *testing.T) {
			require := require.New(tt)
			require.Equal(p.cr, fromProto(p.proto))
			require.Equal(p.proto, p.cr.ToProto())
		})
	}

	t.Run("unspecified and unknown from proto", func(tt *testing.T) {
		require := require.New(tt)
		require.Equal(
			v1alpha1.EncryptionValidationMode(""),
			fromProto(tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED),
		)
		require.Equal(
			v1alpha1.EncryptionValidationMode("99"),
			fromProto(tcloudnamespacev1.EncryptionValidationSpec_EncryptionValidationMode(99)),
		)
	})

	t.Run("empty and unknown to proto", func(tt *testing.T) {
		require := require.New(tt)
		require.Equal(
			tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED,
			v1alpha1.EncryptionValidationMode("").ToProto(),
		)
		require.Equal(
			tcloudnamespacev1.EncryptionValidationSpec_ENCRYPTION_VALIDATION_MODE_UNSPECIFIED,
			v1alpha1.EncryptionValidationMode("Bogus").ToProto(),
		)
	})
}
