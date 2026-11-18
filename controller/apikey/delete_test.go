package apikey_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/apikey"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

func TestReconciler_ResourceDelete(t *testing.T) {
	const opID = "op-delete"

	resourceStatus := func(state cloudcommon.ResourceState, asyncOpID string) *cloudcommon.ResourceStatus {
		return &cloudcommon.ResourceStatus{
			ID: keyID, Version: "rv-1", State: state, AsyncOperationID: asyncOpID,
		}
	}
	withStatus := func(
		state cloudcommon.ResourceState,
		asyncOpID string,
		annotations map[string]string,
	) *tcloudv1alpha1.APIKey {
		key := newAPIKey()
		key.Annotations = annotations
		key.Status.Resource = *resourceStatus(state, asyncOpID)
		return key
	}
	active := withStatus(cloudcommon.ResourceStateActive, "", nil)
	retain := map[string]string{cloudcommon.AnnotationKeyDeletionPolicy: "Retain"}
	opIn := func(s operationv1.AsyncOperation_State) fakeserver.NewOption {
		return fakeserver.OnGetAsyncOperationReturn(
			&tcloudservicev1.GetAsyncOperationResponse{
				AsyncOperation: &operationv1.AsyncOperation{Id: opID, State: s, FailureReason: "nope"},
			}, nil,
		)
	}
	getReturns := func(s tcloudresourcev1.ResourceState) fakeserver.NewOption {
		return fakeserver.OnGetApiKeyReturn(&tcloudservicev1.GetApiKeyResponse{
			ApiKey: &tcloudidentityv1.ApiKey{Id: keyID, ResourceVersion: "rv-2", State: s},
		}, nil)
	}
	ourSecret := func() runtime.Object {
		return tokenSecret(secretName, keyUID, map[string]string{"token": token})
	}

	cases := []struct {
		name          string
		cr            *tcloudv1alpha1.APIKey
		kubeObjs      []runtime.Object
		serverOpts    []fakeserver.NewOption
		deleteErr     error // returned by DeleteApiKey
		expKind       string
		expErr        string
		expDeletes    int32
		expStatus     *cloudcommon.ResourceStatus // nil means the status is unchanged
		expStalled    bool
		expSecret     bool // whether the token Secret exists afterwards
		expControlled bool // whether the APIKey still controls the token Secret
	}{
		{
			name:       "fail if the deletion policy is invalid",
			cr:         withStatus(cloudcommon.ResourceStateActive, "", map[string]string{cloudcommon.AnnotationKeyDeletionPolicy: "destroy"}),
			expKind:    kindRequeue,
			expErr:     `invalid deletion policy "destroy"`,
			expStalled: true,
		},
		{
			name:    "fail if an earlier delete failed",
			cr:      withStatus(cloudcommon.ResourceStateDeleteFailed, opID, nil),
			expKind: kindTerminal,
			expErr:  "not retrying",
		},
		{
			name:       "fail if the delete failed in temporal cloud",
			cr:         withStatus(cloudcommon.ResourceStateDeleting, opID, nil),
			serverOpts: []fakeserver.NewOption{opIn(operationv1.AsyncOperation_STATE_FAILED)},
			expKind:    kindTerminal,
			expErr:     "deleting tcloud api key key-1 failed",
			expStatus:  resourceStatus(cloudcommon.ResourceStateDeleteFailed, opID),
			expStalled: true,
		},
		{
			name: "fail if temporal cloud can't read the api key",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetApiKeyReturn(nil, status.Error(codes.PermissionDenied, "api key can't read api keys")),
			},
			expKind: kindOther,
			expErr:  "failed getting tcloud api key key-1",
		},
		{
			name:       "fail if temporal cloud rejects the delete",
			cr:         active,
			serverOpts: []fakeserver.NewOption{getReturns(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE)},
			deleteErr:  status.Error(codes.PermissionDenied, "api key can't delete api keys"),
			expKind:    kindOther,
			expErr:     "failed deleting tcloud api key key-1",
			expDeletes: 1,
			expStalled: true,
		},
		{
			name:    "a key never created yields nothing to delete",
			cr:      newAPIKey(),
			expKind: kindNil,
		},
		{
			name:          "the retain policy yields a kept key and a released secret",
			cr:            withStatus(cloudcommon.ResourceStateActive, "", retain),
			kubeObjs:      []runtime.Object{ourSecret()},
			expKind:       kindNil,
			expSecret:     true,
			expControlled: false,
		},
		{
			name:    "the retain policy with no token secret yields a kept key",
			cr:      withStatus(cloudcommon.ResourceStateActive, "", retain),
			expKind: kindNil,
		},
		{
			name:    "an already deleted key yields done",
			cr:      withStatus(cloudcommon.ResourceStateDeleted, opID, nil),
			expKind: kindNil,
		},
		{
			name:       "a delete in progress yields a wait",
			cr:         withStatus(cloudcommon.ResourceStateDeleting, opID, nil),
			serverOpts: []fakeserver.NewOption{opIn(operationv1.AsyncOperation_STATE_IN_PROGRESS)},
			expKind:    kindRequeue,
		},
		{
			name:       "a finished delete yields done",
			cr:         withStatus(cloudcommon.ResourceStateDeleting, opID, nil),
			serverOpts: []fakeserver.NewOption{opIn(operationv1.AsyncOperation_STATE_FULFILLED)},
			expKind:    kindNil,
		},
		{
			name:       "a key still updating yields a wait with the state saved",
			cr:         active,
			serverOpts: []fakeserver.NewOption{getReturns(tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING)},
			expKind:    kindRequeue,
			expErr:     "to finish Updating before deleting",
			expStatus:  resourceStatus(cloudcommon.ResourceStateUpdating, ""),
		},
		{
			name:          "an active key yields a delete with the current resource version",
			cr:            active,
			kubeObjs:      []runtime.Object{ourSecret()},
			serverOpts:    []fakeserver.NewOption{getReturns(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE)},
			expKind:       kindRequeue,
			expErr:        "deleting tcloud api key key-1 (async operation op-delete)",
			expDeletes:    1,
			expStatus:     resourceStatus(cloudcommon.ResourceStateDeleting, opID),
			expSecret:     true,
			expControlled: true,
		},
		{
			name:       "a key deleted between the read and the delete yields done",
			cr:         active,
			serverOpts: []fakeserver.NewOption{getReturns(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE)},
			deleteErr:  status.Error(codes.NotFound, "gone"),
			expKind:    kindNil,
			expDeletes: 1,
		},
		{
			name: "a key already gone from temporal cloud yields done",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetApiKeyReturn(nil, status.Error(codes.NotFound, "gone")),
			},
			expKind: kindNil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			r := require.New(tt)
			var deletes atomic.Int32
			sopts := append(tc.serverOpts, fakeserver.WithDeleteApiKey(
				func(
					_ context.Context, req *tcloudservicev1.DeleteApiKeyRequest,
				) (*tcloudservicev1.DeleteApiKeyResponse, error) {
					deletes.Add(1)
					assert.Equal(tt, keyID, req.GetKeyId())
					assert.Equal(tt, "rv-2", req.GetResourceVersion())
					if tc.deleteErr != nil {
						return nil, tc.deleteErr
					}
					return &tcloudservicev1.DeleteApiKeyResponse{
						AsyncOperation: &operationv1.AsyncOperation{Id: opID},
					}, nil
				},
			))
			rec, kc := newReconciler(tt, tc.kubeObjs, sopts...)
			original := apikey.NewResource(tc.cr.DeepCopy())
			before := original.DeepCopy()

			got, err := rec.ResourceDelete(context.TODO(), original)
			r.Equal(tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				r.ErrorContains(err, tc.expErr)
			}
			r.Equal(before.CR(), original.CR(), "original must not be mutated")
			r.Equal(tc.expDeletes, deletes.Load())

			expStatus := tc.cr.Status.Resource
			if tc.expStatus != nil {
				expStatus = *tc.expStatus
			}
			r.Equal(expStatus, got.CR().(*tcloudv1alpha1.APIKey).Status.Resource)
			stalled := condition.Stalled(got)
			if tc.expStalled {
				r.NotNil(stalled)
				r.Equal(metav1.ConditionTrue, stalled.Status)
			} else {
				r.Nil(stalled)
			}

			secret := readSecret(tt, kc, secretName)
			if !tc.expSecret {
				r.Nil(secret, "no Secret must exist")
				return
			}
			r.NotNil(secret, "the token Secret must be kept")
			controller := metav1.GetControllerOf(secret)
			r.Equal(tc.expControlled, controller != nil && controller.UID == keyUID)
			r.Equal(token, string(secret.Data["token"]), "the token must be kept")
		})
	}
}
