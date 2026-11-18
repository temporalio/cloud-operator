package apikey

import (
	"context"
	"errors"

	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
)

// ResourceDelete deletes the Temporal Cloud API key for the supplied
// [types.Resource].
//
// TODO(tomba): delete the API key in Temporal Cloud.
func (r *Reconciler) ResourceDelete(
	_ context.Context,
	original kttypes.Resource,
) (kttypes.Resource, error) {
	return original.DeepCopy(), ctrlerrors.NewTerminalError(
		errors.New("deleting API keys isn't supported yet"),
	)
}
