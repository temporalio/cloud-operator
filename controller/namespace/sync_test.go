package namespace_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestReconciler_ResourceSync_CreatesWithResourceIDAnnotation(t *testing.T) {
	require := require.New(t)

	cr := &tcloudv1alpha1.Namespace{}
	cr.Name = "tmprl-ns"
	cr.Namespace = kubeNamespace
	cr.Annotations = map[string]string{
		cloudcommon.AnnotationKeyResourceID: "tmprl-ns.acct1",
	}
	cr.Spec.Replicas = []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}}
	cr.Spec.RetentionDays = 30

	srv, err := fakeserver.New()
	require.NoError(err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(err)

	r := namespace.NewReconciler(base.WithDefaultCloudClient(*tcc))
	r.SetKubeClient(kube.NewFakeClient(cr))
	desired := namespace.NewResource(cr)
	latest := namespace.NewResource(&tcloudv1alpha1.Namespace{
		ObjectMeta: cr.ObjectMeta,
	})

	_, err = r.ResourceSync(context.TODO(), desired, latest)
	require.ErrorContains(err, "failed creating tcloud namespace")
	require.ErrorContains(err, "Unimplemented")
}

func TestReconciler_ResourceSync_NamespaceGone(t *testing.T) {
	t.Run("adopted namespace is not recreated", func(tt *testing.T) {
		require := require.New(tt)

		got, err := syncGoneNamespace(tt, map[string]string{
			cloudcommon.AnnotationKeyAdoptedOn: "2026-11-03T12:00:00Z",
		})

		var terminal *ctrlerrors.TerminalError
		require.ErrorAs(err, &terminal)
		require.ErrorContains(err, "no longer exists")
		require.Equal("my-ns.acct1", got.CR().(*tcloudv1alpha1.Namespace).Status.Resource.ID)
	})

	t.Run("created namespace is recreated", func(tt *testing.T) {
		require := require.New(tt)

		_, err := syncGoneNamespace(tt, nil)

		// The fake server has no CreateNamespace hook, so Unimplemented shows
		// that the create was attempted.
		require.ErrorContains(err, "failed creating tcloud namespace")
		require.ErrorContains(err, "Unimplemented")
	})

	t.Run("partly adopted namespace waits", func(tt *testing.T) {
		require := require.New(tt)

		// Linked by an adoption whose adopted-on was never saved.
		got, err := syncGoneNamespace(tt, map[string]string{
			cloudcommon.AnnotationKeyAdoptionPolicy: "adopt",
			cloudcommon.AnnotationKeyResourceID:     "my-ns.acct1",
		})

		var requeueAfter *requeue.RequeueNeededAfter
		require.ErrorAs(err, &requeueAfter)
		require.ErrorContains(err, `no tcloud namespace with ID "my-ns.acct1" to adopt`)
		require.Equal("my-ns.acct1", got.CR().(*tcloudv1alpha1.Namespace).Status.Resource.ID)
	})
}

// syncGoneNamespace calls ResourceSync for a CR named my-ns, linked to
// my-ns.acct1 and carrying the supplied annotations, whose Namespace is gone
// from Temporal Cloud.
func syncGoneNamespace(t *testing.T, annotations map[string]string) (types.Resource, error) {
	t.Helper()
	require := require.New(t)

	srv, err := fakeserver.New()
	require.NoError(err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(err)
	r := namespace.NewReconciler(base.WithDefaultCloudClient(*tcc))

	cr := &tcloudv1alpha1.Namespace{}
	cr.Name = "my-ns"
	cr.Annotations = annotations
	cr.Spec.Replicas = []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}}
	cr.Spec.RetentionDays = 30
	cr.Status.Resource.ID = "my-ns.acct1"
	desired := namespace.NewResource(cr)

	// ResourceLatest clears the spec and status when the Namespace is not found.
	gone := cr.DeepCopy()
	gone.Spec = tcloudv1alpha1.NamespaceSpec{}
	gone.Status = tcloudv1alpha1.NamespaceStatus{}
	latest := namespace.NewResource(gone)

	return r.ResourceSync(context.TODO(), desired, latest)
}

