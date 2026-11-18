package apikey

import (
	"context"
	"errors"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/resource/reference"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceLatest returns a copy of the supplied [types.Resource] with the
// latest observed state of the Temporal Cloud API key, and the spec's owner
// reference resolved into Status.References.
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

	cr := latest.CR().(*tcloudv1alpha1.APIKey)

	refs, err := r.resolveReferences(ctx, cr)
	if err != nil {
		return latest, fmt.Errorf("failed resolving references: %w", err)
	}

	var tkey *tcloudidentityv1.ApiKey
	if latest.PreviouslyReconciled() {
		var resp *tcloudservicev1.GetApiKeyResponse
		resp, err = tcc.GetApiKey(ctx, &tcloudservicev1.GetApiKeyRequest{
			KeyId: latest.ResourceID(),
		})
		if status.Code(err) == codes.NotFound {
			// The API key was deleted outside of the operator. Treat it as
			// never created so that ResourceSync creates it again.
			err = nil
		} else if err != nil {
			return latest, fmt.Errorf("failed getting tcloud api key: %w", err)
		}
		tkey = resp.GetApiKey()
	}

	if tkey == nil {
		cr.Spec = tcloudv1alpha1.APIKeySpec{Token: cr.Spec.Token}
		cr.Status = tcloudv1alpha1.APIKeyStatus{References: refs}
		return latest, nil
	}
	cr.Status.References = refs
	*cr = *cr.FromProto(tkey)
	return latest, nil
}

// resolveReferences returns the Temporal Cloud ID of the ServiceAccount custom
// resource that the CR's spec refers to.
func (r *Reconciler) resolveReferences(
	ctx context.Context,
	cr *tcloudv1alpha1.APIKey,
) (tcloudv1alpha1.APIKeyReferences, error) {
	var out tcloudv1alpha1.APIKeyReferences
	owner := cr.Spec.Owner.ServiceAccount
	if owner.Ref == nil {
		return out, nil
	}

	id, err := reference.ResolveServiceAccountID(ctx, r.KubeReader(), cr.Namespace, owner)
	if errors.Is(err, reference.ErrNotReady) {
		return out, nil
	}
	if err != nil {
		return out, err
	}

	namespace := owner.Ref.Namespace
	if namespace == "" {
		namespace = cr.Namespace
	}
	out.ServiceAccount = &tcloudv1alpha1.ServiceAccountResolvedReference{
		Namespace: namespace, Name: owner.Ref.Name, ID: id,
	}
	return out, nil
}
