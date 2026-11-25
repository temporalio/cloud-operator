package usergroup_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	kttypes "github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"k8s.io/apimachinery/pkg/runtime"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/usergroup"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const (
	kubeNamespace = "apps"
	groupName     = "oncall"
	groupID       = "group-1"
	nsName        = "payments"
	nsID          = "payments.acct1"
	projectName   = "orders"
	projectID     = "p-orders"
)

// newReconciler returns a Reconciler that talks to a fake Temporal Cloud server
// set up with serverOpts, and reads CRs from a fake Kubernetes client holding kubeObjs.
func newReconciler(
	t *testing.T,
	kubeObjs []runtime.Object,
	serverOpts ...fakeserver.NewOption,
) *usergroup.Reconciler {
	t.Helper()
	srv, err := fakeserver.New(serverOpts...)
	require.NoError(t, err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(t, err)

	// Set the Kubernetes client directly to keep the fake Cloud client as the default.
	kc := kube.NewFakeClient(kubeObjs...)
	r := usergroup.NewReconciler(base.WithDefaultCloudClient(*tcc))
	r.SetKubeClient(kc)
	r.SetKubeReader(kc)
	return r
}

// referencedNamespace returns the Active Namespace CR that user groups in
// these tests reference, with the supplied Temporal Cloud ID.
func referencedNamespace(id string) *tcloudv1alpha1.Namespace {
	ns := &tcloudv1alpha1.Namespace{}
	ns.Namespace = kubeNamespace
	ns.Name = nsName
	ns.Status.Resource.ID = id
	ns.Status.Resource.State = cloudcommon.ResourceStateActive
	return ns
}

// referencedProject returns the orders Project CR that user groups in these
// tests can reference, with the supplied state and projectID.
func referencedProject(state cloudcommon.ResourceState) *tcloudv1alpha1.Project {
	p := &tcloudv1alpha1.Project{}
	p.Namespace = kubeNamespace
	p.Name = projectName
	p.Status.Resource.ID = projectID
	p.Status.Resource.State = state
	return p
}

// newUserGroup returns the oncall UserGroup CR, a cloud group of Developers
// with Write on the payments Namespace CR.
func newUserGroup() *tcloudv1alpha1.UserGroup {
	g := &tcloudv1alpha1.UserGroup{}
	g.Namespace = kubeNamespace
	g.Name = groupName
	g.Spec = tcloudv1alpha1.UserGroupSpec{
		DisplayName: groupName,
		CloudGroup:  &tcloudv1alpha1.CloudGroupSpec{},
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
	return g
}

// linkedUserGroup returns the oncall UserGroup CR linked to groupID, with the
// supplied state and async operation in its status.
func linkedUserGroup(state cloudcommon.ResourceState, opID string) *tcloudv1alpha1.UserGroup {
	g := newUserGroup()
	g.Status.Resource = cloudcommon.ResourceStatus{
		ID: groupID, Version: "rv-1", State: state, AsyncOperationID: opID,
	}
	return g
}

func ref(name string) cloudcommon.ResourceReferenceOrID {
	return cloudcommon.ResourceReferenceOrID{Ref: &cloudcommon.ResourceReference{Name: name}}
}

// cloudSpec is the Temporal Cloud spec for the user group returned by
// newUserGroup, with its namespace resolved to nsID.
func cloudSpec() *tcloudidentityv1.UserGroupSpec {
	return &tcloudidentityv1.UserGroupSpec{
		DisplayName: groupName,
		GroupType: &tcloudidentityv1.UserGroupSpec_CloudGroup{
			CloudGroup: &tcloudidentityv1.CloudGroupSpec{},
		},
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

func resolvedRefs(namespaceID string) tcloudv1alpha1.UserGroupReferences {
	return tcloudv1alpha1.UserGroupReferences{
		Namespaces: []cloudcommon.ResolvedReference{
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

func userGroupOf(res kttypes.Resource) *tcloudv1alpha1.UserGroup {
	return res.CR().(*tcloudv1alpha1.UserGroup)
}

func statusOf(res kttypes.Resource) cloudcommon.ResourceStatus {
	return userGroupOf(res).Status.Resource
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

// Kinds of error the kube-temporal controller handles differently.
const (
	kindRequeue  = "requeue"
	kindTerminal = "terminal"
)

// requireErrKind checks that err is of the supplied kind, or nil when kind is
// empty.
func requireErrKind(t *testing.T, kind string, err error) {
	t.Helper()
	switch kind {
	case "":
		require.NoError(t, err)
	case kindRequeue:
		requireRequeue(t, err)
	case kindTerminal:
		requireTerminal(t, err)
	default:
		t.Fatalf("unknown error kind %q", kind)
	}
}
