package v1alpha1_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	"github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

var apiKeyExpiry = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

func serviceAccountRef(namespace, name string) cloudcommon.ResourceReferenceOrID {
	return cloudcommon.ResourceReferenceOrID{
		Ref: &cloudcommon.ResourceReference{Namespace: namespace, Name: name},
	}
}

func resolvedServiceAccount(namespace, name, id string) v1alpha1.APIKeyReferences {
	return v1alpha1.APIKeyReferences{
		ServiceAccount: &v1alpha1.ServiceAccountResolvedReference{
			Namespace: namespace, Name: name, ID: id,
		},
	}
}

func TestAPIKeySpec_ToProto(t *testing.T) {
	cases := []struct {
		name string
		spec v1alpha1.APIKeySpec
		refs v1alpha1.APIKeyReferences
		exp  *tcloudidentityv1.ApiKeySpec
	}{
		{
			name: "owner given by id, therefore every field is sent as is",
			spec: v1alpha1.APIKeySpec{
				DisplayName: "ci",
				Description: "deploys",
				Owner:       v1alpha1.APIKeyOwner{ServiceAccount: cloudcommon.ResourceReferenceOrID{ID: "sa-1"}},
				ExpiryTime:  metav1.NewTime(apiKeyExpiry),
				Disabled:    true,
			},
			exp: &tcloudidentityv1.ApiKeySpec{
				OwnerId:     "sa-1",
				OwnerType:   tcloudidentityv1.OwnerType_OWNER_TYPE_SERVICE_ACCOUNT,
				DisplayName: "ci",
				Description: "deploys",
				ExpiryTime:  timestamppb.New(apiKeyExpiry),
				Disabled:    true,
			},
		},
		{
			name: "owner ref without a namespace, therefore the id resolved in the api key's namespace is sent",
			spec: v1alpha1.APIKeySpec{
				DisplayName: "ci",
				Owner:       v1alpha1.APIKeyOwner{ServiceAccount: serviceAccountRef("", "ci")},
			},
			refs: resolvedServiceAccount("apps", "ci", "sa-1"),
			exp: &tcloudidentityv1.ApiKeySpec{
				OwnerId:     "sa-1",
				OwnerType:   tcloudidentityv1.OwnerType_OWNER_TYPE_SERVICE_ACCOUNT,
				DisplayName: "ci",
			},
		},
		{
			name: "owner ref doesn't match the resolved service account, therefore no owner id is sent",
			spec: v1alpha1.APIKeySpec{
				DisplayName: "ci",
				Owner:       v1alpha1.APIKeyOwner{ServiceAccount: serviceAccountRef("shared", "ci")},
			},
			refs: resolvedServiceAccount("apps", "ci", "sa-1"),
			exp: &tcloudidentityv1.ApiKeySpec{
				OwnerType:   tcloudidentityv1.OwnerType_OWNER_TYPE_SERVICE_ACCOUNT,
				DisplayName: "ci",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.spec.ToProto("apps", tc.refs)
			require.True(t, proto.Equal(tc.exp, got), "expected:\n%v\ngot:\n%v", tc.exp, got)
		})
	}
}

func TestAPIKeySpec_FromProto(t *testing.T) {
	cases := []struct {
		name string
		in   *tcloudidentityv1.ApiKeySpec
		exp  v1alpha1.APIKeySpec
	}{
		{
			name: "service account owner, therefore it's read back as an owner id",
			in: &tcloudidentityv1.ApiKeySpec{
				OwnerId:     "sa-1",
				OwnerType:   tcloudidentityv1.OwnerType_OWNER_TYPE_SERVICE_ACCOUNT,
				DisplayName: "ci",
				Description: "deploys",
				ExpiryTime:  timestamppb.New(apiKeyExpiry),
				Disabled:    true,
			},
			exp: v1alpha1.APIKeySpec{
				DisplayName: "ci",
				Description: "deploys",
				Owner:       v1alpha1.APIKeyOwner{ServiceAccount: cloudcommon.ResourceReferenceOrID{ID: "sa-1"}},
				ExpiryTime:  metav1.NewTime(apiKeyExpiry),
				Disabled:    true,
			},
		},
		{
			name: "a user owner isn't supported, therefore the owner is left empty",
			in: &tcloudidentityv1.ApiKeySpec{
				OwnerId:     "user-1",
				OwnerType:   tcloudidentityv1.OwnerType_OWNER_TYPE_USER,
				DisplayName: "ci",
			},
			exp: v1alpha1.APIKeySpec{DisplayName: "ci"},
		},
		{
			name: "no spec from temporal cloud, therefore the spec is empty",
			in:   nil,
			exp:  v1alpha1.APIKeySpec{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var spec v1alpha1.APIKeySpec
			got := spec.FromProto(tc.in)
			require.Equal(t, tc.exp.DisplayName, got.DisplayName)
			require.Equal(t, tc.exp.Description, got.Description)
			require.Equal(t, tc.exp.Owner, got.Owner)
			require.Equal(t, tc.exp.Disabled, got.Disabled)
			require.True(t, tc.exp.ExpiryTime.Equal(&got.ExpiryTime), "expiry: %v", got.ExpiryTime)
		})
	}
}

