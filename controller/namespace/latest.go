package namespace

import (
	"context"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/types"
)

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

	// current is the CR as read from Kubernetes, so it tells us whether an
	// earlier reconcile linked it (status.resource.id is set)
	// and whether it has the adoption-policy: adopt annotation.
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
		// Looks up the Namespace by the ID from the resource-id annotation,
		// which ResourceLatest gets from ResourceID().
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

// getNamespaceByID returns the Temporal Cloud Namespace with the supplied ID,
// or nil if Temporal Cloud reports it NotFound.
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
