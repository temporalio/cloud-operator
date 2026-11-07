package namespace

import (
	"context"
	"fmt"
	"strings"

	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/annotation"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

// adoptNamespace decides whether the Namespace found by the resource-id
// annotation can be linked to latest, which has the adoption-policy
// annotation and no status.id yet. It returns the Namespace to link, or an
// error that says why it can't be linked yet.
//
// The adopt policy takes two reconciles. The first copies Temporal Cloud's
// settings into the CR's spec, sets the adopted annotation and requeues. The
// second links the Namespace once the specs match, so status.id is never saved
// next to an empty spec and ResourceSync has nothing to update.
func (r *Reconciler) adoptNamespace(
	ctx context.Context,
	latest types.Resource,
	found *tcloudnamespacev1.Namespace,
) (*tcloudnamespacev1.Namespace, error) {
	cr := latest.CR().(*tcloudv1alpha1.Namespace)

	_, err := adoptionPolicy(latest)
	if err != nil {
		// Annotation edits don't trigger a reconcile, so requeue to pick up
		// a corrected value.
		return nil, requeue.NeededAfter(
			fmt.Errorf("%w. Fix or remove the %s annotation", err, tcloudcommon.AnnotationKeyAdoptionPolicy),
			requeue.DefaultRequeueAfterDuration,
		)
	}
	id := resourceID(latest)
	if id == "" {
		return nil, requeue.NeededAfter(
			fmt.Errorf(
				"the %s annotation needs the %s annotation set to the ID of the tcloud namespace to adopt",
				tcloudcommon.AnnotationKeyAdoptionPolicy, tcloudcommon.AnnotationKeyResourceID,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	var state tcloudcommon.ResourceState
	state.FromProto(found.GetState())
	if found == nil || state == tcloudcommon.ResourceStateDeleted {
		return nil, requeue.NeededAfter(
			fmt.Errorf("no tcloud namespace with ID %q to adopt", id),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	// The CR's name must be the Namespace's name
	if name := found.GetSpec().GetName(); name != cr.Name {
		return nil, requeue.NeededAfter(
			fmt.Errorf(
				"can't adopt tcloud namespace %q: its name %q doesn't match the CR's name %q",
				id, name, cr.Name,
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	// Deleting, failed, suspended, expired and unknown states
	// namespaces can not be adopted
	switch state {
	case tcloudcommon.ResourceStateActive,
		tcloudcommon.ResourceStateActivating,
		tcloudcommon.ResourceStateUpdating:
	default:
		name := string(state)
		if name == "" {
			name = found.GetState().String()
		}
		return nil, requeue.NeededAfter(
			fmt.Errorf("can't adopt tcloud namespace %q in state %q", found.GetNamespace(), name),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	spec := adoptedSpec(cr, found)
	if !equality.Semantic.DeepEqual(cr.Spec, spec) || !isAdopted(latest) {
		// Patch a copy so latest is unchanged.
		patched := cr.DeepCopy()
		patched.Spec = spec
		if patched.Annotations == nil {
			patched.Annotations = map[string]string{}
		}
		patched.Annotations[tcloudcommon.AnnotationKeyAdopted] = "true"
		err = r.KubeClient().Patch(
			ctx, patched,
			kclient.MergeFromWithOptions(cr, kclient.MergeFromWithOptimisticLock{}),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed copying tcloud namespace %q settings to spec and marking it adopted: %w",
				found.GetNamespace(), err,
			)
		}
		// Annotation edits don't trigger a reconcile, so requeue.
		return nil, requeue.NeededAfter(
			fmt.Errorf(
				"copied tcloud namespace %q settings to spec and marked it adopted, waiting to link it",
				found.GetNamespace(),
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	return found, nil
}

// resourceID returns the subject's cloud.temporal.io/resource-id annotation
// value with surrounding whitespace removed, or "" if it is unset.
func resourceID(subject types.Resource) string {
	v, _ := annotation.Value(subject, tcloudcommon.AnnotationKeyResourceID)
	return strings.TrimSpace(v)
}

// isAdopted returns true if the subject has the cloud.temporal.io/adopted
// annotation set to "true".
func isAdopted(subject types.Resource) bool {
	v, ok := annotation.Value(subject, tcloudcommon.AnnotationKeyAdopted)
	return ok && strings.EqualFold(strings.TrimSpace(v), "true")
}

// clearAdopted removes the cloud.temporal.io/adopted annotation from the
// stored CR, so a Namespace the operator creates for it later isn't treated
// as adopted.
func (r *Reconciler) clearAdopted(
	ctx context.Context,
	cr *tcloudv1alpha1.Namespace,
) error {
	patched := cr.DeepCopy()
	for k := range patched.Annotations {
		if strings.EqualFold(k, tcloudcommon.AnnotationKeyAdopted) {
			delete(patched.Annotations, k)
		}
	}
	err := r.KubeClient().Patch(
		ctx, patched,
		kclient.MergeFromWithOptions(cr, kclient.MergeFromWithOptimisticLock{}),
	)
	if err != nil {
		return fmt.Errorf("failed removing %s annotation: %w", tcloudcommon.AnnotationKeyAdopted, err)
	}
	return nil
}

// adoptedSpec returns Temporal Cloud's settings for found as a CR spec. The
// CR's own project is kept: the CRD doesn't allow adding, changing or
// removing it after creation, and syncNamespace reports one that doesn't
// match.
func adoptedSpec(
	cr *tcloudv1alpha1.Namespace,
	found *tcloudnamespacev1.Namespace,
) tcloudv1alpha1.NamespaceSpec {
	var ns tcloudv1alpha1.Namespace
	ns.FromProto(found)
	ns.Spec.Project = cr.Spec.Project
	return ns.Spec
}

// adoptionPolicy returns the subject's adoption policy from its
// cloud.temporal.io/adoption-policy annotation.
func adoptionPolicy(subject types.Resource) (tcloudcommon.AdoptionPolicy, error) {
	v, ok := annotation.Value(subject, tcloudcommon.AnnotationKeyAdoptionPolicy)
	if !ok {
		return "", nil
	}

	v = strings.ToLower(strings.TrimSpace(v))
	p := tcloudcommon.AdoptionPolicy(v)
	return p, p.Validate()
}
