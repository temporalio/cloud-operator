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
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	testkube "github.com/temporalio/cloud-operator/pkg/testutil/kube"
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
	const nothingFound = `no tcloud namespace with ID "tmprl-ns.acct1" to adopt`
	const noID = `the cloud.temporal.io/adoption-policy annotation needs the cloud.temporal.io/resource-id annotation`
	otherName := found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE)
	otherName.Spec.Name = "other-ns"

	cases := []struct {
		name string
		// policy is the adoption-policy annotation value. Empty means no
		// annotation.
		policy string
		// resourceID is the resource-id annotation value. Empty means the
		// ID of found when policy is set, and no annotation otherwise.
		resourceID string
		// noResourceID leaves out the resource-id annotation.
		noResourceID bool
		// adopted sets the adopted annotation to "true", as the first
		// adopt pass does.
		adopted   bool
		found     *tcloudnamespacev1.Namespace
		expLinked bool
		expErr    string
	}{
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
			name: "adopt without resource-id is refused", policy: "adopt", noResourceID: true,
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE),
			expErr: noID,
		},
		{
			name: "adopt with blank resource-id is refused", policy: "adopt", resourceID: "  ",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE),
			expErr: noID,
		},
		{
			name: "invalid policy is refused before missing resource-id", policy: "import",
			noResourceID: true,
			expErr:       `invalid adoption policy "import"`,
		},
		{
			name: "adopt refuses namespace whose name doesn't match the CR", policy: "adopt",
			found:  otherName,
			expErr: `can't adopt tcloud namespace "tmprl-ns.acct1": its name "other-ns" doesn't match the CR's name "tmprl-ns"`,
		},
		{
			name: "adopt trims resource-id", policy: "adopt", resourceID: " tmprl-ns.acct1 ",
			adopted:   true,
			found:     found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE),
			expLinked: true,
		},
		{
			name:   "adopt treats deleted namespace as nothing to adopt",
			policy: "adopt",
			found:  found(tcloudresourcev1.ResourceState_RESOURCE_STATE_DELETED),
			expErr: nothingFound,
		},
		{
			name: "adopt links active namespace", policy: "adopt",
			adopted:   true,
			found:     found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE),
			expLinked: true,
		},
		{
			name: "adopt links activating namespace", policy: "adopt",
			adopted:   true,
			found:     found(tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVATING),
			expLinked: true,
		},
		{
			name: "adopt links updating namespace", policy: "adopt",
			adopted:   true,
			found:     found(tcloudresourcev1.ResourceState_RESOURCE_STATE_UPDATING),
			expLinked: true,
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
			cr.Annotations = map[string]string{}
			if tc.policy != "" {
				cr.Annotations[tcloudcommon.AnnotationKeyAdoptionPolicy] = tc.policy
				if !tc.noResourceID {
					rid := tc.resourceID
					if rid == "" {
						rid = id
					}
					cr.Annotations[tcloudcommon.AnnotationKeyResourceID] = rid
				}
			}
			if tc.adopted {
				cr.Annotations[tcloudcommon.AnnotationKeyAdopted] = "true"
			}
			if tc.found != nil {
				// Matching specs, so adopt links instead of patching.
				// Patching is covered by TestAdoptNamespace_Patches and
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
			require.Equal(before, cr, "linking must not change latest")
		})
	}
}

