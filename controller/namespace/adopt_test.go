package namespace

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/types"
)

var adoptTestTime = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// newAdoptTestResources returns an unlinked adoption CR and a copy with the
// given Cloud ID and state, as if ResourceLatest had found it.
func newAdoptTestResources(
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
	return NewResource(cr), NewResource(found)
}

// adoptOutcome is what adoptNamespace is expected to do with a case.
type adoptOutcome int

const (
	// adopted: no error, the CR is linked to the found ID and has adopted-on.
	adopted adoptOutcome = iota
	// requeued: a requeue error, not linked, no adopted-on.
	requeued
	// refused: a terminal error, not linked, no adopted-on.
	refused
)

func TestAdoptNamespace(t *testing.T) {
	cases := []struct {
		name    string
		foundID string
		state   cloudcommon.ResourceState
		exp     adoptOutcome
	}{
		{
			name:    "adopts an active namespace",
			foundID: "my-ns.acct1",
			state:   cloudcommon.ResourceStateActive,
			exp:     adopted,
		},
		{
			name: "nothing found requeues",
			exp:  requeued,
		},
		{
			name:    "name mismatch is terminal",
			foundID: "other-ns.acct1",
			state:   cloudcommon.ResourceStateActive,
			exp:     refused,
		},
		{
			name:    "non-adoptable state is terminal",
			foundID: "my-ns.acct1",
			state:   cloudcommon.ResourceStateDeleting,
			exp:     refused,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			desired, latest := newAdoptTestResources(tc.foundID, tc.state)
			latestNS := latest.CR().(*tcloudv1alpha1.Namespace)
			latestNS.Spec.Replicas = []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}}
			latestNS.Spec.RetentionDays = 30

			got, err := adoptNamespace(desired, latest, adoptTestTime)

			switch tc.exp {
			case adopted:
				require.NoError(err)
			case requeued:
				var requeueAfter *requeue.RequeueNeededAfter
				require.ErrorAs(err, &requeueAfter)
			case refused:
				var terminal *ctrlerrors.TerminalError
				require.ErrorAs(err, &terminal)
			}

			gotRes := got.(types.Resource)
			adoptedOn, aoErr := gotRes.AdoptedOn()
			require.NoError(aoErr)

			if tc.exp != adopted {
				// A refusal or requeue must not link the CR.
				require.Empty(gotRes.ResourceStatus().ID)
				require.Nil(adoptedOn)
				return
			}
			require.Equal(tc.foundID, gotRes.ResourceStatus().ID)
			require.NotNil(adoptedOn)
			require.True(adoptTestTime.Equal(*adoptedOn), "adopted-on is %s", adoptedOn)
			// tCloud's spec is copied into the CR.
			gotNS := got.CR().(*tcloudv1alpha1.Namespace)
			require.Equal(latestNS.Spec, gotNS.Spec)
		})
	}
}
