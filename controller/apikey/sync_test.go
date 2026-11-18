package apikey_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/apikey"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

func TestReconciler_ResourceSync_Create(t *testing.T) {
	// notCreated is latest for an API key that doesn't exist in Temporal Cloud
	// yet, with its owner resolved.
	notCreated := func() *tcloudv1alpha1.APIKey {
		key := &tcloudv1alpha1.APIKey{}
		key.ObjectMeta = newAPIKey().ObjectMeta
		key.Spec.Token = newAPIKey().Spec.Token
		key.Status.References = resolvedRefs()
		return key
	}
	unresolved := notCreated()
	unresolved.Status.References = tcloudv1alpha1.APIKeyReferences{}
	noSpec := newAPIKey()
	noSpec.Spec = tcloudv1alpha1.APIKeySpec{}
	otherLocation := newAPIKey()
	otherLocation.Spec.Token.SecretKeyRef = &tcloudv1alpha1.SecretKeyRef{Name: "deploy-creds", Key: "apikey"}

	cases := []struct {
		name          string
		desired       *tcloudv1alpha1.APIKey
		latest        *tcloudv1alpha1.APIKey
		kubeObjs      []runtime.Object
		createErr     error
		expKind       string
		expErr        string
		expCreate     bool
		expID         string
		expSecretData map[string]string // nil means no Secret exists afterwards
		expControlled bool              // whether the APIKey controls the Secret afterwards
	}{
		{
			name:    "fail if spec is left out",
			desired: noSpec,
			latest:  notCreated(),
			expKind: kindTerminal,
			expErr:  "spec is required to create an API key",
		},
		{
			name:    "fail if the owner isn't resolved yet",
			desired: newAPIKey(),
			latest:  unresolved,
			expKind: kindRequeue,
			expErr:  "spec.owner.serviceAccount (apps/ci)",
		},
		{
			name:          "fail if another api key controls the token secret",
			desired:       newAPIKey(),
			latest:        notCreated(),
			kubeObjs:      []runtime.Object{tokenSecret(secretName, "uid-other", map[string]string{"token": "other"})},
			expKind:       kindRequeue,
			expErr:        "is controlled by APIKey owner-uid-other (uid uid-other), not this APIKey",
			expSecretData: map[string]string{"token": "other"},
		},
		{
			name:          "fail if the token secret has no controller",
			desired:       newAPIKey(),
			latest:        notCreated(),
			kubeObjs:      []runtime.Object{tokenSecret(secretName, "", map[string]string{"token": "other"})},
			expKind:       kindRequeue,
			expErr:        "is controlled by nothing, not this APIKey",
			expSecretData: map[string]string{"token": "other"},
		},
		{
			name:      "fail if temporal cloud rejects the create",
			desired:   newAPIKey(),
			latest:    notCreated(),
			createErr: status.Error(codes.InvalidArgument, "bad"),
			expKind:   kindTerminal,
			expErr:    "failed creating tcloud api key",
			expCreate: true,
		},
		{
			name:          "a new key yields a token secret and the saved key id",
			desired:       newAPIKey(),
			latest:        notCreated(),
			expKind:       kindRequeue,
			expErr:        `created tcloud api key "key-1", waiting for activation`,
			expCreate:     true,
			expID:         keyID,
			expSecretData: map[string]string{"token": token},
			expControlled: true,
		},
		{
			name:          "a token secret this api key controls yields the new token",
			desired:       newAPIKey(),
			latest:        notCreated(),
			kubeObjs:      []runtime.Object{tokenSecret(secretName, keyUID, map[string]string{"token": "old"})},
			expKind:       kindRequeue,
			expErr:        `created tcloud api key "key-1", waiting for activation`,
			expCreate:     true,
			expID:         keyID,
			expSecretData: map[string]string{"token": token},
			expControlled: true,
		},
		{
			name:    "another secret name and key yields the token there and keeps other data",
			desired: otherLocation,
			latest:  notCreated(),
			kubeObjs: []runtime.Object{
				tokenSecret("deploy-creds", keyUID, map[string]string{"other": "keep"}),
			},
			expKind:       kindRequeue,
			expErr:        `created tcloud api key "key-1", waiting for activation`,
			expCreate:     true,
			expID:         keyID,
			expSecretData: map[string]string{"apikey": token, "other": "keep"},
			expControlled: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			r := require.New(tt)
			var creates atomic.Int32
			var gotCreate atomic.Pointer[tcloudservicev1.CreateApiKeyRequest]
			rec, kc := newReconciler(tt, tc.kubeObjs, fakeserver.WithCreateApiKey(
				func(
					_ context.Context, req *tcloudservicev1.CreateApiKeyRequest,
				) (*tcloudservicev1.CreateApiKeyResponse, error) {
					creates.Add(1)
					gotCreate.Store(req)
					if tc.createErr != nil {
						return nil, tc.createErr
					}
					return &tcloudservicev1.CreateApiKeyResponse{KeyId: keyID, Token: token}, nil
				},
			))
			desired := apikey.NewResource(tc.desired.DeepCopy())
			latest := apikey.NewResource(tc.latest.DeepCopy())
			beforeDesired, beforeLatest := desired.DeepCopy(), latest.DeepCopy()

			got, err := rec.ResourceSync(context.TODO(), desired, latest)
			r.Equal(tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				r.ErrorContains(err, tc.expErr)
			}
			r.Equal(beforeDesired.CR(), desired.CR(), "desired must not be mutated")
			r.Equal(beforeLatest.CR(), latest.CR(), "latest must not be mutated")

			if tc.expCreate {
				r.Equal(int32(1), creates.Load(), "the key must be created once")
				r.True(proto.Equal(cloudSpec(), gotCreate.Load().GetSpec()), "%v", gotCreate.Load().GetSpec())
			} else {
				r.Zero(creates.Load(), "the key must not be created")
			}
			r.Equal(tc.expID, got.CR().(*tcloudv1alpha1.APIKey).Status.Resource.ID)

			ref := tc.desired.Spec.Token.SecretKeyRef
			if ref == nil {
				ref = newAPIKey().Spec.Token.SecretKeyRef
			}
			secret := readSecret(tt, kc, ref.Name)
			if tc.expSecretData == nil {
				r.Nil(secret, "no Secret must exist")
				return
			}
			r.NotNil(secret)
			gotData := map[string]string{}
			for k, v := range secret.Data {
				gotData[k] = string(v)
			}
			r.Equal(tc.expSecretData, gotData)
			controller := metav1.GetControllerOf(secret)
			r.Equal(tc.expControlled, controller != nil && controller.UID == keyUID)
		})
	}
}