// TestAPIKeySpec_RoundTrip checks that what Temporal Cloud returns for a spec
// compares equal to the spec that was sent.
func TestAPIKeySpec_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		spec v1alpha1.APIKeySpec
	}{
		{
			name: "every field set, therefore every field comes back unchanged",
			spec: v1alpha1.APIKeySpec{
				DisplayName: "ci",
				Description: "deploys",
				Owner:       v1alpha1.APIKeyOwner{ServiceAccount: cloudcommon.ResourceReferenceOrID{ID: "sa-1"}},
				ExpiryTime:  metav1.NewTime(apiKeyExpiry),
				Disabled:    true,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var none v1alpha1.APIKeyReferences
			sent := tc.spec.ToProto("apps", none)
			var fromCloud v1alpha1.APIKeySpec
			fromCloud = fromCloud.FromProto(sent)
			got := fromCloud.ToProto("apps", none)
			require.True(t, proto.Equal(sent, got), "expected:\n%v\ngot:\n%v", sent, got)
		})
	}
}

func TestAPIKey_FromProto(t *testing.T) {
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	key := v1alpha1.APIKey{}
	key.Name = "ci"
	key.Namespace = "apps"
	key.Spec.Token = &v1alpha1.APIKeyToken{
		SecretKeyRef: &v1alpha1.SecretKeyRef{Name: "ci-api-key", Key: "token"},
	}
	key.Status.References = resolvedServiceAccount("apps", "ci", "sa-1")

	cases := []struct {
		name     string
		in       *tcloudidentityv1.ApiKey
		expSpec  v1alpha1.APIKeySpec
		expState cloudcommon.ResourceStatus
	}{
		{
			name: "read from temporal cloud, therefore spec and status are filled in and the token location is kept",
			in: &tcloudidentityv1.ApiKey{
				Id:               "key-1",
				ResourceVersion:  "rv-1",
				State:            tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
				AsyncOperationId: "op-1",
				CreatedTime:      timestamppb.New(created),
				Spec: &tcloudidentityv1.ApiKeySpec{
					OwnerId:     "sa-1",
					OwnerType:   tcloudidentityv1.OwnerType_OWNER_TYPE_SERVICE_ACCOUNT,
					DisplayName: "ci",
					ExpiryTime:  timestamppb.New(apiKeyExpiry),
				},
			},
			expSpec: v1alpha1.APIKeySpec{
				DisplayName: "ci",
				Owner:       v1alpha1.APIKeyOwner{ServiceAccount: cloudcommon.ResourceReferenceOrID{ID: "sa-1"}},
				ExpiryTime:  metav1.NewTime(apiKeyExpiry),
				Token:       key.Spec.Token,
			},
			expState: cloudcommon.ResourceStatus{
				ID:               "key-1",
				Version:          "rv-1",
				State:            cloudcommon.ResourceStateActive,
				AsyncOperationID: "op-1",
			},
		},
		{
			name: "no api key in temporal cloud, therefore only metadata, the token location and references are kept",
			in:   nil,
			expSpec: v1alpha1.APIKeySpec{
				Token: key.Spec.Token,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := key.FromProto(tc.in)
			require.Equal(t, key.ObjectMeta, got.ObjectMeta)
			require.Equal(t, key.Status.References, got.Status.References)
			require.Equal(t, tc.expSpec.Token, got.Spec.Token)
			require.Equal(t, tc.expSpec.Owner, got.Spec.Owner)
			require.Equal(t, tc.expSpec.DisplayName, got.Spec.DisplayName)
			require.True(t, tc.expSpec.ExpiryTime.Equal(&got.Spec.ExpiryTime))
			require.Equal(t, tc.expState.ID, got.Status.Resource.ID)
			require.Equal(t, tc.expState.Version, got.Status.Resource.Version)
			require.Equal(t, tc.expState.State, got.Status.Resource.State)
			require.Equal(t, tc.expState.AsyncOperationID, got.Status.Resource.AsyncOperationID)
			if tc.in != nil {
				require.True(t, got.Status.Resource.CreatedTime.Time.Equal(created))
			}
		})
	}
}

func TestAPIKeyReferences_ServiceAccountID(t *testing.T) {
	refs := resolvedServiceAccount("apps", "ci", "sa-1")
	cases := []struct {
		name    string
		refs    v1alpha1.APIKeyReferences
		ref     cloudcommon.ResourceReference
		expID   string
		expFind bool
	}{
		{
			name:    "a ref without a namespace, therefore it's looked up in the api key's namespace",
			refs:    refs,
			ref:     cloudcommon.ResourceReference{Name: "ci"},
			expID:   "sa-1",
			expFind: true,
		},
		{
			name:    "a ref with the same namespace is found",
			refs:    refs,
			ref:     cloudcommon.ResourceReference{Namespace: "apps", Name: "ci"},
			expID:   "sa-1",
			expFind: true,
		},
		{
			name: "a ref to another service account isn't found",
			refs: refs,
			ref:  cloudcommon.ResourceReference{Name: "deployer"},
		},
		{
			name: "nothing resolved yet, therefore nothing is found",
			ref:  cloudcommon.ResourceReference{Name: "ci"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, found := tc.refs.ServiceAccountID(tc.ref, "apps")
			require.Equal(t, tc.expID, id)
			require.Equal(t, tc.expFind, found)
		})
	}
}
