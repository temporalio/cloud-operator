package namespace

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

func TestAdoptionPolicy(t *testing.T) {
	const allowed = `. allowed values: adopt`
	cases := []struct {
		name        string
		annotations map[string]string
		exp         tcloudcommon.AdoptionPolicy
		expErr      string
	}{
		{
			name: "unset",
		},
		{
			name: "only other annotations",
			annotations: map[string]string{
				tcloudcommon.AnnotationKeyDeletionPolicy: "retain",
			},
		},
		{
			name:        "adopt",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: "adopt"},
			exp:         tcloudcommon.AdoptionPolicyAdopt,
		},
		{
			name:        "mixed case is lowercased",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: "ADOPT"},
			exp:         tcloudcommon.AdoptionPolicyAdopt,
		},
		{
			name:        "adopt-or-create is invalid",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: "adopt-or-create"},
			exp:         "adopt-or-create",
			expErr:      `invalid adoption policy "adopt-or-create"` + allowed,
		},
		{
			name:        "whitespace is trimmed",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: " adopt "},
			exp:         tcloudcommon.AdoptionPolicyAdopt,
		},
		{
			name:        "empty value is invalid",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: ""},
			expErr:      `invalid adoption policy ""` + allowed,
		},
		{
			name:        "invalid value",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: " Import "},
			exp:         "import",
			expErr:      `invalid adoption policy "import"` + allowed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := &tcloudv1alpha1.Namespace{}
			cr.Annotations = tc.annotations

			got, err := adoptionPolicy(&res{cr})
			if tc.expErr != "" {
				require.EqualError(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.exp, got)
		})
	}
}

func TestAdoptNamespace(t *testing.T) {
	const id = "tmprl-ns.acct1"
	found := func(state tcloudresourcev1.ResourceState) *tcloudnamespacev1.Namespace {
		return &tcloudnamespacev1.Namespace{
			Namespace: id,
			Spec: &tcloudnamespacev1.NamespaceSpec{
				Name:          "tmprl-ns",
				Replicas:      []*tcloudnamespacev1.ReplicaSpec{{Region: "aws-us-east-1"}},
				RetentionDays: 30,
			},
			State: state,
		}
	}
	refused := func(state string) string {
		return fmt.Sprintf("can't adopt tcloud namespace %q in state %q", id, state)
	}
	const nothingFound = `no tcloud namespace named "tmprl-ns" to adopt`

	cases := []struct {
		name string
		// policy is the adoption-policy annotation value. Empty means no
		// annotation.
		policy    string
		found     *tcloudnamespacev1.Namespace
		expLinked bool
		expSource tcloudcommon.ResourceSource
		expErr    string
	}{
		{name: "no policy and nothing found creates"},
		{
			name:      "no policy links found namespace without marking it adopted",
			found:     found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE),
			expLinked: true,
		},
		{
			name:      "no policy links deleted namespace as before",
			found:     found(tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETED),
			expLinked: true,
		},
		{
			name:   "invalid policy is refused",
			policy: "import",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE),
			expErr: `invalid adoption policy "import"`,
		},
		{
			name:   "adopt-or-create is refused as invalid",
			policy: "adopt-or-create",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE),
			expErr: `invalid adoption policy "adopt-or-create"`,
		},
		{name: "adopt with nothing found doesn't create", policy: "adopt", expErr: nothingFound},
		{
			name:   "adopt treats deleted namespace as nothing to adopt",
			policy: "adopt",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETED),
			expErr: nothingFound,
		},
		{
			name: "adopt links active namespace", policy: "adopt",
			found:     found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE),
			expLinked: true, expSource: tcloudcommon.ResourceSourceAdopted,
		},
		{
			name: "adopt links activating namespace", policy: "adopt",
			found:     found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATING),
			expLinked: true, expSource: tcloudcommon.ResourceSourceAdopted,
		},
		{
			name: "adopt links updating namespace", policy: "adopt",
			found:     found(tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING),
			expLinked: true, expSource: tcloudcommon.ResourceSourceAdopted,
		},
		{
			name: "adopt refuses deleting namespace", policy: "adopt",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETING),
			expErr: refused(string(tcloudcommon.ResourceStateDeleting)),
		},
		{
			name: "adopt refuses activation failed namespace", policy: "adopt",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATION_FAILED),
			expErr: refused(string(tcloudcommon.ResourceStateActivationFailed)),
		},
		{
			name: "adopt refuses update failed namespace", policy: "adopt",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATE_FAILED),
			expErr: refused(string(tcloudcommon.ResourceStateUpdateFailed)),
		},
		{
			name: "adopt refuses delete failed namespace", policy: "adopt",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETE_FAILED),
			expErr: refused(string(tcloudcommon.ResourceStateDeleteFailed)),
		},
		{
			name: "adopt refuses suspended namespace", policy: "adopt",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_SUSPENDED),
			expErr: refused(string(tcloudcommon.ResourceStateSuspended)),
		},
		{
			name: "adopt refuses expired namespace", policy: "adopt",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_EXPIRED),
			expErr: refused(string(tcloudcommon.ResourceStateExpired)),
		},
		{
			name: "adopt refuses unknown state", policy: "adopt",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_UNSPECIFIED),
			expErr: refused("RESOURCE_STATE_UNSPECIFIED"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)

			cr := &tcloudv1alpha1.Namespace{}
			cr.Name = "tmprl-ns"
			if tc.policy != "" {
				cr.Annotations = map[string]string{
					tcloudcommon.AnnotationKeyAdoptionPolicy: tc.policy,
				}
			}
			if tc.found != nil {
				// Matching specs, so adopt links instead of patching.
				// Patching is covered by
				// TestReconciler_ResourceSetLatest_AdoptTwoPasses.
				cr.Spec = adoptedSpec(cr, tc.found)
			}
			before := cr.DeepCopy()

			// No kube client, so an unexpected patch panics.
			r := NewReconciler()
			got, err := r.adoptNamespace(context.TODO(), &res{cr}, tc.found)

			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				var requeueAfter *requeue.RequeueNeededAfter
				require.True(errors.As(err, &requeueAfter), "expected requeue error, got %v", err)
				require.Nil(got)
				require.Equal(before, cr, "refusals must not change latest")
				return
			}
			require.NoError(err)
			if tc.expLinked {
				require.Same(tc.found, got)
			} else {
				require.Nil(got)
			}
			require.Equal(tc.expSource, cr.Status.Source)
		})
	}
}
