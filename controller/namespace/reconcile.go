package namespace

import (
	"context"
	"fmt"
	"slices"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/equality"
	ctrlrt "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
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

	// "pop" the underlying *tcloudv1alpha1.Namespace from the [types.Resource]
	// wrapper. This is a pointer so by modifying it, you mutate the supplied
	// `latest` parameter.
	cr := latest.(*res).cr

	var tns *tcloudnamespacev1.Namespace
	if cr.Status.ID == "" {
		// The qualified namespace name is only known once a reconcile after
		// the create has saved it to status, and status can be lost (e.g. the
		// CR was recreated). Look the namespace up by name so that a duplicate
		// is never created.
		tns, err = findNamespaceByName(ctx, tcc, cr.Name)
		if err != nil {
			return fmt.Errorf("failed listing tcloud namespaces: %w", err)
		}
	} else {
		var resp *tcloudservicev1.GetNamespaceResponse
		resp, err = tcc.GetNamespace(ctx, &tcloudservicev1.GetNamespaceRequest{
			Namespace: cr.Status.ID,
		})
		if status.Code(err) == codes.NotFound {
			// The namespace was deleted outside of the operator. Treat it
			// the same as one that was never created so ResourceSync
			// recreates it.
			err = nil
		} else if err != nil {
			return fmt.Errorf("failed getting tcloud namespace: %w", err)
		}
		tns = resp.GetNamespace()
	}

	if tns == nil {
		// The namespace does not exist in Temporal Cloud, so mutate latest
		// spec and status to be empty.
		cr.Spec = tcloudv1alpha1.NamespaceSpec{}
		cr.Status.ResourceStatus = cloudcommon.ResourceStatus{}
		return nil
	}

	cr.FromProto(tns)

	// cr := latest.(*res).cr
	// _ = cr

	// // TODO: call the Temporal Cloud API to fetch Namespace spec and status and
	// // mutate the supplied resource to set the spec and status fields
	// // accordingly.
	// req := tcloudservicev1.GetNamespaceRequest{
	// 	Namespace: cr.Name,
	// }
	// resp, err := tcc.GetNamespace(ctx, &req)
	// if err != nil {
	// 	return fmt.Errorf("failed getting tcloud namespace: %w", err)
	// }
	// ns := resp.GetNamespace()
	// cr.Spec.Project = ns.GetProjectId()

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

	err = syncNamespace(ctx, tcc, desired.(*res).cr, latest.(*res).cr)
	return err
}

