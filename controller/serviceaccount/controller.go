package serviceaccount

import (
	"log/slog"

	"github.com/temporalio/kube-temporal/pkg/controller"
	"github.com/temporalio/kube-temporal/pkg/controller/config"

	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
)

// +kubebuilder:rbac:groups=cloud.temporal.io,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=serviceaccounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=serviceaccounts/finalizers,verbs=update
// Service accounts can reference Namespace custom resources.
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=namespaces,verbs=get;list;watch

const (
	ControllerName = "serviceaccount"
)

// New returns a new [controller.Controller] that handles
// [tcloudv1alpha1.ServiceAccount] resources.
func New(
	cfg config.Config,
	logger *slog.Logger,
	opts ...base.NewOption,
) *controller.Controller {
	return controller.New(
		ControllerName,
		cfg,
		NewReconciler(opts...),
		controller.WithLogger(logger),
	)
}
