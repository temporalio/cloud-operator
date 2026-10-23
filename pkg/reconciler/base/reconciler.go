package base

import (
	"context"
	"fmt"

	ctrlreqlog "github.com/temporalio/kube-temporal/pkg/controller/request/log"
	ktbase "github.com/temporalio/kube-temporal/pkg/reconciler/base"
	"github.com/temporalio/kube-temporal/pkg/types"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
	"k8s.io/apimachinery/pkg/api/meta"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	tcloudcommon "github.com/temporalio/resource-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/resource-operator/api/cloud/v1alpha1"
	conmgr "github.com/temporalio/resource-operator/pkg/temporal/cloud/connection/manager"
)

// NewOption modifies the Reconciler returned from New.
type NewOption func(*Reconciler)

// WithDefaultCloudConnectionSpec registers the supplied
// [tcloudv1alpha1.ConnectionSpec] object as the default cloud connection spec
// for the Reconciler.
func WithDefaultCloudConnectionSpec(
	spec tcloudv1alpha1.ConnectionSpec,
) NewOption {
	return func(r *Reconciler) {
		r.defaultCloudConnSpec = &spec
		ccm := r.ccManager
		if ccm == nil {
			ccm = conmgr.New()
			r.ccManager = ccm
		}
		// NOTE(jaypipes): Error-returning code paths in manager.Register are
		// not hittable here, so safe to ignore errors.
		_ = ccm.Register(&spec)
	}
}

// WithDefaultCloudClient registers the supplied [tcclient.Client] as the
// fallback default for the Reconciler.
func WithDefaultCloudClient(tcc tcclient.Client) NewOption {
	return func(r *Reconciler) {
		r.defaultCloudClient = &tcc
	}
}

// WithKubeClient sets the supplied Kubernetes client for the Reconciler.
func WithKubeClient(kc kclient.Client) NewOption {
	return func(r *Reconciler) {
		r.Reconciler.SetKubeClient(kc)
		// Propogate our kube client down to the cloud connection manager
		ccm := r.ccManager
		if ccm == nil {
			ccm = conmgr.New()
			r.ccManager = ccm
		}
		ccm.SetKubeClient(kc)
	}
}

// New returns a new base Reconciler.
func New(opts ...NewOption) *Reconciler {
	r := Reconciler{}
	for _, opt := range opts {
		opt(&r)
	}
	return &r
}

// Reconciler is the base struct for all controller-specific reconcilers in
// resource-operator. It contains common methods that all controller-specific
// reconcilers need, for example, to connect to a Temporal Cloud API endpoint.
type Reconciler struct {
	ktbase.Reconciler
	ccManager *conmgr.Manager

	// defaultCloudConnSpec contains a [tcloudv1alpha1.ConnectionSpec]
	// configured as a default for the Reconciler.
	defaultCloudConnSpec *tcloudv1alpha1.ConnectionSpec

	// defaultCloudClient contains the [tcclient.Client] configured as the
	// default fallback for the Reconciler.
	defaultCloudClient *tcclient.Client
}

// SetDefaultCloudConnectionSpec sets the [tcloudv1alpha1.ConnectionSpec] the
// Reconciler should use as a fallback default when fetching a Temporal Cloud
// client.
func (r *Reconciler) SetDefaultCloudConnectionSpec(
	spec tcloudv1alpha1.ConnectionSpec,
) {
	r.defaultCloudConnSpec = &spec
}

// SetDefaultCloudClient sets the [tcclient.Client] the Reconciler should use
// as a fallback default.
func (r *Reconciler) SetDefaultCloudClient(
	tcc tcclient.Client,
) {
	r.defaultCloudClient = &tcc
}

// CloudConnectionManager returns the Reconciler's manager of Temporal Cloud
// connections and clients.
func (r Reconciler) CloudConnectionManager() *conmgr.Manager {
	return r.ccManager
}

// SetCloudConnectionManager sets the Reconciler's manager of Temporal Cloud
// connections and clients.
func (r *Reconciler) SetCloudConnectionManager(ccm *conmgr.Manager) {
	r.ccManager = ccm
}

// CloudClientFor returns a [tcclient.Client] struct configured for the
// matching supplied subject.
//
// If a client is not found for the supplied subject, we fallback to any
// default client set in the Reconciler.
//
// If subject is a [tcloudv1.ConnectionSpec], we look up the client by the
// connection spec's hash key.
//
// If subject is a string, we look up the client by the HostPort. If no client
// is found, we find the first connection spec having that HostPort and then
// look up the client by that connection spec's hash key.
//
// If subject is a [types.Resource], we look for the
// cloud.temporal.io/hostport annotation and look up the client by the
// annotation value. If no client is found, we find the first connection spec
// having that annotation value and then look up the client by that connection
// spec's hash key.
func (r *Reconciler) CloudClientFor(
	ctx context.Context,
	subject any,
) (tcloudservicev1.CloudServiceClient, error) {
	if r.ccManager == nil && r.defaultCloudClient == nil {
		return nil, fmt.Errorf("connection manager nil and no default client set")
	}

	var err error
	log := ctrlreqlog.FromContext(ctx)
	exit := log.Trace("cloud.client.for")
	defer func() {
		exit(err)
	}()

	if r.ccManager == nil {
		log.Debug(
			"no connection manager, falling back to default cloud client",
		)
		return r.defaultCloudClient.CloudService(), nil
	}

	var cli *tcclient.Client

	switch subject := subject.(type) {
	case string, *tcloudv1alpha1.ConnectionSpec:
		cli, err = r.ccManager.Client(subject)
	case types.Resource:
		ro := subject.RuntimeObject()
		metaAccessor := meta.NewAccessor()
		annos, err := metaAccessor.Annotations(ro)
		if err != nil {
			return nil, fmt.Errorf("failed getting resource annotations: %w", err)
		}
		hp, ok := annos[tcloudcommon.AnnotationKeyTemporalCloudHostPort]
		if !ok {
			return nil, fmt.Errorf(
				"no %q annotation on resource",
				tcloudcommon.AnnotationKeyTemporalCloudHostPort,
			)
		}
		cli, err = r.ccManager.Client(hp)
	default:
		return nil, fmt.Errorf("unsupported type %T", subject)
	}
	if err != nil {
		return nil, err
	}
	return cli.CloudService(), nil
}
