package apikey

import (
	"log/slog"

	"github.com/temporalio/kube-temporal/pkg/controller"
	"github.com/temporalio/kube-temporal/pkg/controller/config"

	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
)

// +kubebuilder:rbac:groups=cloud.temporal.io,resources=apikeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=apikeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=apikeys/finalizers,verbs=update
// API keys reference ServiceAccount custom resources.
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=serviceaccounts,verbs=get;list;watch
// API key tokens are stored in Secrets.
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update

const (
	ControllerName = "apikey"
)

// New returns a new [controller.Controller] that handles
// [tcloudv1alpha1.APIKey] resources.
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
