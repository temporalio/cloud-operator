package loader

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"sync"

	tcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	"github.com/temporalio/kube-temporal/pkg/tlsutil"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
	kcorev1 "k8s.io/api/core/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/version"
)

// Loader can load a Temporal Cloud sdkclient.Client from a variety of sources,
// including a ConnectionSpec.
type Loader struct {
	kc  kclient.Client
	kns string

	tccOpts *tcclient.Options

	connSpec *tcloudv1alpha1.ConnectionSpec
}

// KubeClient returns the Kubernetes controller-runtime Client associated with
// the Loader.
func (l Loader) KubeClient() kclient.Client {
	return l.kc
}

// KubeNamespace returns the Kubernetes Namespace associated with the
// Loader.
func (l Loader) KubeNamespace() string {
	return l.kns
}

// ClientOptions returns the Temporal Cloud SDK client Options to use when
// dialing the Temporal Cloud client.
func (l *Loader) ClientOptions() (*tcclient.Options, error) {
	if l.tccOpts != nil {
		return l.tccOpts, nil
	}
	if l.connSpec == nil {
		return nil, fmt.Errorf("ConnectionSpec required")
	}
	return sync.OnceValues(func() (*tcclient.Options, error) {
		tccOpts, err := l.clientOptionsFromConnectionSpec()
		if err != nil {
			return nil, err
		}
		l.tccOpts = tccOpts
		return tccOpts, nil
	})()
}

// NewOption allows modifying the Loader returned from New.
type NewOption func(*Loader)

// WithKubeClient allows the caller to supply a Kubernetes client to use if
// looking up credentials in Kubernetes Secrets or ConfigMaps.
func WithKubeClient(kc kclient.Client) NewOption {
	return func(l *Loader) {
		l.kc = kc
	}
}

// WithKubeNamespace allows the caller to supply a Kubernetes Namespace to use
// if looking up credentials in Kubernetes Secrets or ConfigMaps.
func WithKubeNamespace(ns string) NewOption {
	return func(l *Loader) {
		l.kns = ns
	}
}

// WithClientOptions allows the caller to override the Options supplied to the
// Temporal Cloud sdkclient.New function when constructing the Temporal Cloud
// Client.
func WithClientOptions(tccOpts tcclient.Options) NewOption {
	return func(l *Loader) {
		l.tccOpts = &tccOpts
	}
}

// WithConnectionSpec sets the Loader's ConnectionSpec which informs the Loader
// where to find various credentials and connectivity information for the
// Temporal sdkclient.Client.
func WithConnectionSpec(spec tcloudv1alpha1.ConnectionSpec) NewOption {
	return func(l *Loader) {
		l.connSpec = &spec
	}
}

// New returns a Loader that knows how to load a Connection from a
// variety of sources, including a ConnectionSpec.
func New(opts ...NewOption) Loader {
	l := Loader{}
	for _, opt := range opts {
		opt(&l)
	}

	return l
}

// clientOptionsFromConnectionSpec returns Temporal Cloud
// sdkclient.Options constructed from the Loader's ConnectionSpec.
func (l *Loader) clientOptionsFromConnectionSpec() (
	*tcclient.Options, error,
) {
	spec := l.connSpec
	if spec == nil {
		return nil, fmt.Errorf("ConnectionSpec required")
	}

	tccOpts, err := l.clientOptionsFromAPIKeyCredentialsConfig(
		spec.Credentials.APIKey,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed getting Temporal Cloud client.Options from "+
				"APIKeyCredentialsConfig: %w",
			err,
		)
	}

	tccOpts.HostPort = spec.HostPort
	tccOpts.UserAgent = version.UserAgent()

	var tlsCfg *tls.Config

	tlsCfg, err = l.tlsConfigFromServerTLSConfig(spec.TLS)
	if err != nil {
		return nil, fmt.Errorf(
			"failed getting TLS Config from ServerTLSConfig: %w", err,
		)
	}

	// If the TLS ServerName has been set, make sure we set it in the
	// tls.Config struct in Temporal Cloud client.Options.TLSConfig.
	if tlsCfg == nil {
		if spec.TLS.ServerName != "" {
			tlsCfg = &tls.Config{}
			tlsCfg.ServerName = spec.TLS.ServerName
		}
	} else {
		if spec.TLS.ServerName != "" {
			tlsCfg.ServerName = spec.TLS.ServerName
		}
	}
	if tlsCfg != nil {
		tccOpts.TLSConfig = tlsCfg
	}
	return tccOpts, nil
}

