package namespace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/equality"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/ptr"
	"github.com/temporalio/cloud-operator/pkg/reconciler/adoption"
	"github.com/temporalio/cloud-operator/pkg/resource/reference"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceSync makes at most one Temporal Cloud call to move the Namespace
// toward the `desired` state. Create, update and tag changes each start an
// async operation, and a Namespace with an operation in flight rejects new
// ones, so after each call it requeues and lets the next reconcile observe the
// result and take the next step. It returns a copy of `latest` with any
// status changes.
//
// IMPORTANT: When an error is returned, the returned Resource copy may have
// been updated to include a condition that the user may inspect to determine
// how to proceed with fixing a broken Resource. The caller should always save
// the metadata and status of the returned Resource back to the Kubernetes API
// server regardless of whether an error was returned.
//
// `desired` is NEVER mutated.
// `latest` is NEVER mutated.
func (r *Reconciler) ResourceSync(
	ctx context.Context,
	desiredRes kttypes.Resource,
	latestRes kttypes.Resource,
) (kttypes.Resource, error) {
	desired := desiredRes.(types.Resource)
	latest := latestRes.(types.Resource)

	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.sync")
	defer func() {
		exit(err)
	}()

	if adoption.Wanted(desired) {
		var adopted kttypes.Resource
		adopted, err = adoption.Adopt(desired, latest, resourceKind, time.Now(), checkAdoptName)
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
		if err = adoption.CheckNotRecreated(desired, resourceKind); err != nil {
			return adoption.Unchanged(desired), err
		}

		cr := desired.CR()
		ns := cr.(*tcloudv1alpha1.Namespace)
		projectID, err := r.resolveProjectID(ctx, ns)
		if errors.Is(err, reference.ErrNotReady) {
			check := reference.NewCheck(ns.Namespace)
			check.Pending("spec.project", *ns.Spec.Project, "")
			return updated, check.Err()
		}
		if err != nil {
			return updated, err
		}
		tns := ns.ToProto()
		resp, err := tcc.CreateNamespace(ctx, &tcloudservicev1.CreateNamespaceRequest{
			Spec:      tns.GetSpec(),
			Tags:      tns.GetTags(),
			ProjectId: projectID,
		})
		if err != nil {
			// Temporal Cloud returns NotFound for a project ID that doesn't
			// exist, and PermissionDenied when projects aren't enabled for the
			// account. The user has to fix spec.project or the account, so
			// retrying won't help.
			if code := status.Code(err); projectID != "" &&
				(code == codes.NotFound || code == codes.PermissionDenied) {
				return updated, ctrlerrors.NewTerminalError(fmt.Errorf(
					"%w: failed creating tcloud namespace in project %q: %w",
					ctrlerrors.ErrResourceSync, projectID, err,
				))
			}
			return updated, cloud.CallError("creating tcloud namespace", err)
		}
		return updated, requeue.NeededAfter(
			fmt.Errorf("created tcloud namespace %q, waiting for activation", resp.GetNamespace()),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	desiredNS := desired.CR().(*tcloudv1alpha1.Namespace)
	latestNS := latest.CR().(*tcloudv1alpha1.Namespace)
	updatedNS := updated.CR().(*tcloudv1alpha1.Namespace)

	// A move can't be cancelled, so a spec change waits for the move in
	// progress to finish.
	if m := latestNS.Status.ProjectMove; m != nil && m.Failure == "" {
		err = checkProjectMove(ctx, tcc, resourceID, updatedNS)
		return updated, err
	}

	// moveTo is the project the Namespace must move to, or empty when it is
	// already in the project spec.project refers to. An unset project never
	// moves the Namespace. A ref that can't be resolved anymore, such as one
	// to a deleted Project CR, leaves the Namespace where it is.
	projectID, resolveErr := r.resolveProjectID(ctx, desiredNS)
	if resolveErr != nil && !errors.Is(resolveErr, reference.ErrNotReady) {
		return updated, resolveErr
	}
	var moveTo string
	if projectID != "" && projectID != latestNS.Status.References.Project {
		moveTo = projectID
	}
	if moveTo == "" {
		updatedNS.Status.ProjectMove = nil
	} else if err = checkMoveAllowed(resourceID, moveTo, desiredNS, latestNS); err != nil {
		return updated, err
	}

	// Only an Active namespace accepts new operations.
	if err = cloud.RequireActive(resourceKind, resourceID, latest.ResourceState()); err != nil {
		return updated, err
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
			return updated, cloud.CallError("updating tcloud namespace", err)
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
			return updated, cloud.CallError("updating tcloud namespace tags", err)
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

	// The move runs after the update so that clearing spec.connectivityRuleIDs
	// and changing spec.project in one edit removes the rules before the move.
	if moveTo != "" {
		err = moveNamespace(ctx, tcc, resourceID, moveTo, desiredNS, latestNS, updatedNS)
		return updated, err
	}

	return updated, nil
}

// resolveProjectID returns the Temporal Cloud ID of the Project in the CR's
// spec, or an empty string when the spec has none. The error wraps
// [reference.ErrNotReady] while a referenced Project isn't Active yet.
func (r *Reconciler) resolveProjectID(
	ctx context.Context,
	cr *tcloudv1alpha1.Namespace,
) (string, error) {
	if cr.Spec.Project == nil {
		return "", nil
	}
	return reference.ResolveProjectID(ctx, r.KubeReader(), cr.Namespace, *cr.Spec.Project)
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
		cs.CustomErrorMessage.Default = ptr.NilIfEmpty(cs.CustomErrorMessage.Default)
		cs.CustomErrorMessage = ptr.NilIfEmpty(cs.CustomErrorMessage)
	}
	out.CodecServer = ptr.NilIfEmpty(out.CodecServer)
	out.MTLSAuth = ptr.NilIfEmpty(out.MTLSAuth)
	out.Lifecycle = ptr.NilIfEmpty(out.Lifecycle)
	out.HighAvailability = ptr.NilIfEmpty(out.HighAvailability)
	out.Fairness = ptr.NilIfEmpty(out.Fairness)
	out.EncryptionValidation = ptr.NilIfEmpty(out.EncryptionValidation)
	return out
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
