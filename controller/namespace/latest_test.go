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
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
	kttestutilcompare "github.com/temporalio/kube-temporal/pkg/testutil/compare"
	kttestutilkube "github.com/temporalio/kube-temporal/pkg/testutil/kube"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
				r = namespace.NewReconciler(base.WithDefaultCloudClient(*tcc))
				r.SetKubeClient(kube.NewFakeClient(tns))
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

func TestReconciler_ResourceLatest_Adopt(t *testing.T) {
	t.Run("found by resource-id", func(tt *testing.T) {
		require := require.New(tt)
		var gotID string

		latest, err := adoptLatest(tt, func(
			_ context.Context, req *tcloudservicev1.GetNamespaceRequest,
		) (*tcloudservicev1.GetNamespaceResponse, error) {
			gotID = req.GetNamespace()
			return &tcloudservicev1.GetNamespaceResponse{
				Namespace: &tcloudnamespacev1.Namespace{
					Namespace: "my-ns.acct1",
					Spec:      &tcloudnamespacev1.NamespaceSpec{Name: "my-ns"},
				},
			}, nil
		})

		require.NoError(err)
		require.Equal("my-ns.acct1", gotID)
		require.Equal("my-ns.acct1", latest.CR().(*tcloudv1alpha1.Namespace).Status.Resource.ID)
	})

	t.Run("resource-id not found", func(tt *testing.T) {
		require := require.New(tt)
		var gotID string

		_, err := adoptLatest(tt, func(
			_ context.Context, req *tcloudservicev1.GetNamespaceRequest,
		) (*tcloudservicev1.GetNamespaceResponse, error) {
			gotID = req.GetNamespace()
			return nil, status.Error(codes.NotFound, "namespace not found")
		})

		require.Equal("my-ns.acct1", gotID)
		require.ErrorContains(err, `no tcloud namespace with ID "my-ns.acct1" to adopt`)
	})
}

// adoptLatest calls ResourceLatest for a CR named my-ns that asks to adopt
// my-ns.acct1
func adoptLatest(
	t *testing.T,
	getNamespace fakeserver.GetNamespaceFunc,
) (types.Resource, error) {
	t.Helper()
	require := require.New(t)

	srv, err := fakeserver.New(fakeserver.WithGetNamespace(getNamespace))
	require.NoError(err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(err)
	r := namespace.NewReconciler(base.WithDefaultCloudClient(*tcc))

	cr := &tcloudv1alpha1.Namespace{}
	cr.Name = "my-ns"
	cr.Annotations = map[string]string{
		cloudcommon.AnnotationKeyAdoptionPolicy: "adopt",
		cloudcommon.AnnotationKeyResourceID:     "my-ns.acct1",
	}

	return r.ResourceLatest(context.TODO(), namespace.NewResource(cr))
}

func TestReconciler_ResourceLatest_FindByName(t *testing.T) {
	// namespaceNamed returns a Temporal Cloud Namespace with the supplied
	// (unqualified) name.
	namespaceNamed := func(name string) *tcloudnamespacev1.Namespace {
		return &tcloudnamespacev1.Namespace{
			Namespace: name + ".acct1",
			Spec:      &tcloudnamespacev1.NamespaceSpec{Name: name},
		}
	}

	cases := []struct {
		name          string
		pages         []*tcloudservicev1.GetNamespacesResponse
		listErr       error
		expID         string
		expErr        string
		expPageTokens []string
	}{
		{
			name: "found on first page",
			pages: []*tcloudservicev1.GetNamespacesResponse{
				{Namespaces: []*tcloudnamespacev1.Namespace{namespaceNamed("my-ns")}},
			},
			expID:         "my-ns.acct1",
			expPageTokens: []string{""},
		},
		{
			name: "found on a later page",
			pages: []*tcloudservicev1.GetNamespacesResponse{
				{NextPageToken: "p2"},
				{
					Namespaces:    []*tcloudnamespacev1.Namespace{namespaceNamed("my-ns")},
					NextPageToken: "p3",
				},
				{},
			},
			expID:         "my-ns.acct1",
			expPageTokens: []string{"", "p2"},
		},
		{
			name: "ignores names that are not an exact match",
			pages: []*tcloudservicev1.GetNamespacesResponse{
				{Namespaces: []*tcloudnamespacev1.Namespace{
					namespaceNamed("my-ns-2"),
					namespaceNamed("my-ns"),
				}},
			},
			expID:         "my-ns.acct1",
			expPageTokens: []string{""},
		},
		{
			name: "cleared when no page matches",
			pages: []*tcloudservicev1.GetNamespacesResponse{
				{
					Namespaces:    []*tcloudnamespacev1.Namespace{namespaceNamed("other")},
					NextPageToken: "p2",
				},
				{Namespaces: []*tcloudnamespacev1.Namespace{namespaceNamed("my-ns-2")}},
			},
			expPageTokens: []string{"", "p2"},
		},
		{
			name:          "cleared when there are no namespaces",
			pages:         []*tcloudservicev1.GetNamespacesResponse{{}},
			expPageTokens: []string{""},
		},
		{
			name:          "returns client errors",
			listErr:       errors.New("boom"),
			expErr:        "failed listing tcloud namespaces",
			expPageTokens: []string{""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			// Record each request's name and page token before serving it.
			var names, tokens []string
			pager := fakeserver.GetNamespacesPages(tc.pages...)
			srv, err := fakeserver.New(fakeserver.WithGetNamespaces(func(
				ctx context.Context, req *tcloudservicev1.GetNamespacesRequest,
			) (*tcloudservicev1.GetNamespacesResponse, error) {
				names = append(names, req.GetName())
				tokens = append(tokens, req.GetPageToken())
				if tc.listErr != nil {
					return nil, tc.listErr
				}
				return pager(ctx, req)
			}))
			require.NoError(err)
			tt.Cleanup(srv.Start())
			tcc, err := srv.Client()
			require.NoError(err)
			r := namespace.NewReconciler(base.WithDefaultCloudClient(*tcc))

			// The CR has never been synced, so ResourceLatest looks it up by name.
			cr := &tcloudv1alpha1.Namespace{}
			cr.Name = "my-ns"
			cr.Namespace = kubeNamespace
			cr.Spec.RetentionDays = 7

			latest, err := r.ResourceLatest(context.TODO(), namespace.NewResource(cr))

			require.Equal(tc.expPageTokens, tokens)
			for _, name := range names {
				require.Equal("my-ns", name)
			}
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				require.ErrorContains(err, "boom")
				return
			}
			require.NoError(err)
			got := latest.CR().(*tcloudv1alpha1.Namespace)
			require.Equal(tc.expID, got.Status.Resource.ID)
			if tc.expID == "" {
				require.Equal(tcloudv1alpha1.NamespaceSpec{}, got.Spec, "latest spec not cleared")
			}
		})
	}
}
