package namespace

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/resource/base"
	"github.com/temporalio/cloud-operator/pkg/types"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
)

var (
	gvk = tcloudv1alpha1.SchemeGroupVersion.WithKind("Namespace")
)

// res wraps a Namespace CR and implements the types.Resource
// interface.
type res struct {
	base.Resource
	// cr is the Kubernetes-native custom resource.
	cr *tcloudv1alpha1.Namespace
}

// CR returns the underlying custom resource.
func (r res) CR() any {
	return r.cr
}

// DeepCopy returns a copy of the resource
func (r res) DeepCopy() kttypes.Resource {
	crCopy := r.cr.DeepCopy()

	// NOTE(jaypipes): we can safely ignore the error returned from base.New()
	// since we know our CR struct pointer always implements client.Object and
	// runtime.Object.
	b, _ := base.New(gvk, crCopy)

	return &res{
		Resource: *b,
		cr:       crCopy,
	}
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

// NewResource returns the [types.Resource] wrapping the supplied
// [tcloudv1alpha1.Namespace] object.
func NewResource(cr *tcloudv1alpha1.Namespace) types.Resource {
	// NOTE(jaypipes): we can safely ignore the error returned from base.New()
	// since we know our CR struct pointer always implements client.Object and
	// runtime.Object.
	b, _ := base.New(gvk, cr)
	return &res{
		Resource: *b,
		cr:       cr,
	}
}
