package apikey_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/apikey"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

func TestReconciler_ResourceLatest(t *testing.T) {
	created := newAPIKey()
	created.Status.Resource.ID = keyID

	activeOwner := []runtime.Object{ownerServiceAccount(cloudcommon.ResourceStateActive)}
	notCreatedSpec := tcloudv1alpha1.APIKeySpec{Token: newAPIKey().Spec.Token}

	cases := []struct {
		name     string
		cr       *tcloudv1alpha1.APIKey
		kubeObjs []runtime.Object
		getResp  *tcloudidentityv1.ApiKey
		getErr   error
		expErr   string
		expGetID string // the key ID read from Temporal Cloud, if any
		expSpec  tcloudv1alpha1.APIKeySpec
		expRes   cloudcommon.ResourceStatus
		expRefs  tcloudv1alpha1.APIKeyReferences
	}{
		{
			name:     "fail if temporal cloud can't read the api key",
			cr:       created,
			kubeObjs: activeOwner,
			getErr:   status.Error(codes.PermissionDenied, "api key can't read api keys"),
			expErr:   "failed getting tcloud api key",
		},
		{
			name:     "a key not created yet yields an empty spec with the owner resolved",
			cr:       newAPIKey(),
			kubeObjs: activeOwner,
			expSpec:  notCreatedSpec,
			expRefs:  resolvedRefs(),
		},
		{
			name:     "an owner that isn't active yet yields no resolved owner",
			cr:       newAPIKey(),
			kubeObjs: []runtime.Object{ownerServiceAccount(cloudcommon.ResourceStateActivating)},
			expSpec:  notCreatedSpec,
		},
		{
			name:     "a created key yields its state from temporal cloud",
			cr:       created,
			kubeObjs: activeOwner,
			getResp: &tcloudidentityv1.ApiKey{
				Id:              keyID,
				ResourceVersion: "rv-1",
				State:           tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
				Spec:            cloudSpec(),
			},
			expGetID: keyID,
			expSpec: tcloudv1alpha1.APIKeySpec{
				DisplayName: "ci",
				Description: "deploys",
				Owner: tcloudv1alpha1.APIKeyOwner{
					ServiceAccount: cloudcommon.ResourceReferenceOrID{ID: saID},
				},
				ExpiryTime: expiry,
				Token:      newAPIKey().Spec.Token,
			},
			expRes: cloudcommon.ResourceStatus{
				ID: keyID, Version: "rv-1", State: cloudcommon.ResourceStateActive,
			},
			expRefs: resolvedRefs(),
		},
		{
			name:     "a key deleted outside the operator yields an empty spec",
			cr:       created,
			kubeObjs: activeOwner,
			getErr:   status.Error(codes.NotFound, "gone"),
			expGetID: keyID,
			expSpec:  notCreatedSpec,
			expRefs:  resolvedRefs(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			r := require.New(tt)
			var gotGetID atomic.Pointer[string]
			rec, _ := newReconciler(tt, tc.kubeObjs, fakeserver.WithGetApiKey(
				func(
					_ context.Context, req *tcloudservicev1.GetApiKeyRequest,
				) (*tcloudservicev1.GetApiKeyResponse, error) {
					id := req.GetKeyId()
					gotGetID.Store(&id)
					if tc.getErr != nil {
						return nil, tc.getErr
					}
					return &tcloudservicev1.GetApiKeyResponse{ApiKey: tc.getResp}, nil
				},
			))
			current := apikey.NewResource(tc.cr.DeepCopy())
			before := current.DeepCopy()

			got, err := rec.ResourceLatest(context.TODO(), current)
			r.Equal(before.CR(), current.CR(), "current must not be mutated")
			if tc.expErr != "" {
				r.ErrorContains(err, tc.expErr)
				return
			}
			r.NoError(err)
			if tc.expGetID == "" {
				r.Nil(gotGetID.Load(), "temporal cloud must not be read")
			} else {
				r.Equal(tc.expGetID, *gotGetID.Load())
			}
			key := got.CR().(*tcloudv1alpha1.APIKey)
			r.Equal(tc.expSpec, key.Spec)
			r.Equal(tc.expRes, key.Status.Resource)
			r.Equal(tc.expRefs, key.Status.References)
		})
	}
}
