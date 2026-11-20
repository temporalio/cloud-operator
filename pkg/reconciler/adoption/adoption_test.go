package adoption_test

import (
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/project"
	"github.com/temporalio/cloud-operator/pkg/reconciler/adoption"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// newProject returns a Project resource with the supplied annotations, status
// ID and state.
func newProject(
	annotations map[string]string,
	id string,
	state cloudcommon.ResourceState,
) types.Resource {
	cr := &tcloudv1alpha1.Project{}
	cr.Name = "my-project"
	cr.Annotations = maps.Clone(annotations)
	cr.Status.Resource.ID = id
	cr.Status.Resource.State = state
	return project.NewResource(cr)
}

var adoptAnnotations = map[string]string{
	cloudcommon.AnnotationKeyAdoptionPolicy: "adopt",
	cloudcommon.AnnotationKeyResourceID:     "proj-1",
}

func TestWanted(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		exp         bool
	}{
		{
			name: "no adoption policy yields false",
			exp:  false,
		},
		{
			name: "adopted resource yields false",
			annotations: map[string]string{
				cloudcommon.AnnotationKeyAdoptionPolicy: "adopt",
				cloudcommon.AnnotationKeyResourceID:     "proj-1",
				cloudcommon.AnnotationKeyAdoptedOn:      "2026-11-03T12:00:00Z",
			},
			exp: false,
		},
		{
			name:        "adopt policy yields true",
			annotations: adoptAnnotations,
			exp:         true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			desired := newProject(tc.annotations, "", "")

			got := adoption.Wanted(desired)

			require.Equal(tc.exp, got)
		})
	}
}

func TestAdopt(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	errNameMismatch := errors.New("name mismatch")

	cases := []struct {
		name        string
		foundID     string
		state       cloudcommon.ResourceState
		check       func(types.Resource) error
		expErr      string
		expRequeue  bool
		expTerminal bool
	}{
		{
			name:       "fail if nothing was found",
			expErr:     `no tcloud project with ID "proj-1" to adopt`,
			expRequeue: true,
		},
		{
			name:        "fail if the kind-specific check refuses",
			foundID:     "proj-1",
			state:       cloudcommon.ResourceStateActive,
			check:       func(types.Resource) error { return errNameMismatch },
			expErr:      "name mismatch",
			expTerminal: true,
		},
		{
			name:        "fail if the found resource is being deleted",
			foundID:     "proj-1",
			state:       cloudcommon.ResourceStateDeleting,
			expErr:      `can't adopt tcloud project "proj-1" in state "Deleting"`,
			expTerminal: true,
		},
		{
			name:    "active resource yields an adopted copy",
			foundID: "proj-1",
			state:   cloudcommon.ResourceStateActive,
		},
		{
			name:    "activating resource yields an adopted copy",
			foundID: "proj-1",
			state:   cloudcommon.ResourceStateActivating,
		},
		{
			name:    "updating resource that passes the check yields an adopted copy",
			foundID: "proj-1",
			state:   cloudcommon.ResourceStateUpdating,
			check:   func(types.Resource) error { return nil },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			desired := newProject(adoptAnnotations, "", "")
			latest := newProject(adoptAnnotations, tc.foundID, tc.state)

			got, err := adoption.Adopt(desired, latest, "project", now, tc.check)

			gotRes := got.(types.Resource)
			adoptedOn, adoptedOnErr := gotRes.AdoptedOn()
			require.NoError(adoptedOnErr)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				var requeueAfter *requeue.RequeueNeededAfter
				require.Equal(tc.expRequeue, errors.As(err, &requeueAfter))
				var terminal *ctrlerrors.TerminalError
				require.Equal(tc.expTerminal, errors.As(err, &terminal))
				// A refused adoption leaves the CR unlinked.
				require.Empty(gotRes.ResourceStatus().ID)
				require.Nil(adoptedOn)
				return
			}
			require.NoError(err)
			require.Equal(tc.foundID, gotRes.ResourceStatus().ID)
			require.NotNil(adoptedOn)
			require.True(now.Equal(*adoptedOn), "adopted-on %s, want %s", adoptedOn, now)
			require.False(latest.IsAdopted(), "latest was mutated")
		})
	}
}

func TestUnchanged(t *testing.T) {
	cases := []struct {
		name  string
		id    string
		expID string
	}{
		{
			name: "unlinked resource yields an unlinked copy without conditions",
		},
		{
			name:  "linked resource yields a linked copy without conditions",
			id:    "proj-1",
			expID: "proj-1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			desired := newProject(adoptAnnotations, tc.id, cloudcommon.ResourceStateActive)
			desired.ReplaceConditions([]*metav1.Condition{{Type: "Ready"}})

			got := adoption.Unchanged(desired)

			gotRes := got.(types.Resource)
			require.Equal(tc.expID, gotRes.ResourceStatus().ID)
			require.Empty(gotRes.Conditions())
			require.Len(desired.Conditions(), 1, "desired was mutated")
		})
	}
}

func TestCheckNotRecreated(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		expErr      string
	}{
		{
			name: "fail if the resource was adopted",
			annotations: map[string]string{
				cloudcommon.AnnotationKeyAdoptedOn: "2026-11-03T12:00:00Z",
			},
			expErr: `adopted tcloud project "proj-1" no longer exists in Temporal Cloud`,
		},
		{
			name: "created resource yields nil",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			desired := newProject(tc.annotations, "proj-1", cloudcommon.ResourceStateActive)

			err := adoption.CheckNotRecreated(desired, "project")

			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				var terminal *ctrlerrors.TerminalError
				require.ErrorAs(err, &terminal)
				return
			}
			require.NoError(err)
		})
	}
}
