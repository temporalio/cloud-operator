package namespace

import (
	"context"
	"fmt"
	"slices"
	"time"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
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
	"github.com/temporalio/cloud-operator/pkg/types"
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
) (kttypes.Resource, error) {
	cr := &tcloudv1alpha1.Namespace{}
	err := r.KubeReadLatestInto(ctx, req.NamespacedName, cr)
	if err != nil {
		return nil, fmt.Errorf("failed reading latest resource: %w", err)
	}
	return NewResource(cr), nil
}

// ResourceLatest returns a copy of the supplied [Resource] with the latest
// observed state of the externally-managed resource.
//
// `current` is NEVER mutated.
//
// Any error returned from this function is considered a terminal error and
// will cause the resource to contain a Stalled Condition with Status=True.
func (r *Reconciler) ResourceLatest(
	ctx context.Context,
	current kttypes.Resource, // mutated by this method
) (kttypes.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.latest")
	defer func() {
		exit(err)
	}()

	// Make a copy of the current resource. We will modify and return this
	// copy, ensuring that the current parameter is never mutated.
	latest := current.DeepCopy().(types.Resource)

	tcc, err := r.CloudClientFor(ctx, current)
	if err != nil {
		return latest, fmt.Errorf("failed getting tcloud client: %w", err)
	}

	// "pop" the underlying *tcloudv1alpha1.Namespace from the [types.Resource]
	// wrapper. This is a pointer so by modifying it, you mutate latest which
	// is what is returned from this function.
	cr := latest.CR()
	ns := cr.(*tcloudv1alpha1.Namespace)
	// resourceID is status.resource.id if an earlier reconcile linked the CR,
	// otherwise the cloud.temporal.io/resource-id annotation.
	resourceID := latest.ResourceID()

	// current is the CR as read, so it says whether an earlier reconcile
	// linked it and whether it asks to be adopted.
	cur := current.(types.Resource)

	var tns *tcloudnamespacev1.Namespace
	switch {
	case cur.PreviouslyReconciled():
		// Linked before: get the namespace by status.resource.id
		tns, err = getNamespaceByID(ctx, tcc, resourceID)
		if err != nil {
			return latest, fmt.Errorf("failed getting tcloud namespace: %w", err)
		}
	case cur.AdoptionPolicy() == cloudcommon.AdoptionPolicyAdopt:
		// Not linked yet and asks to be adopted: get the namespace named by
		// the resource-id annotation.
		if resourceID != "" {
			tns, err = getNamespaceByID(ctx, tcc, resourceID)
			if err != nil {
				return latest, fmt.Errorf("failed getting tcloud namespace %q to adopt: %w", resourceID, err)
			}
		}
		// Nothing to adopt. Without an error the controller would see no
		// difference from the annotations-only CR and report success.
		if tns == nil {
			err = fmt.Errorf("no tcloud namespace with ID %q to adopt", resourceID)
			return latest, err
		}
	default:
		// The qualified namespace name is only known once a reconcile after
		// the create has saved it to status, and status can be lost (e.g. the
		// CR was recreated). Look the namespace up by name so that a duplicate
		// is never created.
		tns, err = findNamespaceByName(ctx, tcc, ns.Name)
		if err != nil {
			return latest, fmt.Errorf("failed listing tcloud namespaces: %w", err)
		}
	}

	if tns == nil {
		// The namespace does not exist in Temporal Cloud, so mutate latest
		// spec and status to be empty.
		ns.Spec = tcloudv1alpha1.NamespaceSpec{}
		ns.Status = tcloudv1alpha1.NamespaceStatus{}
		return latest, nil
	}

	// Construct a new Namespace from the proto we got back for the Cloud
	// resource and mutate the latest types.Resource by dereferencing the cr
	// variable to the value of the Namespace returned from FromProto()
	fromCloud := ns.FromProto(tns)
	*ns = *fromCloud

	return latest, nil
}

