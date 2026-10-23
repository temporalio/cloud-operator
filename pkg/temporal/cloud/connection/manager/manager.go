package manager

import (
	"fmt"
	"sync"

	"github.com/samber/lo"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/temporalio/cloud-operator/pkg/temporal/cloud/connection"
	"github.com/temporalio/cloud-operator/pkg/temporal/cloud/connection/loader"
)

// NewOption adapts the Manager returned from New.
type NewOption func(*Manager)

// WithKubeClient sets the Manager's Kubernetes client.
func WithKubeClient(kc kclient.Client) NewOption {
	return func(m *Manager) {
		m.kc = kc
	}
}

// New returns a new Temporal Cloud connection manager.
func New(opts ...NewOption) *Manager {
	m := Manager{
		hostPortConnSpecs: map[string][]*tcloudv1alpha1.ConnectionSpec{},
		hostPortClients:   map[string]*tcclient.Client{},
		connSpecs:         map[string]*tcloudv1alpha1.ConnectionSpec{},
		clients:           map[string]*tcclient.Client{},
	}
	for _, opt := range opts {
		opt(&m)
	}
	return &m
}

// Manager manages a cache of Temporal Cloud connection-related objects.
type Manager struct {
	sync.RWMutex
	// kc contains the Kubernetes client set on the Manager.
	kc kclient.Client
	// hostPortConnSpecs contains a map, keyed by HostPort string, of slices of
	// [tcloudv1alpha1.ConnectionSpec] objects that contain instructions for
	// creating cloud clients to that HostPort.
	hostPortConnSpecs map[string][]*tcloudv1alpha1.ConnectionSpec
	// hostPortClients contains a map, keyed by HostPort string, of
	// [tcclient.Client] objects associated with the HostPort.
	hostPortClients map[string]*tcclient.Client
	// connSpecs contains the set of [tcloudv1alpha1.ConnectionSpec] objects
	// associated with the hash of a [tcloudv1alpha1.ConnectionSpec].
	connSpecs map[string]*tcloudv1alpha1.ConnectionSpec
	// clients contains the set of [tccClient.Client] objects associated
	// with the hash of a [tcloudv1alpha1.ConnectionSpec].
	clients map[string]*tcclient.Client
}

// SetKubeClient sets the Manager's Kubernetes client.
func (m *Manager) SetKubeClient(kc kclient.Client) {
	m.kc = kc
}

type registerOptions struct {
	tcc       *tcclient.Client
	connSpecs []*tcloudv1alpha1.ConnectionSpec
}

// RegisterOption allows callers to the Manager.Register function to control
// what is being registered with the manager.
type RegisterOption func(*registerOptions)

// RegisterWithClient allows overriding the Temporal Cloud client associated
// with the Register subject.
func RegisterWithClient(tcc *tcclient.Client) RegisterOption {
	return func(o *registerOptions) {
		o.tcc = tcc
	}
}

// RegisterWithConnectionSpecs allows callers to the Manager.Register function
// to associate one or more [tcloudv1alpha1.ConnectionSpec] objects with the
// HostPort being registered.
func RegisterWithConnectionSpecs(
	specs ...*tcloudv1alpha1.ConnectionSpec,
) RegisterOption {
	return func(o *registerOptions) {
		o.connSpecs = append(o.connSpecs, specs...)
	}
}

