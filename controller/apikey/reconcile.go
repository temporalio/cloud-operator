package apikey

import (
	"context"
	"fmt"

	"github.com/temporalio/kube-temporal/pkg/types"
	ctrlrt "sigs.k8s.io/controller-runtime"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
)

// NewReconciler returns a new [types.Reconciler] that can reconcile
// [tcloudv1alpha1.APIKey] resources.
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
	// TODO(tomba): watch ServiceAccount custom resources here as well, so that
	// an API Key whose owner is a ServiceAccount custom resource is reconciled
	// as soon as that ServiceAccount becomes Active, rather than on the next
	// requeue.
	return builder.For(&tcloudv1alpha1.APIKey{})
}

// ResourceFromRequest returns a Resource representing the requested Kubernetes
// namespaced object.
func (r *Reconciler) ResourceFromRequest(
	ctx context.Context,
	req ctrlrt.Request,
) (types.Resource, error) {
	cr := &tcloudv1alpha1.APIKey{}
	err := r.KubeReadLatestInto(ctx, req.NamespacedName, cr)
	if err != nil {
		return nil, fmt.Errorf("failed reading latest resource: %w", err)
	}
	return NewResource(cr), nil
}

var _ types.Reconciler = (*Reconciler)(nil)
