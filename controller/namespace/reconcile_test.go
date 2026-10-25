package namespace_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
	kttestutilcompare "github.com/temporalio/kube-temporal/pkg/testutil/compare"
	kttestutilkube "github.com/temporalio/kube-temporal/pkg/testutil/kube"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
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
	project := "tmprl-project"
	projectOther := "tmprl-project-other"
	tns := &tcloudv1alpha1.Namespace{}
	tns.Name = tnsName
	tns.Namespace = kubeNamespace
	tns.Spec.Project = project

	cases := []struct {
		name         string
		r            *namespace.Reconciler
		reconOpts    []namespace.NewReconcilerOption
		serverOpts   []fakeserver.NewOption
		getNamespace fakeserver.GetNamespaceFunc
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
			res:    namespace.NewResource(tns),
			expErr: "failed getting tcloud namespace: rpc error: code = Unknown desc = Indy! Bad dates!",
		},
		{
			name: "latest spec is mutated",
			serverOpts: []fakeserver.NewOption{
				fakeserver.OnGetNamespaceReturn(
					&tcloudservicev1.GetNamespaceResponse{
						Namespace: &tcloudnamespacev1.Namespace{
							Spec: &tcloudnamespacev1.NamespaceSpec{
								Name: tnsName,
							},
							ProjectId: projectOther,
						},
					}, nil,
				),
			},
			res: namespace.NewResource(tns),
			latest: func() types.Resource {
				// We verify that if the Temporal Cloud client returns a
				// GetNamespaceResponse containing a different ProjectId than
				// the one on the supplied types.Resource's Spec.Project field,
				// that the supplied types.Resource's Spec.Project field is
				// mutated to match the response from the Temporal Cloud
				// server.
				ns := &tcloudv1alpha1.Namespace{}
				ns.Name = tnsName
				ns.Namespace = kubeNamespace
				ns.Spec.Project = projectOther
				return namespace.NewResource(ns)
			}(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			sopts := tc.serverOpts
			if tc.getNamespace != nil {
				sopts = append(
					sopts, fakeserver.WithGetNamespace(tc.getNamespace),
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
				ropts = append(ropts, tc.reconOpts...)

				r = namespace.NewReconciler(ropts...)
			}

			ctx := context.TODO()

			// We expect the types.Resource supplied to ResourceSetLatest to be
			// mutated, so make a deep copy so we can track any changes made.
			latest := tc.res.DeepCopy()

			err = r.ResourceSetLatest(ctx, latest)
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
