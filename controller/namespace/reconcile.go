package namespace

import (
	"context"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
	ctrlrt "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	tcloudv1alpha1 "github.com/temporalio/resource-operator/api/cloud/v1alpha1"
	"github.com/temporalio/resource-operator/pkg/reconciler/base"
)

// NewrOption modifies the Reconciler returned from NewReconciler.
type NewReconcilerOption func(*Reconciler)

// WithDefaultCloudConnectionSpec registers the supplied
// [tcloudv1alpha1.ConnectionSpec] object as the default cloud connection spec
// for the Reconciler.
func WithDefaultCloudConnectionSpec(
	spec tcloudv1alpha1.ConnectionSpec,
) NewReconcilerOption {
	return func(r *Reconciler) {
		r.SetDefaultCloudConnectionSpec(spec)
	}
}

// WithDefaultCloudClient registers the supplied [tcclient.Clinet] as the
// default cloud client fallback for the Reconciler.
func WithDefaultCloudClient(tcc tcclient.Client) NewReconcilerOption {
	return func(r *Reconciler) {
		r.SetDefaultCloudClient(tcc)
	}
}

// WithKubeClient sets the supplied Kubernetes client for the Reconciler.
func WithKubeClient(kc kclient.Client) NewReconcilerOption {
	return func(r *Reconciler) {
		r.SetKubeClient(kc)
	}
}

// NewReconciler returns a new [types.Reconciler] that can Reconciler
// [ttcloudv1alpha1.Namespace] resources.
func NewReconciler(opts ...NewReconcilerOption) *Reconciler {
	r := Reconciler{}
	for _, opt := range opts {
		opt(&r)
	}
	return &r
}

// Reconciler wraps the base Reconciler and implements the [types.Reconciler]
// interface.
type Reconciler struct {
	base.Reconciler
}

// AdaptBuilder returns a [*ctrlrt.Builder] adapted from the supplied
// [*ctrlrt.Builder], typically calling the [ctrlrt.Builder.For] method with
// the Reconciler's managed Kind prototype.
func (r *Reconciler) AdaptBuilder(builder *ctrlrt.Builder) *ctrlrt.Builder {
	return builder.For(&tcloudv1alpha1.Namespace{})
}

// ResourceFromRequest returns a Resource representing the requested Kubernetes
// namespaced object.
func (r *Reconciler) ResourceFromRequest(
	ctx context.Context,
	req ctrlrt.Request,
) (types.Resource, error) {
	cr := &tcloudv1alpha1.Namespace{}
	err := r.KubeReadLatestInto(ctx, req.NamespacedName, cr)
	if err != nil {
		return nil, fmt.Errorf("failed reading latest resource: %w", err)
	}
	return &res{cr}, nil
}

// ResourceSetLatest mutates the supplied [types.Resource] with the latest
// observed state.
//
// Any error returned from this function is considered a runtime error and will
// cause the resource to contain a Stalled Condition with Status=True.
func (r *Reconciler) ResourceSetLatest(
	ctx context.Context,
	latest types.Resource, // mutated by this method
) error {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.set.latest")
	defer func() {
		exit(err)
	}()

	tcc, err := r.CloudClientFor(ctx, latest)
	if err != nil {
		return fmt.Errorf("failed getting tcloud client: %w", err)
	}
	_ = tcc

	// "pop" the underlying *tcloudv1alpha1.Namespace from the [types.Resource]
	// wrapper. This is a pointer so by modifying it, you mutate the supplied
	// `latest` parameter.
	ns := latest.(*res).cr
	_ = ns

	// TODO: call the Temporal Cloud API to fetch Namespace spec and status and
	// mutate the supplied resource to set the spec and status fields
	// accordingly.
	//
	// latestObserved, err := tcc.CloudService().GetNamespace(...)
	// if err != nil {
	//     return fmt.Errorf("failed getting tcloud namespace: %w", err)
	// }
	// ns.Spec.ProjectID = latestObserved.ProjectId
	// ...

	return err
}

// ResourceSyncFunc synchronizes any managed resources to match the supplied
// desired [types.Resource] state.
//
// Returns nil when the synchronization was successful, otherwise returns an
// error derived from `kube-tempoal/pkg/controller/errors.ErrResourceSync`.
//
// `desired` should NEVER be mutated by this callback.
// `latest` should NEVER be mutated by this callback.
//
// This method is only called when there was a difference detected between
// the desired and latest [types.Resource].
func (r *Reconciler) ResourceSync(
	ctx context.Context,
	desired types.Resource,
	latest types.Resource,
) error {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.sync")
	defer func() {
		exit(err)
	}()

	tcc, err := r.CloudClientFor(ctx, desired)
	if err != nil {
		return fmt.Errorf("failed getting tcloud client: %w", err)
	}
	_ = tcc

	// TODO: call the Temporal Cloud API to update Namespace desired
	// configuration to match the desired resource.

	return err
}

// ResourceDelete deletes any managed external resources for the supplied
// [types.Resource]. Note that the Reconciler does NOT need to remove the
// finalizer on the associated [client.Object]. The Controller handles that.
// All the Reconciler needs to do is delete any owned resources or
// externally-managed resources associated with the supplied [types.Resource].
func (r *Reconciler) ResourceDelete(
	ctx context.Context,
	desired types.Resource,
) error {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.delete")
	defer func() {
		exit(err)
	}()

	tcc, err := r.CloudClientFor(ctx, desired)
	if err != nil {
		return fmt.Errorf("failed getting tcloud client: %w", err)
	}
	_ = tcc

	// TODO: call the Temporal Cloud API to delete the Namespace.

	return err
}

// compile-time check that Reconciler satisfies the [types.Reconciler]
// contract.
var _ types.Reconciler = (*Reconciler)(nil)