// clientOptionsFromAPIKeyCredentialsConfig returns the configured Temporal
// Cloud client.Options struct from the supplied APIKeyCredentialsConfig.
func (l Loader) clientOptionsFromAPIKeyCredentialsConfig(
	cfg tcorev1.APIKeyCredentialsConfig,
) (*tcclient.Options, error) {
	switch {
	case cfg.SecretRef != nil:
		kc := l.KubeClient()
		if kc == nil {
			return nil, fmt.Errorf(
				"Loader requires KubeClient to load API Key from Secret",
			)
		}
		secretRef := cfg.SecretRef
		secretName := secretRef.Name
		if secretName == "" {
			return nil, fmt.Errorf("APIKey.SecretRef.Name cannot be empty")
		}
		secretKey := secretRef.Key
		if secretKey == "" {
			return nil, fmt.Errorf("APIKey.SecretRef.Key cannot be empty")
		}

		ctx := context.TODO()
		lookup := ktypes.NamespacedName{
			Name:      secretName,
			Namespace: l.KubeNamespace(),
		}
		var secret kcorev1.Secret
		err := kc.Get(ctx, lookup, &secret)
		if err != nil {
			return nil, fmt.Errorf(
				"failed getting Secret %q referenced by "+
					"APIKey.SecretRef : %w",
				secretName, err,
			)
		}
		apiKeyContents, ok := secret.Data[secretKey]
		if !ok || len(apiKeyContents) == 0 {
			return nil, fmt.Errorf(
				"secret %q referenced by APIKey.SecretRef "+
					"has no entry at key %q",
				secretName, secretKey,
			)
		}
		return &tcclient.Options{
			APIKey: string(apiKeyContents),
		}, nil
	case cfg.Path != "":
		path := cfg.Path
		// Initial check that the file at Path actually exists. We load the API
		// key with dynamic credentials to allow for adapting to rotated
		// tokens...
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf(
				"failed reading file contents from APIKey.Path: %w",
				err,
			)
		}
		return &tcclient.Options{
			APIKey: string(contents),
		}, nil
	default:
		return nil, fmt.Errorf("either SecretRef or Path must be non-empty")
	}
}

// tlsConfigFromServerTLSConfig returns the tls.Config constructed after
// examining the supplied tcorev1.ServerTLSConfig.
func (l Loader) tlsConfigFromServerTLSConfig(
	serverTLSCfg tcorev1.ServerTLSConfig,
) (
	*tls.Config,
	error,
) {
	spec := l.connSpec
	if spec.TLS.CACert == nil {
		return nil, nil
	}
	caCert, err := l.caCertFromCACertConfig(*spec.TLS.CACert)
	if err != nil {
		return nil, err
	}
	rootCAs, err := tlsutil.SystemCAPoolWithCert(caCert)
	if err != nil {
		return nil, err
	}
	if rootCAs == nil {
		// nothing to override, just let the system defaults be used.
		return nil, nil
	}
	return &tls.Config{
		RootCAs: rootCAs,
	}, nil
}

// caCertFromConnectionSpec returns the CA cert after examining the supplied
// tcorev1.CACertConfig.
func (l Loader) caCertFromCACertConfig(
	caCertCfg tcorev1.CACertConfig,
) ([]byte, error) {
	switch {
	case caCertCfg.SecretRef != nil:
		secretName := caCertCfg.SecretRef.Name
		if secretName == "" {
			return nil, fmt.Errorf("SecretRef.Name cannot be empty")
		}
		kc := l.KubeClient()
		if kc == nil {
			return nil, fmt.Errorf(
				"Loader requires KubeClient to load CA cert from Secret",
			)
		}
		ctx := context.TODO()
		lookup := ktypes.NamespacedName{
			Name:      secretName,
			Namespace: l.KubeNamespace(),
		}
		var secret kcorev1.Secret
		err := kc.Get(ctx, lookup, &secret)
		if err != nil {
			return nil, fmt.Errorf(
				"failed reading CA secret %q: %w", secretName, err,
			)
		}
		var secretKey string
		secretType := secret.Type
		switch secretType {
		case kcorev1.SecretTypeTLS:
			secretKey = "ca.crt"
		case kcorev1.SecretTypeOpaque, "":
			secretKey = caCertCfg.SecretRef.Key
			if secretKey == "" {
				return nil, fmt.Errorf(
					"SecretRef.Key cannot be empty for "+
						"Secret %q with opaque secret type",
					secretName,
				)
			}
		default:
			return nil, fmt.Errorf(
				"unsupported secret type %q for CA certificate Secret %q",
				secretType, secretName,
			)
		}
		caCert, ok := secret.Data[secretKey]
		if !ok || len(caCert) == 0 {
			return nil, fmt.Errorf(
				"CA secret %q referenced by tls.caCert.secretRef "+
					"has no entry at key %q",
				secretName, secretKey,
			)
		}
		return caCert, nil
	case caCertCfg.Path != "":
		caCert, err := os.ReadFile(caCertCfg.Path)
		if err != nil {
			return nil, fmt.Errorf(
				"failed reading CA cert from path: %w", err,
			)
		}
		return caCert, nil
	default:
		return nil, fmt.Errorf(
			"either CACert.SecretRef or CACert.Path must be set",
		)
	}
}