// activeNamespace returns a Namespace CR named my-ns that an earlier
// reconcile linked to the Active my-ns.acct1, with optional blocks left
// unset as a user would write it.
func activeNamespace() *tcloudv1alpha1.Namespace {
	cr := &tcloudv1alpha1.Namespace{}
	cr.Name = "my-ns"
	cr.Namespace = kubeNamespace
	cr.Spec = tcloudv1alpha1.NamespaceSpec{
		Replicas:      []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-west-2"}},
		RetentionDays: 30,
		APIKeyAuth:    &tcloudv1alpha1.APIKeyAuthSpec{Enabled: true},
		Capacity:      &tcloudv1alpha1.CapacitySpec{OnDemand: &tcloudv1alpha1.OnDemandCapacity{}},
	}
	cr.Status.Resource = cloudcommon.ResourceStatus{
		ID:      "my-ns.acct1",
		Version: "rv-1",
		State:   cloudcommon.ResourceStateActive,
	}
	return cr
}

// syncActiveNamespace calls ResourceSync for the CR from activeNamespace as
// desired, and a copy changed by changeLatest as latest. Cloud calls the
// supplied server options don't handle fail as Unimplemented. It also checks
// that ResourceSync mutated neither desired nor latest.
func syncActiveNamespace(
	t *testing.T,
	changeDesired func(*tcloudv1alpha1.Namespace),
	changeLatest func(*tcloudv1alpha1.Namespace),
	serverOpts ...fakeserver.NewOption,
) error {
	t.Helper()
	require := require.New(t)

	srv, err := fakeserver.New(serverOpts...)
	require.NoError(err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(err)
	r := namespace.NewReconciler(base.WithDefaultCloudClient(*tcc))

	desiredCR := activeNamespace()
	if changeDesired != nil {
		changeDesired(desiredCR)
	}
	latestCR := activeNamespace()
	if changeLatest != nil {
		changeLatest(latestCR)
	}
	desired := namespace.NewResource(desiredCR)
	latest := namespace.NewResource(latestCR)
	desiredBefore := desired.DeepCopy()
	latestBefore := latest.DeepCopy()

	_, err = r.ResourceSync(context.TODO(), desired, latest)

	require.Equal(desiredBefore, desired, "ResourceSync mutated desired")
	require.Equal(latestBefore, latest, "ResourceSync mutated latest")
	return err
}

func TestReconciler_ResourceSync_UpdateSpec(t *testing.T) {
	t.Run("empty optional blocks from cloud send no update", func(tt *testing.T) {
		err := syncActiveNamespace(tt, nil, func(cr *tcloudv1alpha1.Namespace) {
			s := &cr.Spec
			s.MTLSAuth = &tcloudv1alpha1.MTLSAuthSpec{}
			s.CodecServer = &tcloudv1alpha1.CodecServerSpec{
				CustomErrorMessage: &tcloudv1alpha1.CodecServerCustomErrorMessage{
					Default: &tcloudv1alpha1.CodecServerErrorMessage{},
				},
			}
			s.Lifecycle = &cloudcommon.LifecycleSpec{}
			s.HighAvailability = &tcloudv1alpha1.HighAvailabilitySpec{}
			s.Fairness = &tcloudv1alpha1.FairnessSpec{}
			s.EncryptionValidation = &tcloudv1alpha1.EncryptionValidationSpec{}
		})

		require.NoError(tt, err)
	})

	t.Run("project from cloud sends no update", func(tt *testing.T) {
		err := syncActiveNamespace(tt, nil, func(cr *tcloudv1alpha1.Namespace) {
			cr.Spec.Project = &cloudcommon.ResourceReferenceOrID{ID: "project-1"}
		})

		require.NoError(tt, err)
	})

	t.Run("changed retention sends an update", func(tt *testing.T) {
		require := require.New(tt)
		var got *tcloudservicev1.UpdateNamespaceRequest

		err := syncActiveNamespace(tt, nil,
			func(cr *tcloudv1alpha1.Namespace) {
				cr.Spec.RetentionDays = 7
			},
			fakeserver.OnUpdateNamespace(func(req *tcloudservicev1.UpdateNamespaceRequest) {
				got = req
			}, nil),
		)

		var requeueAfter *requeue.RequeueNeededAfter
		require.ErrorAs(err, &requeueAfter)
		require.NotNil(got, "UpdateNamespace not called")
		require.Equal("my-ns.acct1", got.GetNamespace())
		require.Equal("rv-1", got.GetResourceVersion())
		require.EqualValues(30, got.GetSpec().GetRetentionDays())
	})
}

func TestReconciler_ResourceSync_UpdateTags(t *testing.T) {
	cases := []struct {
		name      string
		desired   map[string]string
		latest    map[string]string
		expUpsert map[string]string
		expRemove []string
	}{
		{
			name: "both empty",
		},
		{
			name:    "equal",
			desired: map[string]string{"a": "1", "b": "2"},
			latest:  map[string]string{"a": "1", "b": "2"},
		},
		{
			name:      "adds new tags",
			desired:   map[string]string{"a": "1", "b": "2"},
			latest:    map[string]string{"a": "1"},
			expUpsert: map[string]string{"b": "2"},
		},
		{
			name:      "updates changed values",
			desired:   map[string]string{"a": "new"},
			latest:    map[string]string{"a": "old"},
			expUpsert: map[string]string{"a": "new"},
		},
		{
			name:      "removes missing tags in sorted order",
			desired:   map[string]string{"a": "1"},
			latest:    map[string]string{"a": "1", "c": "3", "b": "2"},
			expRemove: []string{"b", "c"},
		},
		{
			name:      "removes everything when desired is nil",
			latest:    map[string]string{"a": "1"},
			expRemove: []string{"a"},
		},
		{
			name:      "upserts and removes together",
			desired:   map[string]string{"a": "new", "b": "2"},
			latest:    map[string]string{"a": "old", "c": "3"},
			expUpsert: map[string]string{"a": "new", "b": "2"},
			expRemove: []string{"c"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			var got *tcloudservicev1.UpdateNamespaceTagsRequest

			// Tags aren't part of UpdateNamespace, so with only tags
			// differing, any UpdateNamespace call fails as Unimplemented.
			err := syncActiveNamespace(tt,
				func(cr *tcloudv1alpha1.Namespace) { cr.Spec.Tags = tc.desired },
				func(cr *tcloudv1alpha1.Namespace) { cr.Spec.Tags = tc.latest },
				fakeserver.WithUpdateNamespaceTags(func(
					_ context.Context, req *tcloudservicev1.UpdateNamespaceTagsRequest,
				) (*tcloudservicev1.UpdateNamespaceTagsResponse, error) {
					got = req
					return &tcloudservicev1.UpdateNamespaceTagsResponse{}, nil
				}),
			)

			if tc.expUpsert == nil && tc.expRemove == nil {
				require.NoError(err)
				require.Nil(got, "UpdateNamespaceTags called with no tag changes")
				return
			}
			var requeueAfter *requeue.RequeueNeededAfter
			require.ErrorAs(err, &requeueAfter)
			require.NotNil(got, "UpdateNamespaceTags not called")
			require.Equal("my-ns.acct1", got.GetNamespace())
			require.Equal(tc.expUpsert, got.GetTagsToUpsert())
			require.Equal(tc.expRemove, got.GetTagsToRemove())
		})
	}
}

// newSyncReconciler returns a Reconciler that talks to a fake Temporal Cloud
// server configured with serverOpts, and reads custom resources from a fake
// Kubernetes client holding kubeObjs.
func newSyncReconciler(
	t *testing.T,
	kubeObjs []runtime.Object,
	serverOpts ...fakeserver.NewOption,
) *namespace.Reconciler {
	t.Helper()
	srv, err := fakeserver.New(serverOpts...)
	require.NoError(t, err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(t, err)

	kc := kube.NewFakeClient(kubeObjs...)
	r := namespace.NewReconciler(base.WithDefaultCloudClient(*tcc))
	r.SetKubeClient(kc)
	r.SetKubeReader(kc)
	return r
}

// projectCR returns a Project CR named orders
func projectCR(state cloudcommon.ResourceState) *tcloudv1alpha1.Project {
	p := &tcloudv1alpha1.Project{}
	p.Namespace = kubeNamespace
	p.Name = "orders"
	p.Status.Resource.ID = "p-orders"
	p.Status.Resource.State = state
	return p
}

func TestReconciler_ResourceSync_CreateInProject(t *testing.T) {
	projectRef := &cloudcommon.ResourceReferenceOrID{
		Ref: &cloudcommon.ResourceReference{Name: "orders"},
	}

	cases := []struct {
		name         string
		project      *cloudcommon.ResourceReferenceOrID
		kubeObjs     []runtime.Object
		expErr       string
		expCreate    bool
		expProjectID string
	}{
		{
			name:    "fail if the referenced project doesn't exist yet, so it waits",
			project: projectRef,
			expErr:  "spec.project (temporal-cloud-operator/orders)",
		},
		{
			name:     "fail if the referenced project is still activating, so it waits",
			project:  projectRef,
			kubeObjs: []runtime.Object{projectCR(cloudcommon.ResourceStateActivating)},
			expErr:   "spec.project (temporal-cloud-operator/orders)",
		},
		{
			name:         "a project ref yields a create in the referenced project",
			project:      projectRef,
			kubeObjs:     []runtime.Object{projectCR(cloudcommon.ResourceStateActive)},
			expErr:       "created tcloud namespace",
			expCreate:    true,
			expProjectID: "p-orders",
		},
		{
			name:         "a project id yields a create in that project",
			project:      &cloudcommon.ResourceReferenceOrID{ID: "p-legacy"},
			expErr:       "created tcloud namespace",
			expCreate:    true,
			expProjectID: "p-legacy",
		},
		{
			name:      "no project yields a create without a project id",
			expErr:    "created tcloud namespace",
			expCreate: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			r := require.New(tt)
			var gotCreates []*tcloudservicev1.CreateNamespaceRequest
			sr := newSyncReconciler(tt, tc.kubeObjs,
				fakeserver.WithCreateNamespace(func(
					_ context.Context, req *tcloudservicev1.CreateNamespaceRequest,
				) (*tcloudservicev1.CreateNamespaceResponse, error) {
					gotCreates = append(gotCreates, req)
					return &tcloudservicev1.CreateNamespaceResponse{Namespace: "my-ns.acct1"}, nil
				}),
			)
			cr := activeNamespace()
			cr.Status = tcloudv1alpha1.NamespaceStatus{}
			cr.Spec.Project = tc.project
			latest := &tcloudv1alpha1.Namespace{ObjectMeta: cr.ObjectMeta}

			_, err := sr.ResourceSync(
				context.TODO(), namespace.NewResource(cr), namespace.NewResource(latest),
			)

			// Both waiting for the project and a successful create requeue.
			var requeueAfter *requeue.RequeueNeededAfter
			r.ErrorAs(err, &requeueAfter)
			r.ErrorContains(err, tc.expErr)
			if !tc.expCreate {
				r.Empty(gotCreates, "nothing must be created while the project isn't ready")
				return
			}
			r.Len(gotCreates, 1)
			r.Equal(tc.expProjectID, gotCreates[0].GetProjectId())
		})
	}
}

func TestReconciler_ResourceSync_ProjectMismatch(t *testing.T) {
	projectRef := &cloudcommon.ResourceReferenceOrID{
		Ref: &cloudcommon.ResourceReference{Name: "orders"},
	}

	cases := []struct {
		name         string
		project      *cloudcommon.ResourceReferenceOrID
		cloudProject string
		kubeObjs     []runtime.Object
		expErr       string
	}{
		{
			name:         "fail if the referenced project isn't the one temporal cloud reports",
			project:      projectRef,
			cloudProject: "p-default",
			kubeObjs:     []runtime.Object{projectCR(cloudcommon.ResourceStateActive)},
			expErr:       `is in project "p-default" but spec.project is "p-orders"`,
		},
		{
			name:         "fail if the project id isn't the one temporal cloud reports",
			project:      &cloudcommon.ResourceReferenceOrID{ID: "p-legacy"},
			cloudProject: "p-default",
			expErr:       `is in project "p-default" but spec.project is "p-legacy"`,
		},
		{
			name:         "a project ref matching temporal cloud yields no change",
			project:      projectRef,
			cloudProject: "p-orders",
			kubeObjs:     []runtime.Object{projectCR(cloudcommon.ResourceStateActive)},
		},
		{
			name:         "a project ref that isn't ready yields no change",
			project:      projectRef,
			cloudProject: "p-orders",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			r := require.New(tt)
			sr := newSyncReconciler(tt, tc.kubeObjs)
			desired := activeNamespace()
			desired.Spec.Project = tc.project
			latest := desired.DeepCopy()
			latest.Status.References.Project = tc.cloudProject

			_, err := sr.ResourceSync(
				context.TODO(), namespace.NewResource(desired), namespace.NewResource(latest),
			)

			if tc.expErr != "" {
				var terminal *ctrlerrors.TerminalError
				r.ErrorAs(err, &terminal)
				r.ErrorContains(err, tc.expErr)
				return
			}
			r.NoError(err)
		})
	}
}
