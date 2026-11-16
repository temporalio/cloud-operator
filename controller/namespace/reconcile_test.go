package namespace_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	kttestutilkube "github.com/temporalio/kube-temporal/pkg/testutil/kube"
	"github.com/temporalio/kube-temporal/pkg/types"
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