// TestAdoptNamespace_Patches checks that the adopt policy stores the adopted
// annotation, and Temporal Cloud's settings if the spec differs, before it
// links the Namespace.
func TestAdoptNamespace_Patches(t *testing.T) {
	found := &tcloudnamespacev1.Namespace{
		Namespace: "tmprl-ns.acct1",
		Spec: &tcloudnamespacev1.NamespaceSpec{
			Name:          "tmprl-ns",
			Replicas:      []*tcloudnamespacev1.ReplicaSpec{{Region: "aws-us-east-1"}},
			RetentionDays: 30,
		},
		State: tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
	}

	cases := []struct {
		name string
		// adopted is the adopted annotation value. Empty means no
		// annotation.
		adopted string
		// matching is whether the CR's spec already matches found.
		matching bool
	}{
		{name: "spec differs and not adopted"},
		{name: "spec matches but not adopted", matching: true},
		{name: "spec differs but already adopted", adopted: "true"},
		{name: "adopted false is set to true", adopted: "false", matching: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)
			ctx := context.TODO()

			cr := &tcloudv1alpha1.Namespace{}
			cr.Name = "tmprl-ns"
			cr.Namespace = "temporal-cloud-operator"
			cr.Annotations = map[string]string{
				tcloudcommon.AnnotationKeyAdoptionPolicy: "adopt",
				tcloudcommon.AnnotationKeyResourceID:     "tmprl-ns.acct1",
				"example.com/other":                      "kept",
			}
			if tc.adopted != "" {
				cr.Annotations[tcloudcommon.AnnotationKeyAdopted] = tc.adopted
			}
			if tc.matching {
				cr.Spec = adoptedSpec(cr, found)
			}

			kc := testkube.NewFakeClient(cr)
			stored := &tcloudv1alpha1.Namespace{}
			require.NoError(kc.Get(ctx, kclient.ObjectKeyFromObject(cr), stored))
			before := stored.DeepCopy()

			r := NewReconciler(WithKubeClient(kc))
			got, err := r.adoptNamespace(ctx, &res{stored}, found)

			require.ErrorContains(err, "marked it adopted, waiting to link it")
			var requeueAfter *requeue.RequeueNeededAfter
			require.True(errors.As(err, &requeueAfter), "expected requeue error, got %v", err)
			require.Nil(got, "must not link before the annotation is stored")
			require.Equal(before, stored, "patching must not change latest")

			patched := &tcloudv1alpha1.Namespace{}
			require.NoError(kc.Get(ctx, kclient.ObjectKeyFromObject(cr), patched))
			require.Equal(map[string]string{
				tcloudcommon.AnnotationKeyAdoptionPolicy: "adopt",
				tcloudcommon.AnnotationKeyResourceID:     "tmprl-ns.acct1",
				tcloudcommon.AnnotationKeyAdopted:        "true",
				"example.com/other":                      "kept",
			}, patched.Annotations)
			require.Equal(adoptedSpec(cr, found), patched.Spec)
			require.Empty(patched.Status.ID)

			// The next pass links it without patching again.
			latest := &res{patched.DeepCopy()}
			got, err = r.adoptNamespace(ctx, latest, found)
			require.NoError(err)
			require.Same(found, got)
		})
	}
}

func TestIsAdopted(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		exp         bool
	}{
		{name: "unset"},
		{name: "true", annotations: map[string]string{tcloudcommon.AnnotationKeyAdopted: "true"}, exp: true},
		{name: "mixed case and whitespace", annotations: map[string]string{tcloudcommon.AnnotationKeyAdopted: " TRUE "}, exp: true},
		{name: "false", annotations: map[string]string{tcloudcommon.AnnotationKeyAdopted: "false"}},
		{name: "other value", annotations: map[string]string{tcloudcommon.AnnotationKeyAdopted: "yes"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := &tcloudv1alpha1.Namespace{}
			cr.Annotations = tc.annotations
			require.Equal(t, tc.exp, isAdopted(&res{cr}))
		})
	}
}

func TestResourceID(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		exp         string
	}{
		{name: "unset"},
		{name: "set", annotations: map[string]string{tcloudcommon.AnnotationKeyResourceID: "ns.acct1"}, exp: "ns.acct1"},
		{name: "whitespace is trimmed", annotations: map[string]string{tcloudcommon.AnnotationKeyResourceID: " ns.acct1 "}, exp: "ns.acct1"},
		{name: "blank", annotations: map[string]string{tcloudcommon.AnnotationKeyResourceID: " "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := &tcloudv1alpha1.Namespace{}
			cr.Annotations = tc.annotations
			require.Equal(t, tc.exp, resourceID(&res{cr}))
		})
	}
}
