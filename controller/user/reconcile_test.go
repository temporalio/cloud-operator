package user_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	"github.com/temporalio/kube-temporal/pkg/kube/condition"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/user"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const (
	kubeNamespace = "apps"
	userName      = "alice"
	userEmail     = "alice@example.com"
	userID        = "user-1"
	nsName        = "payments"
	nsID          = "payments.acct1"
)

// newReconciler returns a Reconciler that talks to a fake Temporal Cloud server
// set up with serverOpts, and reads CRs from a fake Kubernetes client holding kubeObjs.
func newReconciler(
	t *testing.T,
	kubeObjs []runtime.Object,
	serverOpts ...fakeserver.NewOption,
) *user.Reconciler {
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
	r := user.NewReconciler(base.WithDefaultCloudClient(*tcc))
	r.SetKubeClient(kc)
	r.SetKubeReader(kc)
	return r
}

// referencedNamespace returns the Active Namespace CR that users in these
// tests reference, with the supplied Temporal Cloud ID.
func referencedNamespace(id string) *tcloudv1alpha1.Namespace {
	ns := &tcloudv1alpha1.Namespace{}
	ns.Namespace = kubeNamespace
	ns.Name = nsName
	ns.Status.Resource.ID = id
	ns.Status.Resource.State = cloudcommon.ResourceStateActive
	return ns
}

// newUser returns alice's User CR, a Developer with Write on the payments
// Namespace CR, not linked to a Temporal Cloud user yet.
func newUser() *tcloudv1alpha1.User {
	u := &tcloudv1alpha1.User{}
	u.Namespace = kubeNamespace
	u.Name = userName
	u.Spec = tcloudv1alpha1.UserSpec{
		Email: userEmail,
		Access: tcloudv1alpha1.AccountScopedAccess{
			AccountAccess: &tcloudv1alpha1.AccountAccess{
				Role: tcloudv1alpha1.AccountRoleDeveloper,
			},
			NamespaceAccesses: []tcloudv1alpha1.NamespaceScopedAccess{{
				Namespace: ref(nsName),
				Access: tcloudv1alpha1.NamespaceAccess{
					Permission: tcloudv1alpha1.NamespacePermissionWrite,
				},
			}},
		},
	}
	return u
}

// linkedUser returns alice's User CR linked to userID, with the supplied state
// and async operation in its status.
func linkedUser(state cloudcommon.ResourceState, opID string) *tcloudv1alpha1.User {
	u := newUser()
	u.Status.Resource = cloudcommon.ResourceStatus{
		ID: userID, Version: "rv-1", State: state, AsyncOperationID: opID,
	}
	return u
}

func ref(name string) cloudcommon.ResourceReferenceOrID {
	return cloudcommon.ResourceReferenceOrID{Ref: &cloudcommon.ResourceReference{Name: name}}
}

func id(s string) cloudcommon.ResourceReferenceOrID {
	return cloudcommon.ResourceReferenceOrID{ID: s}
}

// cloudSpec is the Temporal Cloud spec for the user returned by newUser, with
// its namespace resolved to nsID.
func cloudSpec() *tcloudidentityv1.UserSpec {
	return &tcloudidentityv1.UserSpec{
		Email: userEmail,
		Access: &tcloudidentityv1.Access{
			AccountAccess: &tcloudidentityv1.AccountAccess{
				Role: tcloudidentityv1.AccountAccess_ROLE_DEVELOPER,
			},
			NamespaceAccesses: map[string]*tcloudidentityv1.NamespaceAccess{
				nsID: {Permission: tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE},
			},
		},
	}
}

func resolvedRefs(namespaceID string) tcloudv1alpha1.UserReferences {
	return tcloudv1alpha1.UserReferences{
		Namespaces: []tcloudv1alpha1.NamespaceResolvedReference{
			{Namespace: kubeNamespace, Name: nsName, ID: namespaceID},
		},
	}
}

// asyncOperation makes GetAsyncOperation return the operation with the
// supplied ID and state.
func asyncOperation(opID string, state operationv1.AsyncOperation_State) fakeserver.NewOption {
	return fakeserver.OnGetAsyncOperationReturn(&tcloudservicev1.GetAsyncOperationResponse{
		AsyncOperation: &operationv1.AsyncOperation{
			Id: opID, State: state, FailureReason: "nope",
		},
	}, nil)
}

func userOf(res kttypes.Resource) *tcloudv1alpha1.User {
	return res.CR().(*tcloudv1alpha1.User)
}

func statusOf(res kttypes.Resource) cloudcommon.ResourceStatus {
	return userOf(res).Status.Resource
}

func requireRequeue(t *testing.T, err error) {
	t.Helper()
	var requeueAfter *requeue.RequeueNeededAfter
	require.ErrorAs(t, err, &requeueAfter)
}

func requireTerminal(t *testing.T, err error) {
	t.Helper()
	var terminal *ctrlerrors.TerminalError
	require.ErrorAs(t, err, &terminal)
}

// requireRetried checks that err is one the controller retries with backoff,
// that is, neither terminal nor a requeue.
func requireRetried(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var terminal *ctrlerrors.TerminalError
	var requeueAfter *requeue.RequeueNeededAfter
	require.False(t, errors.As(err, &terminal), "err is terminal: %v", err)
	require.False(t, errors.As(err, &requeueAfter), "err is a requeue: %v", err)
}

func requireStalled(t *testing.T, res kttypes.Resource) {
	t.Helper()
	stalled := condition.Stalled(res)
	require.NotNil(t, stalled)
	require.Equal(t, metav1.ConditionTrue, stalled.Status)
}
