package namespace

import (
	"log/slog"

	"github.com/temporalio/kube-temporal/pkg/controller"
	"github.com/temporalio/kube-temporal/pkg/controller/config"
	tcloudv1alpha1 "github.com/temporalio/resource-operator/api/cloud/v1alpha1"
)

// +kubebuilder:rbac:groups=cloud.temporal.io,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=namespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=namespaces/finalizers,verbs=update

const (
	ControllerName = "namespace"
)

// New returns a new [controller.Controller] that handles
// [cloudv1alpha1.Namespace] resources.
func New(
	cfg config.Config,
	logger *slog.Logger,
) *controller.Controller {
	return controller.New(
		ControllerName,
		cfg,
		NewReconciler(),
		controller.WithLogger(logger),
	)
}

// NewWithDefaultCloudConnectionSpec returns a new [controller.Controller] that
// handles [tcloudv1alpha1.Namespace] resources and passes in a
// [tcloudv1alpha1.ConnectionSpec] to instruct the reconciler how to connect to
// a Temporal Cloud API endpoint.
func NewWithDefaultCloudConnectionSpec(
	cfg config.Config,
	logger *slog.Logger,
	spec tcloudv1alpha1.ConnectionSpec,
) *controller.Controller {
	return controller.New(
		ControllerName,
		cfg,
		NewReconciler(WithDefaultCloudConnectionSpec(spec)),
		controller.WithLogger(logger),
	)
}
