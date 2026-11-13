package namespace_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
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

func TestReconciler_ResourceLatest(t *testing.T) {
	tnsName := "tmprl-ns"
	tnsID := tnsName + ".acct1"
	project := "tmprl-project"
	projectOther := "tmprl-project-other"

	// tns has never been synced, so it has no Status.ID and
	// ResourceSetLatest looks the namespace up by name.
	tns := &tcloudv1alpha1.Namespace{}
	tns.Name = tnsName
	tns.Namespace = kubeNamespace
	tns.Spec.Project = &cloudcommon.ResourceReferenceOrID{
		ID: project,
	}
	tns.Spec.RetentionDays = 7

	// tnsSynced has been synced before, so ResourceSetLatest gets the
	// namespace by its Status.ID.
	tnsSynced := tns.DeepCopy()
	tnsSynced.Status.Resource.ID = tnsID
	tnsSynced.Status.Resource.Version = "rv-0"
	tnsSynced.Status.References = tcloudv1alpha1.NamespaceReferences{
		Project: project,
	}

	// protoOutOfSync is the namespace as Temporal Cloud reports it. Its project
	// differs from the CR's so we can see that latest is mutated to match
	// Temporal Cloud.
	protoOutOfSync := &tcloudnamespacev1.Namespace{
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

	nsOutOfSync := tns.DeepCopy()
	nsOutOfSync.Spec = tcloudv1alpha1.NamespaceSpec{
		// NOTE(jaypipes): The spec's project is never mutated once set.
		// Instead, Status.Cloud.References.Project is set to the value
		// returned from Temporal Cloud API.
		Project: &cloudcommon.ResourceReferenceOrID{
			ID: project,
		},
		Tags:          map[string]string{"team": "core"},
		Replicas:      []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}},
		RetentionDays: 30,
	}
	nsOutOfSync.Status = tcloudv1alpha1.NamespaceStatus{
		Resource: cloudcommon.ResourceStatus{
			ID:               tnsID,
			Version:          "rv-1",
			State:            cloudcommon.ResourceStateActive,
			AsyncOperationID: "op-1",
		},
		References: tcloudv1alpha1.NamespaceReferences{
			Project: projectOther,
		},
	}

	// cleared empties the spec and status, which is what ResourceSetLatest
	// does when the namespace does not exist in Temporal Cloud.
	cleared := tns.DeepCopy()
	cleared.Spec = tcloudv1alpha1.NamespaceSpec{}
	cleared.Status = tcloudv1alpha1.NamespaceStatus{
		Resource: cloudcommon.ResourceStatus{},
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
					&tcloudservicev1.GetNamespaceResponse{Namespace: protoOutOfSync}, nil,
				),
			},
			res: namespace.NewResource(tnsSynced),
			latest: func() types.Resource {
				return namespace.NewResource(nsOutOfSync)
			}(),
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
			res: namespace.NewResource(tnsSynced),
			latest: func() types.Resource {
				return namespace.NewResource(cleared)
			}(),
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
			name: "resource-id annotation without status lists by cr name",
			listAsserter: func(tt *testing.T) fakeserver.GetNamespacesAssertFunc {
				assert := assert.New(tt)
				return func(req *tcloudservicev1.GetNamespacesRequest) {
					assert.Equal(tnsName, req.GetName())
				}
			},
			res: namespace.NewResource(func() *tcloudv1alpha1.Namespace {
				out := tns.DeepCopy()
				out.Annotations = map[string]string{
					cloudcommon.AnnotationKeyResourceID: tnsID,
				}
				return out
			}()),
		},
		{
			name: "latest cr is mutated from namespace found by name",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespacesReturn(
					&tcloudservicev1.GetNamespacesResponse{
						Namespaces: []*tcloudnamespacev1.Namespace{
							{
								Namespace: "other-ns.acct1",
								Spec:      &tcloudnamespacev1.NamespaceSpec{Name: "other-ns"},
							},
							protoOutOfSync,
						},
					}, nil,
				),
			},
			res: namespace.NewResource(tns),
			latest: func() types.Resource {
				return namespace.NewResource(nsOutOfSync)
			}(),
		},
		{
			name: "The latest cr is cleared if it's not found by name",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespacesReturn(
					&tcloudservicev1.GetNamespacesResponse{}, nil,
				),
			},
			res: namespace.NewResource(tns),
			latest: func() types.Resource {
				return namespace.NewResource(cleared)
			}(),
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

			latest, err := r.ResourceLatest(ctx, tc.res)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
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

	r := namespace.NewReconciler(
		namespace.WithDefaultCloudClient(*tcc),
		namespace.WithKubeClient(kube.NewFakeClient(cr)),
	)
	desired := namespace.NewResource(cr)
	latest := namespace.NewResource(&tcloudv1alpha1.Namespace{
		ObjectMeta: cr.ObjectMeta,
	})

	_, err = r.ResourceSync(context.TODO(), desired, latest)
	require.ErrorContains(err, "failed creating tcloud namespace")
	require.ErrorContains(err, "Unimplemented")
}

