package apikey

import (
	"context"
	"fmt"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// readTokenSecret returns the Secret in spec.token.secretKeyRef, or nil when it
// doesn't exist.
func (r *Reconciler) readTokenSecret(
	ctx context.Context,
	cr *tcloudv1alpha1.APIKey,
) (*corev1.Secret, error) {
	key := client.ObjectKey{Namespace: cr.Namespace, Name: cr.Spec.Token.SecretKeyRef.Name}
	secret := &corev1.Secret{}
	err := r.KubeReader().Get(ctx, key, secret)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed reading Secret %s: %w", key, err)
	}
	return secret, nil
}

// checkTokenSecret returns a requeue error when the Secret in
// spec.token.secretKeyRef exists and `cr` doesn't control it.
// We never want the operator to overwrite a Secret it doesn't manage.
func (r *Reconciler) checkTokenSecret(
	ctx context.Context,
	cr *tcloudv1alpha1.APIKey,
) error {
	secret, err := r.readTokenSecret(ctx, cr)
	if err != nil {
		return err
	}
	if secret == nil || controlledBy(secret, cr) {
		return nil
	}
	return secretNotControlledError(secret)
}

// writeTokenSecret stores `token` in the Secret in spec.token.secretKeyRef.
func (r *Reconciler) writeTokenSecret(
	ctx context.Context,
	cr *tcloudv1alpha1.APIKey,
	token string,
) error {
	existing, err := r.readTokenSecret(ctx, cr)
	if err != nil {
		return err
	}
	if existing == nil {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       cr.Namespace,
				Name:            cr.Spec.Token.SecretKeyRef.Name,
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cr, gvk)},
			},
			Type: corev1.SecretTypeOpaque,
		}
		setToken(secret, cr, token)
		if err := r.KubeClient().Create(ctx, secret); err != nil {
			return fmt.Errorf("failed creating Secret %s: %w", client.ObjectKeyFromObject(secret), err)
		}
		return nil
	}
	if !controlledBy(existing, cr) {
		return secretNotControlledError(existing)
	}
	setToken(existing, cr, token)
	if err := r.KubeClient().Update(ctx, existing); err != nil {
		return fmt.Errorf("failed updating Secret %s: %w", client.ObjectKeyFromObject(existing), err)
	}
	return nil
}

// setToken sets `token` in `secret`'s data.
func setToken(secret *corev1.Secret, cr *tcloudv1alpha1.APIKey, token string) {
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[cr.Spec.Token.SecretKeyRef.Key] = []byte(token)
}

// controlledBy reports whether `cr` is the controller of `secret`.
func controlledBy(secret *corev1.Secret, cr *tcloudv1alpha1.APIKey) bool {
	ref := metav1.GetControllerOf(secret)
	return ref != nil && ref.UID == cr.UID
}

// secretNotControlledError returns the error for a token Secret that the
// APIKey doesn't control.
func secretNotControlledError(secret *corev1.Secret) error {
	controller := "nothing"
	if ref := metav1.GetControllerOf(secret); ref != nil {
		controller = fmt.Sprintf("%s %s (uid %s)", ref.Kind, ref.Name, ref.UID)
	}
	// spec.token can't be changed, so requeue until the Secret is deleted or
	// renamed.
	return requeue.NeededAfter(
		fmt.Errorf(
			"Secret %s in spec.token.secretKeyRef already exists and is controlled by %s, "+
				"not this APIKey. Delete or rename the Secret so the operator can create it",
			client.ObjectKeyFromObject(secret), controller,
		),
		requeue.DefaultRequeueAfterDuration,
	)
}

// releaseTokenSecret removes the ownerReference to `cr` from the token Secret,
// so that deleting `cr` doesn't delete the Secret.
func (r *Reconciler) releaseTokenSecret(
	ctx context.Context,
	cr *tcloudv1alpha1.APIKey,
) error {
	secret, err := r.readTokenSecret(ctx, cr)
	if err != nil {
		return err
	}
	if secret == nil || !controlledBy(secret, cr) {
		return nil
	}
	var owners []metav1.OwnerReference
	for _, ref := range secret.OwnerReferences {
		if ref.UID != cr.UID {
			owners = append(owners, ref)
		}
	}
	secret.OwnerReferences = owners
	if err := r.KubeClient().Update(ctx, secret); err != nil {
		return fmt.Errorf("failed updating Secret %s: %w", client.ObjectKeyFromObject(secret), err)
	}
	return nil
}
