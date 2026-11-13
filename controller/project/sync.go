package project

import (
	"context"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"k8s.io/apimachinery/pkg/api/equality"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceSync creates the Temporal Cloud project if it doesn't exist yet, and
// otherwise updates it to match `desired`. It returns a copy of `latest` with
// any status changes.
//
// `desired` is NEVER mutated.
// `latest` is NEVER mutated.
func (r *Reconciler) ResourceSync(
	ctx context.Context,
	desired kttypes.Resource,
	latest kttypes.Resource,
) (kttypes.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.sync")
	defer func() {
		exit(err)
	}()

	updated := latest.DeepCopy()

	tcc, err := r.CloudClientFor(ctx, desired)
	if err != nil {
		return updated, fmt.Errorf("failed getting tcloud client: %w", err)
	}

	desiredProject := desired.CR().(*tcloudv1alpha1.Project)
	if !latest.(types.Resource).PreviouslyReconciled() {
		err = createProject(
			ctx, tcc, desiredProject, updated.CR().(*tcloudv1alpha1.Project),
		)
		return updated, err
	}

	err = updateProject(
		ctx, tcc, desiredProject, latest.CR().(*tcloudv1alpha1.Project),
	)
	return updated, err
}

// createProject creates the Temporal Cloud project for the supplied `desired`
// CR and saves its ID in `updated`.
//
// `desired` is NEVER mutated.
func createProject(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.Project,
	updated *tcloudv1alpha1.Project, // mutated by this function
) error {
	resp, err := tcc.CreateProject(ctx, &tcloudservicev1.CreateProjectRequest{
		Spec: desired.ToProto().GetSpec(),
	})
	if err != nil {
		return cloud.CallError("creating tcloud project", err)
	}

	updated.Status.Resource.ID = resp.GetProjectId()
	return requeue.NeededAfter(
		fmt.Errorf(
			"created tcloud project %q, waiting for activation",
			resp.GetProjectId(),
		),
		requeue.DefaultRequeueAfterDuration,
	)
}

// updateProject makes at most one Temporal Cloud call to move an existing
// project toward the desired state. An update starts an async operation, so
// after it we requeue and let the next reconcile observe the result.
func updateProject(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.Project,
	latest *tcloudv1alpha1.Project,
) error {
	resourceID := latest.Status.Resource.ID
	err := cloud.RequireActive("project", resourceID, latest.Status.Resource.State)
	if err != nil {
		return err
	}

	// The controller's DeepEqual always reports a difference once
	// Status.Conditions is populated, so this sync can run even when nothing
	// has changed. Compare the specs here so we don't send an update we don't
	// need.
	if equality.Semantic.DeepEqual(
		normalizeSpec(&desired.Spec),
		normalizeSpec(&latest.Spec),
	) {
		return nil
	}

	_, err = tcc.UpdateProject(ctx, &tcloudservicev1.UpdateProjectRequest{
		ProjectId:       resourceID,
		Spec:            desired.ToProto().GetSpec(),
		ResourceVersion: latest.Status.Resource.Version,
	})
	if err != nil {
		return cloud.CallError("updating tcloud project", err)
	}
	return requeue.NeededAfter(
		fmt.Errorf(
			"updated tcloud project %q, waiting for update to complete",
			resourceID,
		),
		requeue.DefaultRequeueAfterDuration,
	)
}

// normalizeSpec returns a copy of the supplied spec with optional blocks that
// hold only zero values set to nil. Temporal Cloud can return an empty block
// for a field the CR leaves unset, and the two would otherwise compare as
// different, causing an update on every reconcile.
//
// Mirrors specWithoutProjectAndTags in the namespace controller.
func normalizeSpec(spec *tcloudv1alpha1.ProjectSpec) *tcloudv1alpha1.ProjectSpec {
	out := spec.DeepCopy()
	out.Lifecycle = nilIfEmpty(out.Lifecycle)
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