func TestReconciler_ResourceLatest_Adopt(t *testing.T) {
	cases := []struct {
		name   string
		getErr error
		expErr string
	}{
		{
			name: "found by resource-id",
		},
		{
			name:   "resource-id not found",
			getErr: status.Error(codes.NotFound, "namespace not found"),
			expErr: `no tcloud namespace with ID "my-ns.acct1" to adopt`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			var gotID string

			srv, err := fakeserver.New(
				fakeserver.WithGetNamespace(func(
					_ context.Context, req *tcloudservicev1.GetNamespaceRequest,
				) (*tcloudservicev1.GetNamespaceResponse, error) {
					gotID = req.GetNamespace()
					if tc.getErr != nil {
						return nil, tc.getErr
					}
					return &tcloudservicev1.GetNamespaceResponse{
						Namespace: &tcloudnamespacev1.Namespace{
							Namespace: "my-ns.acct1",
							Spec:      &tcloudnamespacev1.NamespaceSpec{Name: "my-ns"},
						},
					}, nil
				}),
			)
			require.NoError(err)
			tt.Cleanup(srv.Start())
			tcc, err := srv.Client()
			require.NoError(err)
			r := namespace.NewReconciler(namespace.WithDefaultCloudClient(*tcc))

			cr := &tcloudv1alpha1.Namespace{}
			cr.Name = "my-ns"
			cr.Annotations = map[string]string{
				cloudcommon.AnnotationKeyAdoptionPolicy: "adopt",
				cloudcommon.AnnotationKeyResourceID:     "my-ns.acct1",
			}

			latest, err := r.ResourceLatest(context.TODO(), namespace.NewResource(cr))
			require.Equal("my-ns.acct1", gotID)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				return
			}
			require.NoError(err)
			require.Equal("my-ns.acct1", latest.CR().(*tcloudv1alpha1.Namespace).Status.Resource.ID)
		})
	}
}

// missingNamespaceOutcome is what ResourceSync is expected to do when a linked
// Namespace is gone from Temporal Cloud.
type missingNamespaceOutcome int

const (
	// stalls: a terminal error, nothing is created, the old ID is kept in status.
	stalls missingNamespaceOutcome = iota
	// recreates: CreateNamespace is called.
	recreates
)

func TestReconciler_ResourceSync_NamespaceGone(t *testing.T) {
	cases := []struct {
		name    string
		adopted bool
		exp     missingNamespaceOutcome
	}{
		{
			name:    "adopted namespace is not recreated",
			adopted: true,
			exp:     stalls,
		},
		{
			name: "created namespace is recreated",
			exp:  recreates,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			srv, err := fakeserver.New()
			require.NoError(err)
			tt.Cleanup(srv.Start())
			tcc, err := srv.Client()
			require.NoError(err)
			r := namespace.NewReconciler(namespace.WithDefaultCloudClient(*tcc))

			cr := &tcloudv1alpha1.Namespace{}
			cr.Name = "my-ns"
			cr.Spec.Replicas = []tcloudv1alpha1.ReplicaSpec{{Region: "aws-us-east-1"}}
			cr.Spec.RetentionDays = 30
			cr.Status.Resource.ID = "my-ns.acct1"
			if tc.adopted {
				cr.Annotations = map[string]string{
					cloudcommon.AnnotationKeyAdoptedOn: "2026-11-03T12:00:00Z",
				}
			}
			desired := namespace.NewResource(cr)

			// ResourceLatest clears the spec and status when the Namespace is not found.
			gone := cr.DeepCopy()
			gone.Spec = tcloudv1alpha1.NamespaceSpec{}
			gone.Status = tcloudv1alpha1.NamespaceStatus{}
			latest := namespace.NewResource(gone)

			got, err := r.ResourceSync(context.TODO(), desired, latest)

			switch tc.exp {
			case stalls:
				var terminal *ctrlerrors.TerminalError
				require.ErrorAs(err, &terminal)
				require.ErrorContains(err, "no longer exists")
				require.Equal("my-ns.acct1", got.CR().(*tcloudv1alpha1.Namespace).Status.Resource.ID)
			case recreates:
				require.ErrorContains(err, "failed creating tcloud namespace")
				require.ErrorContains(err, "Unimplemented")
			}
		})
	}
}
