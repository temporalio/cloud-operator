package apikey

import (
	"context"
	"errors"
	"fmt"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/resource/reference"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceSync creates the Temporal Cloud API key and its token Secret if the
// key doesn't exist yet. It returns a copy of `latest` with any status
// changes.
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

	desiredKey := desired.CR().(*tcloudv1alpha1.APIKey)
	if desiredKey.Spec.Token.SecretKeyRef == nil {
		err = ctrlerrors.NewTerminalError(
			fmt.Errorf("%w: spec is required to create an API key", ctrlerrors.ErrResourceSync),
		)
		return updated, err
	}

	tcc, err := r.CloudClientFor(ctx, desired)
	if err != nil {
		return updated, fmt.Errorf("failed getting tcloud client: %w", err)
	}

	if !latest.(types.Resource).PreviouslyReconciled() {
		err = r.createAPIKey(ctx, tcc, desiredKey, updated.CR().(*tcloudv1alpha1.APIKey))
		return updated, err
	}

	// TODO(tomba): update the mutable fields of an existing API key.
	return updated, nil
}

// createAPIKey creates the Temporal Cloud API key for the supplied `desired`
// CR, writes its token to the Secret in spec.token.secretKeyRef, and saves
// the key's ID in `updated`.
//
// `desired` is NEVER mutated.
func (r *Reconciler) createAPIKey(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.APIKey,
	updated *tcloudv1alpha1.APIKey, // mutated by this function
) error {
	refs := updated.Status.References
	if err := checkReferences(desired, refs); err != nil {
		return err
	}

	// Temporal Cloud returns the token only in the create response, so check
	// that it can be stored before creating the key.
	if err := r.checkTokenSecret(ctx, desired); err != nil {
		return err
	}

	resp, err := tcc.CreateApiKey(ctx, &tcloudservicev1.CreateApiKeyRequest{
		Spec: desired.Spec.ToProto(desired.Namespace, refs),
	})
	if err != nil {
		return cloud.CallError("creating tcloud api key", err)
	}

	keyID := resp.GetKeyId()
	updated.Status.Resource.ID = keyID
	if err := r.writeTokenSecret(ctx, desired, resp.GetToken()); err != nil {
		// TODO(tomba): nothing holds this key's token anymore. Replace the key
		// instead of keeping it.
		return errors.Join(fmt.Errorf("created tcloud api key %q but failed storing its token", keyID), err)
	}
	return requeue.NeededAfter(
		fmt.Errorf("created tcloud api key %q, waiting for activation", keyID),
		requeue.DefaultRequeueAfterDuration,
	)
}

// checkReferences checks the owner ref in the spec before anything is sent to
// Temporal Cloud.
func checkReferences(
	cr *tcloudv1alpha1.APIKey,
	refs tcloudv1alpha1.APIKeyReferences,
) error {
	check := reference.NewCheck(cr.Namespace)
	owner := cr.Spec.Owner.ServiceAccount
	if owner.Ref != nil {
		id, _ := refs.ServiceAccountID(*owner.Ref, cr.Namespace)
		check.Pending("spec.owner.serviceAccount", owner, id)
	}
	return check.Err()
}
