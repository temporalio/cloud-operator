package namespace_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// adoptTestResources returns an unlinked adoption CR and a copy with the
// given Cloud ID and state, as if ResourceLatest had found it.
func adoptTestResources(
	foundID string,
	state cloudcommon.ResourceState,
) (types.Resource, types.Resource) {
	cr := &tcloudv1alpha1.Namespace{}
	cr.Name = "my-ns"
	cr.Annotations = map[string]string{
		cloudcommon.AnnotationKeyAdoptionPolicy: "adopt",
		cloudcommon.AnnotationKeyResourceID:     "my-ns.acct1",
	}
	found := cr.DeepCopy()
	found.Status.Resource.ID = foundID
	found.Status.Resource.State = state
	return namespace.NewResource(cr), namespace.NewResource(found)
}

// Adoption runs before ResourceSync needs a Temporal Cloud client, so these
// tests use a reconciler without one.
func TestReconciler_ResourceSync_Adopt(t *testing.T) {
	t.Run("adopts an active namespace", func(tt *testing.T) {
		require := require.New(tt)
		desired, latest := adoptTestResources("my-ns.acct1", cloudcommon.ResourceStateActive)
		latestNS := latest.CR().(*tcloudv1alpha1.Namespace)
		latestNS.Spec.Replicas = []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}}
		latestNS.Spec.RetentionDays = 30

		before := time.Now().Truncate(time.Second)
		got, err := namespace.NewReconciler().ResourceSync(context.TODO(), desired, latest)
		after := time.Now()

		require.NoError(err)
		gotRes := got.(types.Resource)
		require.Equal("my-ns.acct1", gotRes.ResourceStatus().ID)
		adoptedOn, err := gotRes.AdoptedOn()
		require.NoError(err)
		require.NotNil(adoptedOn)
		require.False(adoptedOn.Before(before), "adopted-on %s is before %s", adoptedOn, before)
		require.False(adoptedOn.After(after), "adopted-on %s is after %s", adoptedOn, after)
		// Temporal Cloud's spec is copied into the CR.
		require.Equal(latestNS.Spec, got.CR().(*tcloudv1alpha1.Namespace).Spec)
	})

	// Every refusal is a terminal error that leaves the CR unlinked.
	refusals := []struct {
		name    string
		foundID string
		state   cloudcommon.ResourceState
	}{
		{
			name:    "refuses a name mismatch",
			foundID: "other-ns.acct1",
			state:   cloudcommon.ResourceStateActive,
		},
		{
			name:    "refuses a non-adoptable state",
			foundID: "my-ns.acct1",
			state:   cloudcommon.ResourceStateDeleting,
		},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			desired, latest := adoptTestResources(tc.foundID, tc.state)

			got, err := namespace.NewReconciler().ResourceSync(context.TODO(), desired, latest)

			var terminal *ctrlerrors.TerminalError
			require.ErrorAs(err, &terminal)
			gotRes := got.(types.Resource)
			require.Empty(gotRes.ResourceStatus().ID)
			adoptedOn, err := gotRes.AdoptedOn()
			require.NoError(err)
			require.Nil(adoptedOn)
		})
	}
}