// Register safely adds the supplied subject to the Manager's various caches.
//
// If subject is a [tcloudv1alpha1.ConnectionSpec], the Manager map's the
// spec's host port to the connection spec.
//
// If subject is a string, it is assumed to be a HostPort and at least one
// RegisterOption should be provided that associates the HostPort to a Temporal
// Cloud client or a set of related connection specs.
func (m *Manager) Register(subject any, opts ...RegisterOption) error {
	ro := registerOptions{}
	for _, opt := range opts {
		opt(&ro)
	}

	specs := []*tcloudv1alpha1.ConnectionSpec{}
	switch subject := subject.(type) {
	case *tcloudv1alpha1.ConnectionSpec:
		specHash := string(subject.Hash())
		m.RLock()
		_, ok := m.connSpecs[specHash]
		m.RUnlock()

		if ok {
			// We already know about this subject
			return nil
		}

		specs = append(specs, subject)
	case string:
		m.RLock()
		_, ok := m.hostPortConnSpecs[subject]
		m.RUnlock()
		if ok {
			// We already know about this subject
			return nil
		}

		m.RLock()
		// The HostPort may have been previously registered with no connection
		// specs and just a pre-created Temporal Cloud Client.
		_, ok = m.hostPortClients[subject]
		m.RUnlock()
		if ok {
			// We already know about this subject
			return nil
		}

		if ro.tcc == nil && len(ro.connSpecs) == 0 {
			return fmt.Errorf(
				"connection specs or temporal cloud client " +
					"required to register HostPort",
			)
		}

		// Ensure that all the supplied connection specs actually use the
		// subject HostPort.
		for _, spec := range ro.connSpecs {
			hp := spec.HostPort
			if hp == "" {
				hp = tcloudv1alpha1.DefaultConnectionHostPort
			}
			if hp != subject {
				return fmt.Errorf(
					"connection spec HostPort %q does not match subject %q",
					hp, subject,
				)
			}
		}
		specs = ro.connSpecs
	default:
		return fmt.Errorf("unsupported type %T", subject)
	}

	m.Lock()
	defer m.Unlock()

	if len(specs) == 0 {
		// We shouldn't get here, but just in case...
		if ro.tcc == nil {
			return fmt.Errorf(
				"connection specs or temporal cloud client " +
					"required to register HostPort",
			)
		}
		m.hostPortClients[subject.(string)] = ro.tcc
		return nil
	}

	for _, spec := range specs {
		hp := spec.HostPort
		if hp == "" {
			hp = tcloudv1alpha1.DefaultConnectionHostPort
		}

		specHash := string(spec.Hash())

		m.connSpecs[specHash] = spec
		if ro.tcc != nil {
			m.clients[specHash] = ro.tcc
		}

		hpcs := m.hostPortConnSpecs[hp]
		if len(hpcs) == 0 {
			hpcs = []*tcloudv1alpha1.ConnectionSpec{}
		}
		hpcs = append(hpcs, spec)
		m.hostPortConnSpecs[hp] = hpcs
		if ro.tcc != nil {
			m.hostPortClients[hp] = ro.tcc
		}
	}
	return nil
}

// Client returns a [tcclient.Client] struct configured for the
// matching the supplied subject.
//
// If a cached client is not found for the supplied subject, we attempt to
// create the client and cache the created client in the Manager.
//
// If subject is a [tcloudv1.ConnectionSpec], we look up the client by the
// connection spec's hash key.
//
// If subject is a string, we find the first known connection spec having that
// HostPort and then look up the client by that connection spec's hash key.
func (m *Manager) Client(subject any) (*tcclient.Client, error) {
	var specHash string
	var spec *tcloudv1alpha1.ConnectionSpec

	switch subject := subject.(type) {
	case *tcloudv1alpha1.ConnectionSpec:
		// Caller passed a ConnectionSpec. If we have a pre-created Temporal
		// Cloud client for this connection spec, just return it. Otherwise, we
		// fall through and attempt to create client from the supplied
		// connection spec that contains instructions to create a client for
		// this HostPort.
		if lo.IsNil(subject) {
			return nil, fmt.Errorf("nil cloud connection spec")
		}

		specHash = string(subject.Hash())

		m.RLock()
		cli, ok := m.clients[specHash]
		m.RUnlock()
		if ok {
			return cli, nil
		}

		m.RLock()
		_, ok = m.connSpecs[specHash]
		m.RUnlock()
		if !ok {
			return nil, fmt.Errorf("no cloud client registered for spec")
		}
		spec = subject
	case string:
		// Caller passed a HostPort. If we have a pre-created Temporal Cloud
		// client for this HostPort, just return it. Otherwise, we check to see
		// if we have any connection specs that contain instructions to create
		// a client for this HostPort.

		m.RLock()
		cli, ok := m.hostPortClients[subject]
		m.RUnlock()
		if ok {
			return cli, nil
		}

		// We don't have a cached Temporal Cloud for this HostPort, so get the
		// first ConnectionSpec associated with the supplied HostPort.
		m.RLock()
		specs, ok := m.hostPortConnSpecs[subject]
		m.RUnlock()
		if !ok || len(specs) == 0 {
			return nil, fmt.Errorf(
				"no client or connection specs found for HostPort %q", subject,
			)
		}
		spec = specs[0]
		specHash = string(spec.Hash())
	default:
		return nil, fmt.Errorf(
			"unsupported type %T", subject,
		)

	}

	m.RLock()
	cli, ok := m.clients[specHash]
	m.RUnlock()
	if ok {
		return cli, nil
	}

	// lazily create the Temporal Cloud client.

	m.Lock()
	defer m.Unlock()

	conn, err := connection.New(
		connection.WithLoader(
			loader.New(
				loader.WithKubeClient(m.kc),
				loader.WithConnectionSpec(*spec),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed creating cloud connection: %w", err,
		)
	}
	tc, err := conn.Client()
	if err != nil {
		return nil, fmt.Errorf(
			"failed getting cloud client: %w", err,
		)
	}
	m.clients[specHash] = tc
	return tc, nil
}