// syncNamespace makes at most one Temporal Cloud call to move the namespace
// toward the desired state. Create, update and tag changes each start an
// async operation, and a namespace with an operation in flight rejects new
// ones, so after each call we requeue and let the next reconcile observe the
// result and take the next step.
func syncNamespace(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.Namespace,
	latest *tcloudv1alpha1.Namespace,
) error {
	if latest.Status.ID == "" {
		tns := desired.ToProto()
		resp, err := tcc.CreateNamespace(ctx, &tcloudservicev1.CreateNamespaceRequest{
			Spec:      tns.GetSpec(),
			Tags:      tns.GetTags(),
			ProjectId: tns.GetProjectId(),
		})
		if err != nil {
			return cloudCallError("creating tcloud namespace", err)
		}
		return requeue.NeededAfter(
			fmt.Errorf("created tcloud namespace %q, waiting for activation", resp.GetNamespace()),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	// Only an Active namespace accepts new operations. Every other state
	// either resolves on its own, so we wait, or needs someone to step in, so
	// we stop. Unknown states also wait rather than fall through to an update.
	id := latest.Status.ID
	switch state := latest.Status.State; state {
	case cloudcommon.ResourceStateActive:
	case cloudcommon.ResourceStateActivating, cloudcommon.ResourceStateUpdating:
		return requeue.NeededAfter(
			fmt.Errorf("waiting for tcloud namespace %q to leave state %s", id, state),
			requeue.DefaultRequeueAfterDuration,
		)
	case cloudcommon.ResourceStateDeleting, cloudcommon.ResourceStateDeleted:
		// The namespace was deleted outside of the operator. Once Temporal
		// Cloud reports it as not found, ResourceSetLatest clears the status
		// and the next sync recreates it.
		return requeue.NeededAfter(
			fmt.Errorf("waiting for tcloud namespace %q to finish deleting, state %s", id, state),
			requeue.DefaultRequeueAfterDuration,
		)
	case cloudcommon.ResourceStateActivationFailed,
		cloudcommon.ResourceStateUpdateFailed,
		cloudcommon.ResourceStateDeleteFailed,
		cloudcommon.ResourceStateSuspended,
		cloudcommon.ResourceStateExpired:
		return ctrlerrors.NewTerminalError(fmt.Errorf(
			"%w: tcloud namespace %q is in state %s, contact Temporal Cloud support",
			ctrlerrors.ErrResourceSync, id, state,
		))
	default:
		return requeue.NeededAfter(
			fmt.Errorf("waiting for tcloud namespace %q to leave unknown state %q", id, state),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	// spec.project is immutable on the CR, so a mismatch means the namespace
	// was moved outside of the operator. An empty spec.project means the
	// account's default project, whose ID Temporal Cloud reports back.
	if desired.Spec.Project != "" && desired.Spec.Project != latest.Spec.Project {
		return ctrlerrors.NewTerminalError(fmt.Errorf(
			"%w: tcloud namespace %q is in project %q but spec.project is %q",
			ctrlerrors.ErrResourceSync, id, latest.Spec.Project, desired.Spec.Project,
		))
	}

	// The controller's DeepEqual always reports a difference
	// once Status.Conditions is populated, so compare the fields each API
	// owns here to avoid sending no-op updates on every reconcile. Project
	// and tags are not part of UpdateNamespace.
	if !equality.Semantic.DeepEqual(
		specWithoutProjectAndTags(&desired.Spec),
		specWithoutProjectAndTags(&latest.Spec),
	) {
		_, err := tcc.UpdateNamespace(ctx, &tcloudservicev1.UpdateNamespaceRequest{
			Namespace:       id,
			Spec:            desired.ToProto().GetSpec(),
			ResourceVersion: latest.Status.ResourceVersion,
		})
		if err != nil {
			return cloudCallError("updating tcloud namespace", err)
		}
		return requeue.NeededAfter(
			fmt.Errorf("updated tcloud namespace %q, waiting for update to complete", id),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	upsert, remove := tagsDiff(desired.Spec.Tags, latest.Spec.Tags)
	if len(upsert) > 0 || len(remove) > 0 {
		_, err := tcc.UpdateNamespaceTags(ctx, &tcloudservicev1.UpdateNamespaceTagsRequest{
			Namespace:    id,
			TagsToUpsert: upsert,
			TagsToRemove: remove,
		})
		if err != nil {
			return cloudCallError("updating tcloud namespace tags", err)
		}
		return requeue.NeededAfter(
			fmt.Errorf("updated tcloud namespace %q tags, waiting for update to complete", id),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	return nil
}

// specWithoutProjectAndTags returns a copy of the supplied spec with the
// fields that UpdateNamespace does not manage cleared.
func specWithoutProjectAndTags(
	spec *tcloudv1alpha1.NamespaceSpec,
) *tcloudv1alpha1.NamespaceSpec {
	out := spec.DeepCopy()
	out.Project = ""
	out.Tags = nil
	return out
}

// cloudCallError wraps an error returned from a Temporal Cloud call in
// [ctrlerrors.ErrResourceSync] and classifies it for the controller.
func cloudCallError(action string, err error) error {
	wrapped := fmt.Errorf("%w: failed %s: %w", ctrlerrors.ErrResourceSync, action, err)
	switch status.Code(err) {
	case codes.InvalidArgument:
		// The request itself is bad; retrying will not help until the spec
		// changes.
		return ctrlerrors.NewTerminalError(wrapped)
	case codes.AlreadyExists, codes.FailedPrecondition, codes.Aborted:
		// Our view of the namespace is stale (e.g. an outdated resource
		// version, or a create that ResourceSetLatest has not observed yet).
		// The next reconcile re-reads it.
		return requeue.NeededAfter(wrapped, requeue.DefaultRequeueAfterDuration)
	}
	return wrapped
}

// findNamespaceByName returns the Temporal Cloud Namespace with the supplied
// (unqualified) name, or nil if there is none. Namespace names are unique
// within an account.
func findNamespaceByName(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	name string,
) (*tcloudnamespacev1.Namespace, error) {
	req := &tcloudservicev1.GetNamespacesRequest{Name: name}
	for {
		resp, err := tcc.GetNamespaces(ctx, req)
		if err != nil {
			return nil, err
		}
		// Compare names ourselves rather than relying on the filter being an
		// exact match.
		for _, tns := range resp.GetNamespaces() {
			if tns.GetSpec().GetName() == name {
				return tns, nil
			}
		}
		if resp.GetNextPageToken() == "" {
			return nil, nil
		}
		req.PageToken = resp.GetNextPageToken()
	}
}

// tagsDiff returns the tags that must be upserted and the tag keys that must
// be removed to turn the latest tags into the desired tags.
func tagsDiff(
	desired map[string]string,
	latest map[string]string,
) (map[string]string, []string) {
	var upsert map[string]string
	for k, v := range desired {
		if lv, ok := latest[k]; !ok || lv != v {
			if upsert == nil {
				upsert = map[string]string{}
			}
			upsert[k] = v
		}
	}
	var remove []string
	for k := range latest {
		if _, ok := desired[k]; !ok {
			remove = append(remove, k)
		}
	}
	slices.Sort(remove)
	return upsert, remove
}

// compile-time check that Reconciler satisfies the [types.Reconciler]
// contract.
var _ types.Reconciler = (*Reconciler)(nil)
