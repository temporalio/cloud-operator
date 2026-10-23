package namespace_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/testutil"
	kttestutil "github.com/temporalio/kube-temporal/pkg/testutil"
	"github.com/temporalio/kube-temporal/pkg/types"
	ctrlrt "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubeNamespace = "kube-temporal"
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
				return testutil.NewFakeClient()
			}(),
			req:    kttestutil.NewResourceRequest(tnsName, kubeNamespace),
			expErr: "namespaces.cloud.temporal.io \"kube-temporal\" not found",
		},
		{
			name: "yields resource",
			client: func() client.Client {
				return testutil.NewFakeClient(tns)
			}(),
			req: kttestutil.NewResourceRequest(kubeNamespace, tnsName),
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
				require.EqualExportedValues(tc.exp, got)
			}
		})
	}
}

func TestReconciler_ResourceSetLatest(t *testing.T) {
	tnsName := "tmprl-ns"
	tns := &tcloudv1alpha1.Namespace{}
	tns.Name = tnsName
	tns.Namespace = kubeNamespace

	tcc, err := testutil.NewFakeCloudClient()
	require.Nil(t, err)

	cases := []struct {
		name   string
		r      *namespace.Reconciler
		res    types.Resource
		expErr string
	}{
		{
			name:   "fail nil cloud connection manager or default client",
			r:      namespace.NewReconciler(),
			res:    namespace.NewResource(tns),
			expErr: "connection manager nil and no default client set",
		},
		{
			name: "mutates latest resource",
			r: namespace.NewReconciler(
				namespace.WithKubeClient(
					testutil.NewFakeClient(tns),
				),
				namespace.WithDefaultCloudClient(*tcc),
			),
			res: namespace.NewResource(tns),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			ctx := context.TODO()

			err := tc.r.ResourceSetLatest(ctx, tc.res)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				return
			}

			require.Nil(err)
		})
	}
}
