package serviceaccount_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/serviceaccount"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const (
	kubeNamespace = "apps"
	saName        = "ci"
	saID          = "sa-1"
	nsName        = "payments"
	nsID          = "payments.acct1"
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
// server configured with serverOpts, and reads custom resources from a fake
// Kubernetes client holding kubeObjs.
func newReconciler(
	t *testing.T,
	kubeObjs []runtime.Object,
	serverOpts ...fakeserver.NewOption,
) *serviceaccount.Reconciler {
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
	kc := kube.NewFakeClient(kubeObjs...)
	r := serviceaccount.NewReconciler(base.WithDefaultCloudClient(*tcc))
	r.SetKubeClient(kc)
	r.SetKubeReader(kc)
	return r
}

// referencedNamespace returns the Active Namespace CR that service accounts in
// these tests reference, with the supplied Temporal Cloud ID.
func referencedNamespace(id string) *tcloudv1alpha1.Namespace {
	ns := &tcloudv1alpha1.Namespace{}
	ns.Namespace = kubeNamespace
	ns.Name = nsName
	ns.Status.Resource.ID = id
	ns.Status.Resource.State = cloudcommon.ResourceStateActive
	return ns
}

// newServiceAccount returns a namespace scoped ServiceAccount CR whose
// namespace is the supplied reference.
func newServiceAccount(namespace cloudcommon.ResourceReferenceOrID) *tcloudv1alpha1.ServiceAccount {
	sa := &tcloudv1alpha1.ServiceAccount{}
	sa.Namespace = kubeNamespace
	sa.Name = saName
	sa.Spec = tcloudv1alpha1.ServiceAccountSpec{
		DisplayName: "ci",
		Description: "deploys",
		NamespaceScopedAccess: &tcloudv1alpha1.NamespaceScopedAccess{
			Namespace: namespace,
			Access: tcloudv1alpha1.NamespaceAccess{
				Permission: tcloudv1alpha1.NamespacePermissionWrite,
			},
		},
	}
	return sa
}

func ref(name string) cloudcommon.ResourceReferenceOrID {
	return cloudcommon.ResourceReferenceOrID{Ref: &cloudcommon.ResourceReference{Name: name}}
}

func id(s string) cloudcommon.ResourceReferenceOrID {
	return cloudcommon.ResourceReferenceOrID{ID: s}
}

// cloudSpec is the Temporal Cloud spec for the service account returned by
// newServiceAccount, with its namespace resolved to nsID.
func cloudSpec() *tcloudidentityv1.ServiceAccountSpec {
	return &tcloudidentityv1.ServiceAccountSpec{
		Name:        "ci",
		Description: "deploys",
		NamespaceScopedAccess: &tcloudidentityv1.NamespaceScopedAccess{
			Namespace: nsID,
			Access: &tcloudidentityv1.NamespaceAccess{
				Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE,
			},
		},
	}
}

func resolvedRefs(namespaceID string) tcloudv1alpha1.ServiceAccountReferences {
	return tcloudv1alpha1.ServiceAccountReferences{
		Namespaces: []tcloudv1alpha1.NamespaceResolvedReference{
			{Namespace: kubeNamespace, Name: nsName, ID: namespaceID},
		},
	}
}
