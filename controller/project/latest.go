package project

import (
	"context"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudprojectv1 "go.temporal.io/cloud-sdk/api/project/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceLatest returns a copy of the supplied [types.Resource] with the
// latest observed state of the Temporal Cloud project.
//
// `current` is NEVER mutated.
func (r *Reconciler) ResourceLatest(
	ctx context.Context,
	current kttypes.Resource,
) (kttypes.Resource, error) {
	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("resource.latest")
	defer func() {
		exit(err)
	}()

	latest := current.DeepCopy().(types.Resource)

	tcc, err := r.CloudClientFor(ctx, current)
	if err != nil {
		return latest, fmt.Errorf("failed getting tcloud client: %w", err)
	}

	cr := latest.CR().(*tcloudv1alpha1.Project)

	getProject := func(id string) (*tcloudprojectv1.Project, error) {
		resp, err := tcc.GetProject(ctx, &tcloudservicev1.GetProjectRequest{
			ProjectId: id,
		})
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return resp.GetProject(), nil
	}

	// TODO: like the service account controller, this misses a project whose
	// create succeeded but whose ID was never saved to status (for example
	// because the operator restarted in between). Display names aren't a safe
	// unique key to look it up by, so a duplicate project would be created.
	var tp *tcloudprojectv1.Project
	switch {
	case latest.PreviouslyReconciled():
		// The project may have been deleted outside of the operator. Treat it
		// the same as one that was never created so ResourceSync recreates it
		// in this pass.
		tp, err = getProject(latest.ResourceID())
		if err != nil {
			return latest, fmt.Errorf("failed getting tcloud project: %w", err)
		}
	case latest.AdoptionPolicy() == cloudcommon.AdoptionPolicyAdopt:
		resourceID := latest.ResourceID()
		tp, err = getProject(resourceID)
		if err != nil {
			return latest, fmt.Errorf("failed getting tcloud project %q to adopt: %w", resourceID, err)
		}
		// There's nothing to adopt. Without an error the controller would see
		// no difference from the annotations-only CR and report success.
		if tp == nil {
			err = fmt.Errorf("no tcloud project with ID %q to adopt", resourceID)
			return latest, err
		}
	}

	if tp == nil {
		// The project does not exist in Temporal Cloud, so empty the spec and
		// status.
		cr.Spec = tcloudv1alpha1.ProjectSpec{}
		cr.Status = tcloudv1alpha1.ProjectStatus{}
		return latest, nil
	}

	*cr = *cr.FromProto(tp)
	return latest, nil
}
