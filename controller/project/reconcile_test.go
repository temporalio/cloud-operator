package project_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	tcloudprojectv1 "go.temporal.io/cloud-sdk/api/project/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/project"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const (
	kubeNamespace = "apps"
	projectName   = "payments"
	projectID     = "project-1"
)

// Kinds of error the controller treats differently, as returned by errKind.
const (
	kindNil      = "nil"
	kindRequeue  = "requeue"
	kindTerminal = "terminal"
	kindOther    = "other"
)

// errKind classifies err the way the kube-temporal controller does.
func errKind(err error) string {
	var terminal *ctrlerrors.TerminalError
	var requeueAfter *requeue.RequeueNeededAfter
	switch {
	case err == nil:
		return kindNil
	case errors.As(err, &terminal):
		return kindTerminal
	case errors.As(err, &requeueAfter):
		return kindRequeue
	default:
		return kindOther
	}
}

// newReconciler returns a Reconciler that talks to a fake Temporal Cloud
// server configured with serverOpts. A Temporal Cloud call the test doesn't
// set up a handler for fails with Unimplemented.
func newReconciler(
	t *testing.T,
	serverOpts ...fakeserver.NewOption,
) *project.Reconciler {
	t.Helper()
	srv, err := fakeserver.New(serverOpts...)
	require.NoError(t, err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(t, err)

	// Set the kube client and reader the way BindManager does.
	// base.WithKubeClient would also create a cloud connection manager,
	// which looks for a hostport annotation instead of using the default
	// client.
	kc := kube.NewFakeClient()
	r := project.NewReconciler(base.WithDefaultCloudClient(*tcc))
	r.SetKubeClient(kc)
	r.SetKubeReader(kc)
	return r
}

// newProject returns a Project CR as a user would write it, with optional
// blocks left unset.
func newProject() *tcloudv1alpha1.Project {
	return &tcloudv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeNamespace, Name: projectName},
		Spec: tcloudv1alpha1.ProjectSpec{
			DisplayName: "Payments",
			Description: "payments team",
		},
	}
}

// cloudSpec returns the Temporal Cloud spec matching newProject's spec.
func cloudSpec() *tcloudprojectv1.ProjectSpec {
	return &tcloudprojectv1.ProjectSpec{
		DisplayName: "Payments",
		Description: "payments team",
	}
}
