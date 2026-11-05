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

// adoptNamespace decides whether the Namespace found by name can be linked to
// latest, which has no status.id yet. It returns the Namespace to link, or nil
// so the create path runs. Without the adoption-policy annotation it returns
// found unchanged.
//
// The adopt policy takes two reconciles. The first copies Temporal Cloud's
// settings into the CR's spec and requeues. The spec change triggers the
// second, which links the Namespace once the specs match, so status.id is
// never saved next to an empty spec and ResourceSync has nothing to update.
func (r *Reconciler) adoptNamespace(
	ctx context.Context,
	latest types.Resource,
	found *tcloudnamespacev1.Namespace,
) (*tcloudnamespacev1.Namespace, error) {
	cr := latest.CR().(*tcloudv1alpha1.Namespace)

	policy, err := adoptionPolicy(latest)
	if err != nil {
		// Annotation edits don't trigger a reconcile, so requeue to pick up
		// a corrected value.
		return nil, requeue.NeededAfter(
			fmt.Errorf("%w. Fix or remove the %s annotation", err, tcloudcommon.AnnotationKeyAdoptionPolicy),
			requeue.DefaultRequeueAfterDuration,
		)
	}
	if policy == "" {
		return found, nil
	}

	var state tcloudcommon.ResourceState
	state.FromProto(found.GetState())
	if found == nil || state == tcloudcommon.ResourceStateDeleted {
		return nil, requeue.NeededAfter(
			fmt.Errorf("no tcloud namespace named %q to adopt", cr.Name),
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
	if !equality.Semantic.DeepEqual(cr.Spec, spec) {
		// Patch a copy so latest is unchanged.
		patched := cr.DeepCopy()
		patched.Spec = spec
		err = r.KubeClient().Patch(
			ctx, patched,
			kclient.MergeFromWithOptions(cr, kclient.MergeFromWithOptimisticLock{}),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed copying tcloud namespace %q settings to spec: %w",
				found.GetNamespace(), err,
			)
		}
		return nil, requeue.NeededAfter(
			fmt.Errorf("copied tcloud namespace %q settings to spec, waiting to link it", found.GetNamespace()),
			requeue.DefaultRequeueAfterDuration,
		)
	}

	cr.Status.Source = tcloudcommon.ResourceSourceAdopted
	return found, nil
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
