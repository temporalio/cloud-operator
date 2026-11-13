// Package reference resolves references between custom resources into
// Temporal Cloud IDs.
package reference

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

// ErrNotReady is wrapped by the error ResolveID returns when the referenced
// custom resource doesn't exist or isn't ready in Temporal Cloud yet.
var ErrNotReady = errors.New("referenced resource not ready")

// ResolveID returns the Temporal Cloud ID that `refOrID` refers to. `cloudID`
// returns the ID of `obj`, or an empty string when it isn't ready yet.
func ResolveID[T client.Object](
	ctx context.Context,
	kr client.Reader,
	referrerNamespace string,
	refOrID cloudcommon.ResourceReferenceOrID,
	obj T,
	cloudID func(T) string,
) (string, error) {
	if refOrID.Ref == nil {
		return refOrID.ID, nil
	}

	key := client.ObjectKey{Namespace: refOrID.Ref.Namespace, Name: refOrID.Ref.Name}
	if key.Namespace == "" {
		key.Namespace = referrerNamespace
	}
	if kr == nil {
		return "", fmt.Errorf("failed reading %s: nil kube reader", key)
	}

	err := kr.Get(ctx, key, obj)
	if apierrors.IsNotFound(err) {
		return "", fmt.Errorf("%w: %s not found", ErrNotReady, key)
	}
	if err != nil {
		return "", fmt.Errorf("failed reading %s: %w", key, err)
	}

	id := cloudID(obj)
	if id == "" {
		return "", fmt.Errorf("%w: %s isn't ready in Temporal Cloud yet", ErrNotReady, key)
	}
	return id, nil
}

// ResolveNamespaceID returns the Temporal Cloud ID of the Namespace that refOrID
// refers to, once that Namespace is Active.
func ResolveNamespaceID(
	ctx context.Context,
	kr client.Reader,
	referrerNamespace string,
	refOrID cloudcommon.ResourceReferenceOrID,
) (string, error) {
	return ResolveID(
		ctx, kr, referrerNamespace, refOrID,
		&tcloudv1alpha1.Namespace{},
		func(ns *tcloudv1alpha1.Namespace) string {
			// Temporal Cloud rejects a service account for a namespace that is
			// still activating.
			if ns.Status.Resource.State != cloudcommon.ResourceStateActive {
				return ""
			}
			return ns.Status.Resource.ID
		},
	)
}

// Check collects the references in a spec that can't be sent to
// Temporal Cloud yet.
type Check struct {
	referrerNamespace string
	pending           []string
	unsupported       []string
}

// NewCheck returns a Check for a custom resource in the supplied namespace.
func NewCheck(referrerNamespace string) *Check {
	return &Check{referrerNamespace: referrerNamespace}
}

// Pending records the reference at path when it is a ref that has no
// resolved ID yet.
func (c *Check) Pending(
	path string,
	refOrID cloudcommon.ResourceReferenceOrID,
	resolved string,
) {
	if refOrID.Ref == nil || resolved != "" {
		return
	}
	namespace := refOrID.Ref.Namespace
	if namespace == "" {
		namespace = c.referrerNamespace
	}
	c.pending = append(c.pending, fmt.Sprintf("%s (%s/%s)", path, namespace, refOrID.Ref.Name))
}

// Unsupported records the reference at path when it is a ref to a kind that
// can't be referenced yet.
//
// TODO(tomba): remove when all resource types have been implemented.
func (c *Check) Unsupported(path string, refOrID cloudcommon.ResourceReferenceOrID) {
	if refOrID.Ref != nil {
		c.unsupported = append(c.unsupported, path)
	}
}

// Err returns a terminal error for unsupported references, a requeue error
// for pending ones, or nil.
func (c *Check) Err() error {
	if len(c.unsupported) > 0 {
		return ctrlerrors.NewTerminalError(
			fmt.Errorf(
				"%w: references to these custom resources aren't supported yet, "+
					"set id instead: %s",
				ctrlerrors.ErrResourceSync, strings.Join(c.unsupported, ", "),
			),
		)
	}
	if len(c.pending) > 0 {
		return requeue.NeededAfter(
			fmt.Errorf(
				"waiting for referenced custom resources to be ready in Temporal Cloud: %s",
				strings.Join(c.pending, ", "),
			),
			requeue.DefaultRequeueAfterDuration,
		)
	}
	return nil
}
