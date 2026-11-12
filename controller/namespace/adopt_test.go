package namespace

import (
	"errors"
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

func TestAdoptNamespace_Adopts(t *testing.T) {
	desired, latest := newAdoptTestResources("my-ns.acct1", cloudcommon.ResourceStateActive)
	latestNS := latest.CR().(*tcloudv1alpha1.Namespace)
	latestNS.Spec.Replicas = []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}}
	latestNS.Spec.RetentionDays = 30

	got, err := adoptNamespace(desired, latest, adoptTestTime)

	require.NoError(t, err)
	adoptedOn, err := got.(types.Resource).AdoptedOn()
	require.NoError(t, err)
	require.NotNil(t, adoptedOn)
	require.True(t, adoptTestTime.Equal(*adoptedOn), "adopted-on is %s", adoptedOn)
	gotNS := got.CR().(*tcloudv1alpha1.Namespace)
	require.Equal(t, "my-ns.acct1", gotNS.Status.Resource.ID)
	require.Equal(t, latestNS.Spec, gotNS.Spec)
}

func TestAdoptNamespace_NothingFoundRequeues(t *testing.T) {
	desired, latest := newAdoptTestResources("", "")

	got, err := adoptNamespace(desired, latest, adoptTestTime)

	var requeueAfter *requeue.RequeueNeededAfter
	require.True(t, errors.As(err, &requeueAfter), "expected a requeue error, got %v", err)
	require.Empty(t, got.(types.Resource).ResourceStatus().ID)
}

func TestAdoptNamespace_NameMismatchIsTerminal(t *testing.T) {
	desired, latest := newAdoptTestResources("other-ns.acct1", cloudcommon.ResourceStateActive)

	got, err := adoptNamespace(desired, latest, adoptTestTime)

	var terminal *ctrlerrors.TerminalError
	require.True(t, errors.As(err, &terminal), "expected a terminal error, got %v", err)
	require.Empty(t, got.(types.Resource).ResourceStatus().ID)
}

func TestAdoptNamespace_NonAdoptableStateIsTerminal(t *testing.T) {
	desired, latest := newAdoptTestResources("my-ns.acct1", cloudcommon.ResourceStateDeleting)

	got, err := adoptNamespace(desired, latest, adoptTestTime)

	var terminal *ctrlerrors.TerminalError
	require.True(t, errors.As(err, &terminal), "expected a terminal error, got %v", err)
	require.Empty(t, got.(types.Resource).ResourceStatus().ID)
}
