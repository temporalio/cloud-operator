package namespace

import (
	"log/slog"

	"github.com/temporalio/kube-temporal/pkg/kube/transform"
	"github.com/temporalio/kube-temporal/pkg/resource"
	"github.com/temporalio/kube-temporal/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tcloudv1alpha1 "github.com/temporalio/resource-operator/api/cloud/v1alpha1"
)

// res wraps a Namespace CR and implements the types.Resource
// interface.
type res struct {
	// cr is the Kubernetes-native custom resource.
	cr *tcloudv1alpha1.Namespace
}

// IsBeingDeleted returns true if the Kubernetes resource has a non-zero
// deletion timestamp
func (r res) IsBeingDeleted() bool {
	return !r.cr.DeletionTimestamp.IsZero()
}

// ClientObject returns the Kubernetes controller-runtime Client representation
// of the Resource
func (r res) ClientObject() client.Object {
	return r.cr
}

// RuntimeObject returns the Kubernetes apimachinery/runtime representation of
// the Resource
func (r res) RuntimeObject() runtime.Object {
	return r.cr
}

// GroupVersionKind returns the GroupVersionKind that the Resource
// represents.
func (r res) GroupVersionKind() schema.GroupVersionKind {
	return tcloudv1alpha1.SchemeGroupVersion.WithKind("Namespace")
}

// Conditions returns the Resource's collection of Conditions from its status
// field. Satisfies the `pkg/condition.Manager` interface.
func (r res) Conditions() []*metav1.Condition {
	return r.cr.Status.Conditions
}

// ReplaceConditions replaces the Resource's collection of Conditions with the
// supplied collection. Satisfies the `pkg/condition.Manager` interface.
func (r *res) ReplaceConditions(conditions []*metav1.Condition) {
	r.cr.Status.Conditions = conditions
}

// DeepCopy returns a copy of the resource
func (r res) DeepCopy() types.Resource {
	crCopy := r.cr.DeepCopy()
	return &res{crCopy}
}

// Unstructured returns the resource as a [unstructured.Unstructured]
func (r res) Unstructured() (*unstructured.Unstructured, error) {
	return transform.ToUnstructured(r.RuntimeObject())
}

// LogValue implements [slog.LogValuer] to format the Resource as a group.
func (r *res) LogValue() slog.Value {
	return resource.LogValue(r)
}

// NewResource returns the [types.Resource] wrapping the supplied
// [tcloudv1alpha1.Namespace] object.
func NewResource(cr *tcloudv1alpha1.Namespace) types.Resource {
	return &res{cr}
}
