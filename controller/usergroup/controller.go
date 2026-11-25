package usergroup

import (
	"log/slog"

	"github.com/temporalio/kube-temporal/pkg/controller"
	"github.com/temporalio/kube-temporal/pkg/controller/config"

	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
)

// +kubebuilder:rbac:groups=cloud.temporal.io,resources=usergroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=usergroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=usergroups/finalizers,verbs=update
// User groups can reference Namespace, Project and User custom resources.
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=projects,verbs=get;list;watch
// +kubebuilder:rbac:groups=cloud.temporal.io,resources=users,verbs=get;list;watch

const (
	ControllerName = "usergroup"

	// resourceKind names the Temporal Cloud resource in messages.
	resourceKind = "usergroup"
)

// New returns a new [controller.Controller] that handles
// [tcloudv1alpha1.UserGroup] resources.
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
