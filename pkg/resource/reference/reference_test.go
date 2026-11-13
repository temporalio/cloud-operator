package reference_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	ctrlerrors "github.com/temporalio/kube-temporal/pkg/controller/errors"
	"github.com/temporalio/kube-temporal/pkg/controller/requeue"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/resource/reference"
	"github.com/temporalio/cloud-operator/pkg/testutil/kube"
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

func TestResolveNamespaceID(t *testing.T) {
	const referrerNamespace = "apps"

	newNamespace := func(
		kubeNamespace, name, id string,
		state cloudcommon.ResourceState,
	) *tcloudv1alpha1.Namespace {
		ns := &tcloudv1alpha1.Namespace{}
		ns.Namespace = kubeNamespace
		ns.Name = name
		ns.Status.Resource.ID = id
		ns.Status.Resource.State = state
		return ns
	}
	kc := kube.NewFakeClient(
		newNamespace(referrerNamespace, "payments", "payments.acct1", cloudcommon.ResourceStateActive),
		newNamespace("shared", "billing", "billing.acct1", cloudcommon.ResourceStateActive),
		newNamespace(referrerNamespace, "pending", "", ""),
		newNamespace(referrerNamespace, "activating", "activating.acct1", cloudcommon.ResourceStateActivating),
	)

	cases := []struct {
		name        string
		refOrID     cloudcommon.ResourceReferenceOrID
		exp         string
		expNotReady bool
	}{
		{
			name:    "id is returned as is",
			refOrID: cloudcommon.ResourceReferenceOrID{ID: "legacy.acct1"},
			exp:     "legacy.acct1",
		},
		{
			name: "ref without namespace uses the referrer's",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "payments"},
			},
			exp: "payments.acct1",
		},
		{
			name: "ref with namespace",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Namespace: "shared", Name: "billing"},
			},
			exp: "billing.acct1",
		},
		{
			name: "ref to a missing custom resource is not ready",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "nope"},
			},
			expNotReady: true,
		},
		{
			name: "ref to a custom resource without an ID is not ready",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "pending"},
			},
			expNotReady: true,
		},
		{
			name: "ref to a namespace that is still activating is not ready",
			refOrID: cloudcommon.ResourceReferenceOrID{
				Ref: &cloudcommon.ResourceReference{Name: "activating"},
			},
			expNotReady: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := reference.ResolveNamespaceID(context.TODO(), kc, referrerNamespace, tc.refOrID)
			if tc.expNotReady {
				require.True(t, errors.Is(err, reference.ErrNotReady), "got %v", err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.exp, got)
		})
	}
}

func TestReferenceCheck(t *testing.T) {
	ref := func(namespace, name string) cloudcommon.ResourceReferenceOrID {
		return cloudcommon.ResourceReferenceOrID{
			Ref: &cloudcommon.ResourceReference{Namespace: namespace, Name: name},
		}
	}
	type pending struct {
		path     string
		refOrID  cloudcommon.ResourceReferenceOrID
		resolved string
	}
	type unsupported struct {
		path    string
		refOrID cloudcommon.ResourceReferenceOrID
	}

	cases := []struct {
		name        string
		pending     []pending
		unsupported []unsupported
		expKind     string
		expErr      string
	}{
		{
			name:    "nothing recorded",
			expKind: kindNil,
		},
		{
			name: "ids and resolved refs are fine",
			pending: []pending{
				{path: "spec.a", refOrID: cloudcommon.ResourceReferenceOrID{ID: "a.acct"}},
				{path: "spec.b", refOrID: ref("", "b"), resolved: "b.acct"},
			},
			unsupported: []unsupported{
				{path: "spec.c", refOrID: cloudcommon.ResourceReferenceOrID{ID: "p-1"}},
			},
			expKind: kindNil,
		},
		{
			name: "unresolved ref waits and names it",
			pending: []pending{
				{path: "spec.a", refOrID: ref("", "payments")},
				{path: "spec.b", refOrID: ref("shared", "billing")},
			},
			expKind: kindRequeue,
			expErr:  "spec.a (apps/payments), spec.b (shared/billing)",
		},
		{
			name: "unsupported ref is terminal, even with pending refs",
			pending: []pending{
				{path: "spec.a", refOrID: ref("", "payments")},
			},
			unsupported: []unsupported{
				{path: "spec.project", refOrID: ref("", "payments")},
			},
			expKind: kindTerminal,
			expErr:  "set id instead: spec.project",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := reference.NewCheck("apps")
			for _, p := range tc.pending {
				check.Pending(p.path, p.refOrID, p.resolved)
			}
			for _, u := range tc.unsupported {
				check.Unsupported(u.path, u.refOrID)
			}

			err := check.Err()
			require.Equal(t, tc.expKind, errKind(err), "err: %v", err)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			}
		})
	}
}
