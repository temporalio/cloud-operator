package project

import (
	"context"
	"fmt"

	"github.com/temporalio/kube-temporal/pkg/types"
	ctrlrt "sigs.k8s.io/controller-runtime"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
)

// NewReconciler returns a new [types.Reconciler] that can reconcile
// [tcloudv1alpha1.Project] resources.
func NewReconciler(opts ...base.NewOption) *Reconciler {
	r := Reconciler{}
	for _, opt := range opts {
		opt(&r.Reconciler)
	}
	return &r
}

// Reconciler wraps the base Reconciler and implements the [types.Reconciler]
// interface.
type Reconciler struct {
	base.Reconciler
}

// AdaptBuilder returns a [*ctrlrt.Builder] adapted from the supplied
// [*ctrlrt.Builder], calling the [ctrlrt.Builder.For] method with the
// Reconciler's managed Kind prototype.
func (r *Reconciler) AdaptBuilder(builder *ctrlrt.Builder) *ctrlrt.Builder {
	return builder.For(&tcloudv1alpha1.Project{})
}

// ResourceFromRequest returns a Resource representing the requested Kubernetes
// namespaced object.
func (r *Reconciler) ResourceFromRequest(
	ctx context.Context,
	req ctrlrt.Request,
) (types.Resource, error) {
	cr := &tcloudv1alpha1.Project{}
	err := r.KubeReadLatestInto(ctx, req.NamespacedName, cr)
	if err != nil {
		return nil, fmt.Errorf("failed reading latest resource: %w", err)
	}
	return NewResource(cr), nil
}

var _ types.Reconciler = (*Reconciler)(nil)