// ResourceSync synchronizes any managed resources to match the supplied
// desired [Resource] state.
//
// Returns a copy of latest updated with any annotations or status fields that
// ResourceSync may have changed.
//
// IMPORTANT: When an error is returned, the returned Resource copy may have
// been updated to include a condition that the user may inspect to determine
// how to proceed with fixing a broken Resource. The caller should always save
// the metadata and status of the returned Resource back to the Kubernetes API
// server regardless of whether an error was returned.
//
// `desired` is NEVER mutated.
// `latest` is NEVER mutated.
//
// This method is only called when there was a difference detected between the
// desired and latest [Resource].
func (r *Reconciler) ResourceSync(
	ctx context.Context,
	desired kttypes.Resource,
	latest kttypes.Resource,
) (kttypes.Resource, error) {
	return r.syncNamespace(
		ctx,
		desired.(types.Resource),
		latest.(types.Resource),
	)
}

// syncNamespace makes at most one Temporal Cloud call to move the namespace
// toward the desired state. Create, update and tag changes each start an
// async operation, and a namespace with an operation in flight rejects new
// ones, so after each call we requeue and let the next reconcile observe the
// result and take the next step.
func (r *Reconciler) syncNamespace(
	ctx context.Context,
	desired types.Resource,
	latest types.Resource,
) (kttypes.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.sync")
	defer func() {
		exit(err)
	}()

	// Adoption comes before the create branch, so a CR asking to adopt an
	// existing Namespace never creates one.
	if wantsAdoption(desired) {
		var adopted kttypes.Resource
		adopted, err = adoptNamespace(desired, latest, time.Now())
		return adopted, err
	}

	// Make a copy of the latest resource. We may modify and return this copy,
	// ensuring that neither the desired nor latest parameter is ever mutated.
	updated := latest.DeepCopy()

	tcc, err := r.CloudClientFor(ctx, desired)
	if err != nil {
		return updated, fmt.Errorf("failed getting tcloud client: %w", err)
	}

	resourceID := latest.ResourceID()

	if latest.ResourceStatus().ID == "" {

		// The operator doesn't recreate a Namespace it adopted
		if desired.IsAdopted() {
			err = ctrlerrors.NewTerminalError(fmt.Errorf(
				"%w: adopted tcloud namespace %q no longer exists in Temporal Cloud; "+
					"the operator doesn't recreate adopted namespaces, "+
					"delete this CR to stop managing it",
				ctrlerrors.ErrResourceSync, desired.ResourceID(),
			))
			return resetLatestStatus(desired, latest), err
		}

		cr := desired.CR()
		ns := cr.(*tcloudv1alpha1.Namespace)
		tns := ns.ToProto()
		resp, err := tcc.CreateNamespace(ctx, &tcloudservicev1.CreateNamespaceRequest{
			Spec:      tns.GetSpec(),
			Tags:      tns.GetTags(),
			ProjectId: tns.GetProjectId(),
		})
		if err != nil {
			return updated, cloudCallError("creating tcloud namespace", err)
		}
		return updated, requeue.NeededAfter(
			fmt.Errorf("created tcloud namespace %q, waiting for activation", resp.GetNamespace()),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	desiredNS := desired.CR().(*tcloudv1alpha1.Namespace)
	latestNS := latest.CR().(*tcloudv1alpha1.Namespace)

	// If the CR specifies a project ID, it must match the project reported
	// by Temporal Cloud. An unset project does not need this check.
	if p := desiredNS.Spec.Project; p != nil && p.ID != "" && p.ID != latestNS.Status.References.Project {
		return updated, ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: tcloud namespace %q is in project %q "+
					"but spec.project.id is %q",
				ctrlerrors.ErrResourceSync, resourceID,
				latestNS.Status.References.Project, p.ID,
			),
		)
	}

	// Only an Active namespace accepts new operations. Every other state
	// either resolves on its own, so we wait, or needs someone to step in, so
	// we stop. Unknown states also wait rather than fall through to an update.
	resourceState := latest.ResourceState()
	switch resourceState {
	case cloudcommon.ResourceStateActive:
	case cloudcommon.ResourceStateActivating, cloudcommon.ResourceStateUpdating:
		return updated, requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud namespace %q to leave state %s",
				resourceID, resourceState,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	case cloudcommon.ResourceStateDeleting, cloudcommon.ResourceStateDeleted:
		// The namespace was deleted outside of the operator. Once Temporal
		// Cloud reports it as not found, ResourceSetLatest clears the status
		// and the next sync recreates it.
		return updated, requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud namespace %q to finish deleting, state %s",
				resourceID, resourceState,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	case cloudcommon.ResourceStateActivationFailed,
		cloudcommon.ResourceStateUpdateFailed,
		cloudcommon.ResourceStateDeleteFailed,
		cloudcommon.ResourceStateSuspended,
		cloudcommon.ResourceStateExpired:
		return updated, ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: tcloud namespace %q is in state %s, "+
					"contact Temporal Cloud support",
				ctrlerrors.ErrResourceSync, resourceID, resourceState,
			),
		)
	default:
		return updated, requeue.NeededAfter(
			fmt.Errorf(
				"waiting for tcloud namespace %q to leave unknown state %q",
				resourceID, resourceState,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	// The controller's DeepEqual always reports a difference
	// once Status.Conditions is populated, so compare the fields each API
	// owns here to avoid sending no-op updates on every reconcile. Project
	// and tags are not part of UpdateNamespace.
	if !equality.Semantic.DeepEqual(
		specWithoutProjectAndTags(&desiredNS.Spec),
		specWithoutProjectAndTags(&latestNS.Spec),
	) {
		_, err := tcc.UpdateNamespace(ctx, &tcloudservicev1.UpdateNamespaceRequest{
			Namespace:       resourceID,
			Spec:            desiredNS.ToProto().GetSpec(),
			ResourceVersion: latestNS.Status.Resource.Version,
		})
		if err != nil {
			return updated, cloudCallError("updating tcloud namespace", err)
		}
		return updated, requeue.NeededAfter(
			fmt.Errorf("updated tcloud namespace %q, waiting for update to complete", resourceID),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	upsert, remove := tagsDiff(desiredNS.Spec.Tags, latestNS.Spec.Tags)
	if len(upsert) > 0 || len(remove) > 0 {
		_, err := tcc.UpdateNamespaceTags(ctx, &tcloudservicev1.UpdateNamespaceTagsRequest{
			Namespace:    resourceID,
			TagsToUpsert: upsert,
			TagsToRemove: remove,
		})
		if err != nil {
			return updated, cloudCallError("updating tcloud namespace tags", err)
		}
		return updated, requeue.NeededAfter(
			fmt.Errorf(
				"updated tcloud namespace %q tags, "+
					"waiting for update to complete",
				resourceID,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	return updated, nil
}

// specWithoutProjectAndTags returns a copy of the supplied spec with the
// fields that UpdateNamespace does not manage cleared.
func specWithoutProjectAndTags(
	spec *tcloudv1alpha1.NamespaceSpec,
) *tcloudv1alpha1.NamespaceSpec {
	out := spec.DeepCopy()
	out.Project = &cloudcommon.ResourceReferenceOrID{
		ID: "",
	}
	out.Tags = nil
	if cs := out.CodecServer; cs != nil && cs.CustomErrorMessage != nil {
		cs.CustomErrorMessage.Default = nilIfEmpty(cs.CustomErrorMessage.Default)
		cs.CustomErrorMessage = nilIfEmpty(cs.CustomErrorMessage)
	}
	out.CodecServer = nilIfEmpty(out.CodecServer)
	out.MTLSAuth = nilIfEmpty(out.MTLSAuth)
	out.Lifecycle = nilIfEmpty(out.Lifecycle)
	out.HighAvailability = nilIfEmpty(out.HighAvailability)
	out.Fairness = nilIfEmpty(out.Fairness)
	out.EncryptionValidation = nilIfEmpty(out.EncryptionValidation)
	return out
}

// nilIfEmpty returns nil if p points to a zero value, otherwise p.
func nilIfEmpty[T any](p *T) *T {
	var zero T
	if p != nil && equality.Semantic.DeepEqual(*p, zero) {
		return nil
	}
	return p
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

// getNamespaceByID returns the Temporal Cloud Namespace with the supplied ID
func getNamespaceByID(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	id string,
) (*tcloudnamespacev1.Namespace, error) {
	resp, err := tcc.GetNamespace(ctx, &tcloudservicev1.GetNamespaceRequest{
		Namespace: id,
	})

	if status.Code(err) == codes.NotFound {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return resp.GetNamespace(), nil
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
var _ kttypes.Reconciler = (*Reconciler)(nil)
