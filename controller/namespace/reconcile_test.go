package namespace_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttestutilcompare "github.com/temporalio/kube-temporal/pkg/testutil/compare"
	kttestutilkube "github.com/temporalio/kube-temporal/pkg/testutil/kube"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ctrlrt "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubeNamespace = "temporal-cloud-operator"
)

func TestReconciler_ResourceFromRequest(t *testing.T) {
	tnsName := "tmprl-ns"
	tns := &tcloudv1alpha1.Namespace{}
	tns.Name = tnsName
	tns.Namespace = kubeNamespace

	cases := []struct {
		name   string
		client client.Client
		req    ctrlrt.Request
		exp    types.Resource
		expErr string
	}{
		{
			name: "fail if no client",
			client: func() client.Client {
				return nil
			}(),
			expErr: "nil kube client",
		},
		{
			name: "fail if resource not found",
			client: func() client.Client {
				return kube.NewFakeClient()
			}(),
			req:    kttestutilkube.NewResourceRequest(kubeNamespace, tnsName),
			expErr: "namespaces.cloud.temporal.io \"tmprl-ns\" not found",
		},
		{
			name: "yields resource",
			client: func() client.Client {
				return kube.NewFakeClient(tns)
			}(),
			req: kttestutilkube.NewResourceRequest(kubeNamespace, tnsName),
			exp: namespace.NewResource(tns),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			kc := tc.client
			r := namespace.NewReconciler()
			r.SetKubeClient(kc)
			r.SetKubeReader(kc)

			ctx := context.TODO()

			got, err := r.ResourceFromRequest(ctx, tc.req)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				return
			}

			require.Nil(err)
			if tc.exp != nil {
				require.Equal(tc.exp, got)
			}
		})
	}
}