func TestReconciler_ResourceSync_Update(t *testing.T) {
	// active is latest for an API key that matches newAPIKey.
	active := func() *tcloudv1alpha1.APIKey {
		key := newAPIKey()
		key.Spec = tcloudv1alpha1.APIKeySpec{}.FromProto(cloudSpec())
		key.Spec.Token = newAPIKey().Spec.Token
		key.Status.Resource = cloudcommon.ResourceStatus{
			ID: keyID, Version: "rv-1", State: cloudcommon.ResourceStateActive,
		}
		key.Status.References = resolvedRefs()
		return key
	}
	unresolved := active()
	unresolved.Status.References = tcloudv1alpha1.APIKeyReferences{}
	updating := active()
	updating.Status.Resource.State = cloudcommon.ResourceStateUpdating
	otherOwner := active()
	otherOwner.Spec.Owner.ServiceAccount.ID = "sa-other"

	renamed := newAPIKey()
	renamed.Spec.DisplayName = "ci-renamed"
	renamedSpec := cloudSpec()
	renamedSpec.DisplayName = "ci-renamed"

	disabled := newAPIKey()
	disabled.Spec.Disabled = true
	disabledSpec := cloudSpec()
	disabledSpec.Disabled = true

	cases := []struct {
		name      string
		desired   *tcloudv1alpha1.APIKey
		latest    *tcloudv1alpha1.APIKey
		updateErr error
		expKind   string
		expErr    string
		expUpdate *tcloudidentityv1.ApiKeySpec // nil means no update is sent
	}{
		{
			name:    "fail if the owner isn't resolved",
			desired: newAPIKey(),
			latest:  unresolved,
			expKind: kindRequeue,
			expErr:  "spec.owner.serviceAccount (apps/ci)",
		},
		{
			name:    "fail if the key isn't active",
			desired: newAPIKey(),
			latest:  updating,
			expKind: kindRequeue,
			expErr:  `waiting for tcloud api key "key-1" to leave state Updating`,
		},
		{
			name:    "fail if temporal cloud reports another owner",
			desired: newAPIKey(),
			latest:  otherOwner,
			expKind: kindTerminal,
			expErr:  `is owned by service account "sa-other" but spec.owner.serviceAccount is "sa-1"`,
		},
		{
			name:      "fail if temporal cloud rejects the update",
			desired:   renamed,
			latest:    active(),
			updateErr: status.Error(codes.InvalidArgument, "bad"),
			expKind:   kindTerminal,
			expErr:    "failed updating tcloud api key",
			expUpdate: renamedSpec,
		},
		{
			name:    "a key matching the spec yields no update",
			desired: newAPIKey(),
			latest:  active(),
			expKind: kindNil,
		},
		{
			name:      "a new display name yields an update",
			desired:   renamed,
			latest:    active(),
			expKind:   kindRequeue,
			expErr:    `updated tcloud api key "key-1", waiting for update to complete`,
			expUpdate: renamedSpec,
		},
		{
			name:      "disabling the key yields an update",
			desired:   disabled,
			latest:    active(),
			expKind:   kindRequeue,
			expErr:    `updated tcloud api key "key-1", waiting for update to complete`,
			expUpdate: disabledSpec,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			r := require.New(tt)
			var updates atomic.Int32
			var gotUpdate atomic.Pointer[tcloudservicev1.UpdateApiKeyRequest]
			rec, _ := newReconciler(tt, nil, fakeserver.WithUpdateApiKey(
				func(
					_ context.Context, req *tcloudservicev1.UpdateApiKeyRequest,
				) (*tcloudservicev1.UpdateApiKeyResponse, error) {
					updates.Add(1)
					gotUpdate.Store(req)
					if tc.updateErr != nil {
						return nil, tc.updateErr
					}
					return &tcloudservicev1.UpdateApiKeyResponse{}, nil
				},
			))
			desired := apikey.NewResource(tc.desired.DeepCopy())
			latest := apikey.NewResource(tc.latest.DeepCopy())
			beforeDesired, beforeLatest := desired.DeepCopy(), latest.DeepCopy()

			got, err := rec.ResourceSync(context.TODO(), desired, latest)
			r.Equal(tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				r.ErrorContains(err, tc.expErr)
			}
			r.Equal(beforeDesired.CR(), desired.CR(), "desired must not be mutated")
			r.Equal(beforeLatest.CR(), latest.CR(), "latest must not be mutated")
			r.Equal(beforeLatest.CR(), got.CR(), "the returned resource must keep latest's state")

			if tc.expUpdate == nil {
				r.Zero(updates.Load(), "no update must be sent")
				return
			}
			r.Equal(int32(1), updates.Load(), "the update must be sent once")
			req := gotUpdate.Load()
			r.Equal(keyID, req.GetKeyId())
			r.Equal("rv-1", req.GetResourceVersion())
			r.True(proto.Equal(tc.expUpdate, req.GetSpec()), "%v", req.GetSpec())
		})
	}
}
