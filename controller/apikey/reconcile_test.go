package apikey_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/apikey"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
	fakeserver "github.com/temporalio/cloud-operator/pkg/testutil/temporal/cloud/fake/server"
)

const (
	kubeNamespace = "apps"
	keyName       = "ci"
	keyUID        = k8stypes.UID("uid-ci")
	keyID         = "key-1"
	saName        = "ci"
	saID          = "sa-1"
	secretName    = "ci-api-key"
	token         = "tok-1"
)

var expiry = metav1.NewTime(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))

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

// newReconciler returns a Reconciler backed by a fake Temporal Cloud server
// and a fake Kubernetes client holding kubeObjs.
func newReconciler(
	t *testing.T,
	kubeObjs []runtime.Object,
	serverOpts ...fakeserver.NewOption,
) (*apikey.Reconciler, client.Client) {
	t.Helper()
	srv, err := fakeserver.New(serverOpts...)
	require.NoError(t, err)
	t.Cleanup(srv.Start())
	tcc, err := srv.Client()
	require.NoError(t, err)

	// base.WithKubeClient would add a connection manager that ignores the
	// default cloud client, so set the kube client directly.
	kc := kube.NewFakeClient(kubeObjs...)
	r := apikey.NewReconciler(base.WithDefaultCloudClient(*tcc))
	r.SetKubeClient(kc)
	r.SetKubeReader(kc)
	return r, kc
}

// newAPIKey returns the APIKey CR used by these tests.
func newAPIKey() *tcloudv1alpha1.APIKey {
	key := &tcloudv1alpha1.APIKey{}
	key.Namespace = kubeNamespace
	key.Name = keyName
	key.UID = keyUID
	key.Spec = tcloudv1alpha1.APIKeySpec{
		DisplayName: "ci",
		Description: "deploys",
		Owner: tcloudv1alpha1.APIKeyOwner{
			ServiceAccount: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: saName},
			},
		},
		ExpiryTime: expiry,
		Token: tcloudv1alpha1.APIKeyToken{
			SecretKeyRef: &tcloudv1alpha1.SecretKeyRef{Name: secretName, Key: "token"},
		},
	}
	return key
}

// ownerServiceAccount returns the ServiceAccount CR that owns the API key, in
// the supplied state.
func ownerServiceAccount(state cloudcommon.ResourceState) *tcloudv1alpha1.ServiceAccount {
	sa := &tcloudv1alpha1.ServiceAccount{}
	sa.Namespace = kubeNamespace
	sa.Name = saName
	sa.Status.Resource.ID = saID
	sa.Status.Resource.State = state
	return sa
}

// resolvedRefs returns the references of newAPIKey with its owner resolved.
func resolvedRefs() tcloudv1alpha1.APIKeyReferences {
	return tcloudv1alpha1.APIKeyReferences{
		ServiceAccount: &tcloudv1alpha1.ServiceAccountResolvedReference{
			Namespace: kubeNamespace, Name: saName, ID: saID,
		},
	}
}

// cloudSpec returns the Temporal Cloud spec for newAPIKey with its owner
// resolved to saID.
func cloudSpec() *tcloudidentityv1.ApiKeySpec {
	return &tcloudidentityv1.ApiKeySpec{
		OwnerId:     saID,
		OwnerType:   tcloudidentityv1.OwnerType_OWNER_TYPE_SERVICE_ACCOUNT,
		DisplayName: "ci",
		Description: "deploys",
		ExpiryTime:  timestamppb.New(expiry.Time),
	}
}

// tokenSecret returns the Secret `name` holding `data`, controlled by the
// APIKey with controllerUID, or by nothing when it's empty.
func tokenSecret(name string, controllerUID k8stypes.UID, data map[string]string) *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Namespace = kubeNamespace
	secret.Name = name
	secret.Data = map[string][]byte{}
	for k, v := range data {
		secret.Data[k] = []byte(v)
	}
	if controllerUID != "" {
		owner := newAPIKey()
		owner.Name = "owner-" + string(controllerUID)
		owner.UID = controllerUID
		secret.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(owner, tcloudv1alpha1.SchemeGroupVersion.WithKind("APIKey")),
		}
	}
	return secret
}

// readSecret returns the Secret `name` from kc, or nil when it doesn't exist.
func readSecret(t *testing.T, kc client.Client, name string) *corev1.Secret {
	t.Helper()
	secret := &corev1.Secret{}
	err := kc.Get(context.TODO(), client.ObjectKey{Namespace: kubeNamespace, Name: name}, secret)
	if apierrors.IsNotFound(err) {
		return nil
	}
	require.NoError(t, err)
	return secret
}