func TestReconciler_ResourceSetLatest(t *testing.T) {
	tnsName := "tmprl-ns"
	tnsID := tnsName + ".acct1"
	project := "tmprl-project"
	projectOther := "tmprl-project-other"

	// tns has never been synced, so it has no Status.ID and
	// ResourceSetLatest looks the namespace up by name.
	tns := &tcloudv1alpha1.Namespace{}
	tns.Name = tnsName
	tns.Namespace = kubeNamespace
	tns.Spec.Project = project
	tns.Spec.RetentionDays = 7

	// tnsSynced has been synced before, so ResourceSetLatest gets the
	// namespace by its Status.ID.
	tnsSynced := tns.DeepCopy()
	tnsSynced.Status.ID = tnsID
	tnsSynced.Status.ResourceVersion = "rv-0"
	// tnsAdopted is a synced namespace the operator adopted.
	tnsAdopted := tnsSynced.DeepCopy()
	tnsAdopted.Status.Source = cloudcommon.ResourceSourceAdopted

	// cloudNS is the namespace as Temporal Cloud reports it. Its project
	// differs from the CR's so we can see that latest is mutated to match
	// Temporal Cloud.
	cloudNS := &tcloudnamespacev1.Namespace{
		Namespace: tnsID,
		ProjectId: projectOther,
		Tags:      map[string]string{"team": "core"},
		Spec: &tcloudnamespacev1.NamespaceSpec{
			Name:          tnsName,
			Replicas:      []*tcloudnamespacev1.ReplicaSpec{{Region: "aws-us-east-1"}},
			RetentionDays: 30,
		},
		ResourceVersion:  "rv-1",
		State:            tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		AsyncOperationId: "op-1",
	}

	// withLatest returns a copy of the supplied CR as a types.Resource after
	// applying mutate, for building the expected latest resource.
	withLatest := func(
		cr *tcloudv1alpha1.Namespace,
		mutate func(*tcloudv1alpha1.Namespace),
	) types.Resource {
		ns := cr.DeepCopy()
		mutate(ns)
		return namespace.NewResource(ns)
	}
	// mappedFromCloud sets the spec and status that cloudNS maps to.
	mappedFromCloud := func(ns *tcloudv1alpha1.Namespace) {
		ns.Spec = tcloudv1alpha1.NamespaceSpec{
			Project:       projectOther,
			Tags:          map[string]string{"team": "core"},
			Replicas:      []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
			RetentionDays: 30,
		}
		ns.Status.ResourceStatus = cloudcommon.ResourceStatus{
			ID:               tnsID,
			ResourceVersion:  "rv-1",
			State:            cloudcommon.ResourceStateActive,
			AsyncOperationID: "op-1",
		}
	}
	// cleared empties the spec and status, which is what ResourceSetLatest
	// does when the namespace does not exist in Temporal Cloud.
	cleared := func(ns *tcloudv1alpha1.Namespace) {
		ns.Spec = tcloudv1alpha1.NamespaceSpec{}
		ns.Status.ResourceStatus = cloudcommon.ResourceStatus{}
		ns.Status.Source = ""
	}
	// withAnnotation returns a copy of the supplied CR with only the
	// supplied annotation.
	withAnnotation := func(
		ns *tcloudv1alpha1.Namespace,
		key, value string,
	) *tcloudv1alpha1.Namespace {
		out := ns.DeepCopy()
		out.Annotations = map[string]string{key: value}
		return out
	}

	cases := []struct {
		name         string
		r            *namespace.Reconciler
		serverOpts   []fakeserver.NewOption
		asserter     func(*testing.T) fakeserver.GetNamespaceAssertFunc
		listAsserter func(*testing.T) fakeserver.GetNamespacesAssertFunc
		res          types.Resource
		latest       types.Resource
		expErr       string
		// expRequeue is whether the error is a requeue. Requeues from
		// adoption return before latest is changed, so latest must equal res.
		expRequeue bool
	}{
		{
			name:   "fail nil cloud connection manager or default client",
			expErr: "connection manager nil and no default client set",
			r:      namespace.NewReconciler(), // imitate no conn mgr
			res:    namespace.NewResource(tns),
		},
		{
			name: "fail if tcloud get namespace errors",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespaceReturn(
					nil, fmt.Errorf("Indy! Bad dates!"),
				),
			},
			res:    namespace.NewResource(tnsSynced),
			expErr: "failed getting tcloud namespace: rpc error: code = Unknown desc = Indy! Bad dates!",
		},
		{
			name: "get namespace request uses status id",
			asserter: func(tt *testing.T) fakeserver.GetNamespaceAssertFunc {
				assert := assert.New(tt)
				return func(req *tcloudservicev1.GetNamespaceRequest) {
					assert.Equal(tnsID, req.GetNamespace())
				}
			},
			res: namespace.NewResource(tnsSynced),
		},
		{
			name: "latest is mutated from get namespace",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespaceReturn(
					&tcloudservicev1.GetNamespaceResponse{Namespace: cloudNS}, nil,
				),
			},
			res:    namespace.NewResource(tnsSynced),
			latest: withLatest(tnsSynced, mappedFromCloud),
		},
		{
			// The namespace was deleted outside of the operator, so latest is
			// cleared and ResourceSync recreates it.
			name: "latest is cleared if get namespace is not found",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespaceReturn(
					nil, status.Error(codes.NotFound, "namespace not found"),
				),
			},
			res:    namespace.NewResource(tnsSynced),
			latest: withLatest(tnsSynced, cleared),
		},
		{
			// The adopted namespace was deleted outside of the operator.
			// ResourceSync recreates it, so it's no longer the adopted one.
			name: "source is cleared if get namespace is not found",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespaceReturn(
					nil, status.Error(codes.NotFound, "namespace not found"),
				),
			},
			res:    namespace.NewResource(tnsAdopted),
			latest: withLatest(tnsAdopted, cleared),
		},
		{
			name: "source is kept if get namespace finds it",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespaceReturn(
					&tcloudservicev1.GetNamespaceResponse{Namespace: cloudNS}, nil,
				),
			},
			res:    namespace.NewResource(tnsAdopted),
			latest: withLatest(tnsAdopted, mappedFromCloud),
		},
		{
			name: "fail if tcloud list namespaces errors",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespacesReturn(
					nil, fmt.Errorf("Indy! Bad dates!"),
				),
			},
			res:    namespace.NewResource(tns),
			expErr: "failed listing tcloud namespaces: rpc error: code = Unknown desc = Indy! Bad dates!",
		},
		{
			name: "list namespaces request uses cr name",
			listAsserter: func(tt *testing.T) fakeserver.GetNamespacesAssertFunc {
				assert := assert.New(tt)
				return func(req *tcloudservicev1.GetNamespacesRequest) {
					assert.Equal(tnsName, req.GetName())
				}
			},
			res: namespace.NewResource(tns),
		},
		{
			name: "latest is mutated from namespace found by name",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespacesReturn(
					&tcloudservicev1.GetNamespacesResponse{
						Namespaces: []*tcloudnamespacev1.Namespace{
							{
								Namespace: "other-ns.acct1",
								Spec:      &tcloudnamespacev1.NamespaceSpec{Name: "other-ns"},
							},
							cloudNS,
						},
					}, nil,
				),
			},
			res:    namespace.NewResource(tns),
			latest: withLatest(tns, mappedFromCloud),
		},
		{
			name: "latest is cleared if not found by name",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespacesReturn(
					&tcloudservicev1.GetNamespacesResponse{}, nil,
				),
			},
			res:    namespace.NewResource(tns),
			latest: withLatest(tns, cleared),
		},
		{
			name: "adopt with nothing found requeues and doesn't create",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespacesReturn(
					&tcloudservicev1.GetNamespacesResponse{}, nil,
				),
			},
			res: namespace.NewResource(withAnnotation(
				tns, cloudcommon.AnnotationKeyAdoptionPolicy, "adopt",
			)),
			expErr:     `no tcloud namespace named "tmprl-ns" to adopt`,
			expRequeue: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			sopts := tc.serverOpts

			if tc.asserter != nil {
				sopts = append(
					sopts,
					fakeserver.OnGetNamespaceAssert(tc.asserter(tt)),
				)
			}
			if tc.listAsserter != nil {
				sopts = append(
					sopts,
					fakeserver.OnGetNamespacesAssert(tc.listAsserter(tt)),
				)
			}

			srv, err := fakeserver.New(sopts...)
			require.Nil(err)
			stop := srv.Start()
			tt.Cleanup(stop)

			tcc, err := srv.Client()
			require.Nil(err)

			r := tc.r
			if r == nil {
				ropts := []namespace.NewReconcilerOption{
					namespace.WithDefaultCloudClient(*tcc),
					namespace.WithKubeClient(
						kube.NewFakeClient(tns),
					),
				}

				r = namespace.NewReconciler(ropts...)
			}

			ctx := context.TODO()

			// We expect the types.Resource supplied to ResourceSetLatest to be
			// mutated, so make a deep copy so we can track any changes made.
			latest := tc.res.DeepCopy()

			err = r.ResourceSetLatest(ctx, latest)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				if tc.expRequeue {
					var requeueAfter *requeue.RequeueNeededAfter
					require.True(
						errors.As(err, &requeueAfter),
						"expected requeue error, got %v", err,
					)
					// Returned before latest was changed, so no status.id is
					// saved and ResourceSync isn't called.
					require.True(
						kttestutilcompare.EqualIgnoringFields(
							tc.res, latest,
							kttestutilkube.ResourceIgnoredFields...,
						),
						kttestutilcompare.DiffIgnoringFields(
							tc.res, latest,
							kttestutilkube.ResourceIgnoredFields...,
						),
					)
				}
				return
			}

			require.Nil(err)
			if tc.latest != nil {
				require.True(
					kttestutilcompare.EqualIgnoringFields(
						tc.latest, latest,
						kttestutilkube.ResourceIgnoredFields...,
					),
					kttestutilcompare.DiffIgnoringFields(
						tc.latest, latest,
						kttestutilkube.ResourceIgnoredFields...,
					),
				)
			}
		})
	}
}

