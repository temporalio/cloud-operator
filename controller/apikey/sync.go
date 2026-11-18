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
	"google.golang.org/protobuf/proto"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/resource/reference"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud"
	"github.com/temporalio/cloud-operator/pkg/types"
)

// ResourceSync creates the Temporal Cloud API key and its token Secret if the
// key doesn't exist yet, and otherwise updates the key to match `desired`. It
// returns a copy of `latest` with any status changes.
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

	err = updateAPIKey(ctx, tcc, desiredKey, latest.CR().(*tcloudv1alpha1.APIKey))
	return updated, err
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

// updateAPIKey sends at most one update per reconcile. Temporal Cloud applies
// it asynchronously, so the next reconcile observes the result.
func updateAPIKey(
	ctx context.Context,
	tcc tcloudservicev1.CloudServiceClient,
	desired *tcloudv1alpha1.APIKey,
	latest *tcloudv1alpha1.APIKey,
) error {
	refs := latest.Status.References
	if err := checkReferences(desired, refs); err != nil {
		return err
	}
	want := desired.Spec.ToProto(desired.Namespace, refs)

	resourceID := latest.Status.Resource.ID
	err := cloud.RequireActive(resourceKind, resourceID, latest.Status.Resource.State)
	if err != nil {
		return err
	}

	// latest's spec comes from Temporal Cloud, so its owner is already an ID.
	var none tcloudv1alpha1.APIKeyReferences
	got := latest.Spec.ToProto(latest.Namespace, none)

	// The owner can't be changed, but a ref can resolve to a new ID, for example
	// when its ServiceAccount is recreated.
	if want.GetOwnerId() != got.GetOwnerId() {
		return ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: tcloud api key %q is owned by service account %q but spec.owner.serviceAccount is %q",
				ctrlerrors.ErrResourceSync, resourceID, got.GetOwnerId(), want.GetOwnerId(),
			),
		)
	}

	// This sync runs on most reconciles, even when nothing changed, for example
	// because desired names the owner by ref and latest by ID. Only send an
	// update when Temporal Cloud would see a difference.
	if proto.Equal(want, got) {
		return nil
	}

	_, err = tcc.UpdateApiKey(ctx, &tcloudservicev1.UpdateApiKeyRequest{
		KeyId:           resourceID,
		Spec:            want,
		ResourceVersion: latest.Status.Resource.Version,
	})
	if err != nil {
		return cloud.CallError("updating tcloud api key", err)
	}
	return requeue.NeededAfter(
		fmt.Errorf("updated tcloud api key %q, waiting for update to complete", resourceID),
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
