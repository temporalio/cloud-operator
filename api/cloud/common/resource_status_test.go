package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
)

func TestResourceStateFromProto(t *testing.T) {
	cases := []struct {
		in  tcloudresourcev1.ResourceState
		exp ResourceState
	}{
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_UNSPECIFIED, ""},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATING, ResourceStateActivating},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATION_FAILED, ResourceStateActivationFailed},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE, ResourceStateActive},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING, ResourceStateUpdating},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATE_FAILED, ResourceStateUpdateFailed},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETING, ResourceStateDeleting},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETE_FAILED, ResourceStateDeleteFailed},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETED, ResourceStateDeleted},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_SUSPENDED, ResourceStateSuspended},
		{tcloudresourcev1.ResourceState_RESOURCE_STATE_EXPIRED, ResourceStateExpired},
		// Unknown states must map to "" so the enum-validated status patch
		// is not rejected.
		{tcloudresourcev1.ResourceState(99), ""},
	}
	for _, tc := range cases {
		t.Run(tc.in.String(), func(tt *testing.T) {
			// Start from a non-empty state to check that FromProto always
			// overwrites it, including with "".
			got := ResourceStateActive
			got.FromProto(tc.in)
			require.Equal(tt, tc.exp, got)
		})
	}
}