// TestReconciler_ResourceSetLatest_AdoptTwoPasses runs the adopt policy on a
// name-only CR: the first pass copies Temporal Cloud's settings into the spec
// without linking, and the second links it with no Temporal Cloud update.
func TestReconciler_ResourceSetLatest_AdoptTwoPasses(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()

	// The user writes only the name and the adopt annotation.
	cr := &tcloudv1alpha1.Namespace{}
	cr.Name = "tmprl-ns"
	cr.Namespace = kubeNamespace
	cr.Annotations = map[string]string{
		cloudcommon.AnnotationKeyAdoptionPolicy: "adopt",
	}

	cloudNS := &tcloudnamespacev1.Namespace{
		Namespace: "tmprl-ns.acct1",
		ProjectId: "tmprl-project",
		Tags:      map[string]string{"team": "core"},
		Spec: &tcloudnamespacev1.NamespaceSpec{
			Name:          "tmprl-ns",
			Replicas:      []*tcloudnamespacev1.ReplicaSpec{{Region: "aws-us-east-1"}},
			RetentionDays: 30,
			ApiKeyAuth:    &tcloudnamespacev1.ApiKeyAuthSpec{Enabled: true},
			SearchAttributes: map[string]tcloudnamespacev1.NamespaceSpec_SearchAttributeType{
				"CustomerId": tcloudnamespacev1.NamespaceSpec_SEARCH_ATTRIBUTE_TYPE_KEYWORD,
			},
		},
		ResourceVersion: "rv-1",
		State:           tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
	}

	// Only listing is implemented. Create, update and tag calls return
	// Unimplemented, so ResourceSync below fails if adopting makes one.
	srv, err := fakeserver.New(
		fakeserver.OnGetNamespacesReturn(
			&tcloudservicev1.GetNamespacesResponse{
				Namespaces: []*tcloudnamespacev1.Namespace{cloudNS},
			}, nil,
		),
	)
	require.NoError(err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(err)

	kc := kube.NewFakeClient(cr)
	r := namespace.NewReconciler(
		namespace.WithDefaultCloudClient(*tcc),
		namespace.WithKubeClient(kc),
	)
	// readCR returns the CR as stored, with its resourceVersion, as
	// ResourceFromRequest would.
	readCR := func() *tcloudv1alpha1.Namespace {
		got := &tcloudv1alpha1.Namespace{}
		require.NoError(kc.Get(ctx, client.ObjectKeyFromObject(cr), got))
		return got
	}

	// Pass 1 copies Temporal Cloud's settings into the spec, without
	// project, and doesn't link.
	desired := namespace.NewResource(readCR())
	latest := desired.DeepCopy()
	err = r.ResourceSetLatest(ctx, latest)
	var requeueAfter *requeue.RequeueNeededAfter
	require.True(errors.As(err, &requeueAfter), "expected requeue error, got %v", err)
	require.True(
		kttestutilcompare.EqualIgnoringFields(
			desired, latest,
			kttestutilkube.ResourceIgnoredFields...,
		),
		kttestutilcompare.DiffIgnoringFields(
			desired, latest,
			kttestutilkube.ResourceIgnoredFields...,
		),
	)

	stored := readCR()
	require.Equal(tcloudv1alpha1.NamespaceSpec{
		Tags:          map[string]string{"team": "core"},
		Replicas:      []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
		RetentionDays: 30,
		APIKeyAuth:    &tcloudv1alpha1.APIKeyAuthSpec{Enabled: true},
		SearchAttributes: map[string]tcloudv1alpha1.SearchAttributeType{
			"CustomerId": tcloudv1alpha1.SearchAttributeTypeKeyword,
		},
	}, stored.Spec)
	require.Empty(stored.Status.ID)

	// Pass 2 links it and marks it adopted.
	desired = namespace.NewResource(stored)
	latest = desired.DeepCopy()
	require.NoError(r.ResourceSetLatest(ctx, latest))
	got := latest.CR().(*tcloudv1alpha1.Namespace)
	require.Equal("tmprl-ns.acct1", got.Status.ID)
	require.Equal(cloudcommon.ResourceSourceAdopted, got.Status.Source)

	// The specs match, so the sync makes no Temporal Cloud calls.
	require.NoError(r.ResourceSync(ctx, desired, latest))
}
