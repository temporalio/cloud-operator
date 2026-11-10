package serviceaccount_test

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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/serviceaccount"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

func TestReconciler_ResourceDelete(t *testing.T) {
	const opID = "op-delete"

	resourceStatus := func(
		state cloudcommon.ResourceState,
		asyncOpID string,
	) *cloudcommon.ResourceStatus {
		return &cloudcommon.ResourceStatus{
			ID: saID, Version: "rv-1", State: state, AsyncOperationID: asyncOpID,
		}
	}
	withStatus := func(
		state cloudcommon.ResourceState,
		asyncOpID string,
		annotations map[string]string,
	) *tcloudv1alpha1.ServiceAccount {
		sa := newServiceAccount(id(nsID))
		sa.Annotations = annotations
		sa.Status.Resource = *resourceStatus(state, asyncOpID)
		return sa
	}
	active := withStatus(cloudcommon.ResourceStateActive, "", nil)
	opIn := func(s operationv1.AsyncOperation_State) fakeserver.NewOption {
		return fakeserver.OnGetAsyncOperationReturn(
			&tcloudservicev1.GetAsyncOperationResponse{
				AsyncOperation: &operationv1.AsyncOperation{Id: opID, State: s, FailureReason: "nope"},
			}, nil,
		)
	}

	cases := []struct {
		name       string
		cr         *tcloudv1alpha1.ServiceAccount
		serverOpts []fakeserver.NewOption
		deleteErr  error // returned by DeleteServiceAccount
		expKind    string
		expErr     string
		expDelete  bool
		expStatus  *cloudcommon.ResourceStatus // nil means the status is unchanged
		expStalled bool
	}{
		// Not linked to a service account in Temporal Cloud.
		{
			name:    "never created in temporal cloud, so there's nothing to delete",
			cr:      newServiceAccount(id(nsID)),
			expKind: kindNil,
		},
		{
			name: "only a resource-id annotation, so nothing is deleted",
			cr: func() *tcloudv1alpha1.ServiceAccount {
				sa := newServiceAccount(id(nsID))
				sa.Annotations = map[string]string{cloudcommon.AnnotationKeyResourceID: "sa-existing"}
				return sa
			}(),
			expKind: kindNil,
		},

		// The deletion policy.
		{
			name: "deletion policy is retain, so the service account is kept",
			cr: withStatus(cloudcommon.ResourceStateActive, "", map[string]string{
				cloudcommon.AnnotationKeyDeletionPolicy: "Retain",
			}),
			expKind: kindNil,
		},
		{
			name: "deletion policy is invalid, so it stalls until it's fixed",
			cr: withStatus(cloudcommon.ResourceStateActive, "", map[string]string{
				cloudcommon.AnnotationKeyDeletionPolicy: "destroy",
			}),
			expKind:    kindRequeue,
			expErr:     `invalid deletion policy "destroy"`,
			expStalled: true,
		},

		// An earlier delete, or a create or update that is still running, decides
		// what happens, so no new delete is sent.
		{
			name:    "already deleted, so it's done",
			cr:      withStatus(cloudcommon.ResourceStateDeleted, opID, nil),
			expKind: kindNil,
		},
		{
			name:    "an earlier delete failed, so it isn't retried",
			cr:      withStatus(cloudcommon.ResourceStateDeleteFailed, opID, nil),
			expKind: kindTerminal,
			expErr:  "not retrying",
		},
		{
			name:       "the delete is still running, so it waits",
			cr:         withStatus(cloudcommon.ResourceStateDeleting, opID, nil),
			serverOpts: []fakeserver.NewOption{opIn(operationv1.AsyncOperation_STATE_IN_PROGRESS)},
			expKind:    kindRequeue,
		},
		{
			name:       "the delete finished, so it's done",
			cr:         withStatus(cloudcommon.ResourceStateDeleting, opID, nil),
			serverOpts: []fakeserver.NewOption{opIn(operationv1.AsyncOperation_STATE_FULFILLED)},
			expKind:    kindNil,
		},
		{
			name:       "the delete failed in temporal cloud, so it fails and stalls",
			cr:         withStatus(cloudcommon.ResourceStateDeleting, opID, nil),
			serverOpts: []fakeserver.NewOption{opIn(operationv1.AsyncOperation_STATE_FAILED)},
			expKind:    kindTerminal,
			expErr:     "deleting tcloud service account sa-1 failed",
			expStatus:  resourceStatus(cloudcommon.ResourceStateDeleteFailed, opID),
			expStalled: true,
		},
		{
			name:    "still activating, so it waits before deleting",
			cr:      withStatus(cloudcommon.ResourceStateActivating, "", nil),
			expKind: kindRequeue,
			expErr:  "to finish Activating before deleting",
		},
		{
			name:    "still updating, so it waits before deleting",
			cr:      withStatus(cloudcommon.ResourceStateUpdating, "", nil),
			expKind: kindRequeue,
			expErr:  "to finish Updating before deleting",
		},

		// Sending the delete.
		{
			name: "active, so the delete is sent with the current resource version",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetServiceAccountReturn(&tcloudservicev1.GetServiceAccountResponse{
					ServiceAccount: &tcloudidentityv1.ServiceAccount{Id: saID, ResourceVersion: "rv-2"},
				}, nil),
			},
			expKind:   kindRequeue,
			expErr:    "deleting tcloud service account sa-1 (async operation op-delete)",
			expDelete: true,
			expStatus: resourceStatus(cloudcommon.ResourceStateDeleting, opID),
		},
		{
			name: "already gone from temporal cloud, so it's done",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetServiceAccountReturn(nil, status.Error(codes.NotFound, "gone")),
			},
			expKind: kindNil,
		},
		{
			name: "temporal cloud can't be read, so an error is returned",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetServiceAccountReturn(
					nil, status.Error(codes.PermissionDenied, "api key can't read service accounts"),
				),
			},
			expKind: kindOther,
			expErr:  "failed getting tcloud service account sa-1",
		},
		{
			name: "temporal cloud rejects the delete, so it stalls",
			cr:   active,
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetServiceAccountReturn(&tcloudservicev1.GetServiceAccountResponse{
					ServiceAccount: &tcloudidentityv1.ServiceAccount{Id: saID, ResourceVersion: "rv-2"},
				}, nil),
			},
			deleteErr:  status.Error(codes.PermissionDenied, "api key can't delete service accounts"),
			expKind:    kindOther,
			expErr:     "failed deleting tcloud service account sa-1",
			expDelete:  true,
			expStalled: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var deleted atomic.Bool
			sopts := append(tc.serverOpts, fakeserver.OnDeleteServiceAccount(
				func(req *tcloudservicev1.DeleteServiceAccountRequest) {
					deleted.Store(true)
					assert.Equal(t, saID, req.GetServiceAccountId())
					assert.Equal(t, "rv-2", req.GetResourceVersion())
				},
				func() (*tcloudservicev1.DeleteServiceAccountResponse, error) {
					if tc.deleteErr != nil {
						return nil, tc.deleteErr
					}
					return &tcloudservicev1.DeleteServiceAccountResponse{
						AsyncOperation: &operationv1.AsyncOperation{Id: opID},
					}, nil
				},
			))
			r := newReconciler(t, nil, sopts...)
			original := serviceaccount.NewResource(tc.cr.DeepCopy())
			before := original.DeepCopy()

			got, err := r.ResourceDelete(context.TODO(), original)
			require.Equal(t, tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			}
			require.Equal(t, before.CR(), original.CR(), "original must not be mutated")
			require.Equal(t, tc.expDelete, deleted.Load())

			expStatus := tc.cr.Status.Resource
			if tc.expStatus != nil {
				expStatus = *tc.expStatus
			}
			cr := got.CR().(*tcloudv1alpha1.ServiceAccount)
			require.Equal(t, expStatus, cr.Status.Resource)
			stalled := condition.Stalled(got)
			if tc.expStalled {
				require.NotNil(t, stalled)
				require.Equal(t, metav1.ConditionTrue, stalled.Status)
			} else {
				require.Nil(t, stalled)
			}
		})
	}
}
