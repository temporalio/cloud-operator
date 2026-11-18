package namespace

import (
	"log/slog"

	"github.com/temporalio/kube-temporal/pkg/controller"
	"github.com/temporalio/kube-temporal/pkg/controller/config"

	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
)

// +kubebuilder:rbac:groups=cloud.temporal.io,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=namespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=namespaces/finalizers,verbs=update
// Namespaces can reference Project custom resources.
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=projects,verbs=get;list;watch

const (
	ControllerName = "namespace"

	// resourceKind names the Temporal Cloud resource in messages.
	resourceKind = "namespace"
)

// New returns a new [controller.Controller] that handles
// [tcloudv1alpha1.Namespace] resources.
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
