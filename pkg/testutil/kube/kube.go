package kube

import (
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	kttestutilkube "github.com/temporalio/kube-temporal/pkg/testutil/kube"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = tcloudv1alpha1.AddToScheme(scheme)
}

// NewFakeClient returns a controller-runtime Client populated with the
// supplied objects.
func NewFakeClient(objs ...runtime.Object) client.Client {
	builder := kttestutilkube.NewFakeClientBuilder()
	return builder.WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
}
