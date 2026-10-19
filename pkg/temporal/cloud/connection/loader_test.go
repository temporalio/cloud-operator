package connection_test

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	tcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	"github.com/temporalio/kube-temporal/pkg/testutil"
	tcloudv1alpha1 "github.com/temporalio/resource-operator/api/cloud/v1alpha1"
	"github.com/temporalio/resource-operator/pkg/temporal/cloud/connection"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
	kcorev1 "k8s.io/api/core/v1"
	kcrfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	tlsCAName     = "test CA"
	tlsServerName = "localhost"
	tlsClientName = "test client"
	kubeNamespace = "kube-temporal"
)

var (
	once sync.Once
)

var validCAKeyPair = sync.OnceValues(
	func() (*testutil.TLSKeyPair, error) {
		return testutil.TLSNewCAKeyPair(tlsCAName)
	},
)

func TestLoader_ClientOptions(t *testing.T) {
	tccOpts := newTCCOptions()

	cases := []struct {
		name   string
		args   []connection.NewLoaderOption
		exp    *tcclient.Options
		expErr string
	}{
		{
			name:   "fail if no connection spec or tcclient opts",
			args:   []connection.NewLoaderOption{},
			expErr: "ConnectionSpec required",
		},
		{
			name: "tcclient opts override provided",
			args: []connection.NewLoaderOption{
				connection.WithTemporalCloudClientOptions(*tccOpts),
			},
			exp: tccOpts,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			loader := connection.NewLoader(tc.args...)
			got, err := loader.TemporalCloudClientOptions()
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				return
			}
			require.Nil(err)
			require.NotNil(got)
			require.EqualExportedValues(tc.exp, got)
		})
	}
}

func TestLoader_ClientOptions_ControllerSpec_APIKeyCredentials(t *testing.T) {
	customHostPort := "host:12345"

	caKP, err := validCAKeyPair()
	require.Nil(t, err)

	poolWithValidCACert, err := x509.SystemCertPool()
	require.Nil(t, err)
	added := poolWithValidCACert.AppendCertsFromPEM(caKP.Cert)
	require.True(t, added)

	tlsCACertSecretName := "tls-ca-cert-secret-name"
	tlsCACertOpaqueSecretDataKey := "ca.pem"

	tlsCACertOpaqueSecret := &kcorev1.Secret{}
	tlsCACertOpaqueSecret.Name = tlsCACertSecretName
	tlsCACertOpaqueSecret.Data = map[string][]byte{
		tlsCACertOpaqueSecretDataKey: caKP.Cert,
	}

	tlsCACertInvalidSecret := &kcorev1.Secret{}
	tlsCACertInvalidSecret.Name = tlsCACertSecretName
	tlsCACertInvalidSecret.Data = map[string][]byte{
		tlsCACertOpaqueSecretDataKey: []byte("invalid cert"),
	}

	tlsCACertTLSSecret := &kcorev1.Secret{}
	tlsCACertTLSSecret.Type = kcorev1.SecretTypeTLS
	tlsCACertTLSSecret.Name = tlsCACertSecretName
	tlsCACertTLSSecret.Data = map[string][]byte{
		"ca.crt": caKP.Cert,
	}
	apiKeySecretName := "api-key-secret-name"
	apiKeySecretDataKey := "api-key-secret-data-key"
	apiKeySecretDataValue := "api-key-secret-data-value"

	apiKeySecret := &kcorev1.Secret{}
	apiKeySecret.Name = apiKeySecretName
	apiKeySecret.Data = map[string][]byte{
		apiKeySecretDataKey: []byte(apiKeySecretDataValue),
	}

	apiKeySecretWithNamespace := &kcorev1.Secret{}
	apiKeySecretWithNamespace.Name = apiKeySecretName
	apiKeySecretWithNamespace.Namespace = kubeNamespace
	apiKeySecretWithNamespace.Data = map[string][]byte{
		apiKeySecretDataKey: []byte(apiKeySecretDataValue),
	}

	apiKeyDir := t.TempDir()
	apiKeyPath := filepath.Join(apiKeyDir, "api-key")

	err = os.WriteFile(apiKeyPath, []byte(apiKeySecretDataValue), 0644)
	require.Nil(t, err)

	cases := []struct {
		name   string
		args   []connection.NewLoaderOption
		exp    *tcclient.Options
		expErr string
	}{
		{
			name: "fail if no api key secret ref or path",
			args: []connection.NewLoaderOption{
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{},
				),
			},
			expErr: "either SecretRef or Path must be non-empty",
		},
		{
			name: "fail if api key secret no kube client",
			args: []connection.NewLoaderOption{
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: apiKeySecretName,
									},
									Key: apiKeySecretDataKey,
								},
							},
						},
					},
				),
			},
			expErr: "Loader requires KubeClient to load API Key from Secret",
		},
		{
			name: "fail if api key secret no such secret",
			args: []connection.NewLoaderOption{
				connection.WithKubeClient(
					kcrfake.NewFakeClient(),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: apiKeySecretName,
									},
									Key: apiKeySecretDataKey,
								},
							},
						},
					},
				),
			},
			expErr: "secrets \"api-key-secret-name\" not found",
		},
		{
			name: "fail if api key secret no such secret in namespace",
			args: []connection.NewLoaderOption{
				connection.WithKubeNamespace(kubeNamespace),
				connection.WithKubeClient(
					// This is the non-namespaced secret, which should not
					// match the lookup with the kubeNamespace override from
					// the loader options.
					kcrfake.NewFakeClient(apiKeySecret),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: apiKeySecretName,
									},
									Key: apiKeySecretDataKey,
								},
							},
						},
					},
				),
			},
			expErr: "secrets \"api-key-secret-name\" not found",
		},
		{
			name: "api key secret",
			args: []connection.NewLoaderOption{
				connection.WithKubeClient(
					kcrfake.NewFakeClient(apiKeySecret),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: apiKeySecretName,
									},
									Key: apiKeySecretDataKey,
								},
							},
						},
					},
				),
			},
			exp: func() *tcclient.Options {
				tcco := newTCCOptions()
				tcco.APIKey = apiKeySecretDataValue
				return tcco
			}(),
		},
		{
			name: "api key secret in namespace",
			args: []connection.NewLoaderOption{
				connection.WithKubeNamespace(kubeNamespace),
				connection.WithKubeClient(
					kcrfake.NewFakeClient(apiKeySecretWithNamespace),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: apiKeySecretName,
									},
									Key: apiKeySecretDataKey,
								},
							},
						},
					},
				),
			},
			exp: func() *tcclient.Options {
				tcco := newTCCOptions()
				tcco.APIKey = apiKeySecretDataValue
				return tcco
			}(),
		},
		{
			name: "api key secret with custom host port",
			args: []connection.NewLoaderOption{
				connection.WithKubeClient(
					kcrfake.NewFakeClient(apiKeySecret),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						HostPort: customHostPort,
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: apiKeySecretName,
									},
									Key: apiKeySecretDataKey,
								},
							},
						},
					},
				),
			},
			exp: func() *tcclient.Options {
				tcco := newTCCOptions()
				tcco.HostPort = customHostPort
				tcco.APIKey = apiKeySecretDataValue
				return tcco
			}(),
		},
		{
			name: "api key secret with tls server name",
			args: []connection.NewLoaderOption{
				connection.WithKubeClient(
					kcrfake.NewFakeClient(apiKeySecret),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						TLS: tcorev1.ServerTLSConfig{
							ServerName: tlsServerName,
						},
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: apiKeySecretName,
									},
									Key: apiKeySecretDataKey,
								},
							},
						},
					},
				),
			},
			exp: func() *tcclient.Options {
				tcco := newTCCOptions()
				tcco.TLSConfig = &tls.Config{
					ServerName: tlsServerName,
				}
				tcco.APIKey = apiKeySecretDataValue
				return tcco
			}(),
		},
		{
			name: "fail if api key path not found",
			args: []connection.NewLoaderOption{
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								Path: "path/does/not/exist",
							},
						},
					},
				),
			},
			expErr: "path/does/not/exist: no such file or directory",
		},
		{
			name: "api key path",
			args: []connection.NewLoaderOption{
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								Path: apiKeyPath,
							},
						},
					},
				),
			},
			exp: func() *tcclient.Options {
				tcco := newTCCOptions()
				tcco.APIKey = apiKeySecretDataValue
				return tcco
			}(),
		},
		{
			name: "api key path with tls server name",
			args: []connection.NewLoaderOption{
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						TLS: tcorev1.ServerTLSConfig{
							ServerName: tlsServerName,
						},
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								Path: apiKeyPath,
							},
						},
					},
				),
			},
			exp: func() *tcclient.Options {
				tcco := newTCCOptions()
				tcco.APIKey = apiKeySecretDataValue
				tcco.TLSConfig = &tls.Config{
					ServerName: tlsServerName,
				}
				return tcco
			}(),
		},
		{
			name: "fail tls CA cert secret no such secret",
			args: []connection.NewLoaderOption{
				connection.WithKubeClient(
					kcrfake.NewFakeClient(),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								Path: apiKeyPath,
							},
						},
						TLS: tcorev1.ServerTLSConfig{
							CACert: &tcorev1.CACertConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: tlsCACertSecretName,
									},
									Key: tlsCACertOpaqueSecretDataKey,
								},
							},
						},
					},
				),
			},
			expErr: "secrets \"tls-ca-cert-secret-name\" not found",
		},
		{
			name: "fail tls CA cert secret invalid cert",
			args: []connection.NewLoaderOption{
				connection.WithKubeClient(
					kcrfake.NewFakeClient(tlsCACertInvalidSecret),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								Path: apiKeyPath,
							},
						},
						TLS: tcorev1.ServerTLSConfig{
							CACert: &tcorev1.CACertConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: tlsCACertSecretName,
									},
									Key: tlsCACertOpaqueSecretDataKey,
								},
							},
						},
					},
				),
			},
			expErr: "failed appending CA cert to root CAs",
		},
		{
			name: "api key with tls CA cert opaque secret type",
			args: []connection.NewLoaderOption{
				connection.WithKubeClient(
					kcrfake.NewFakeClient(tlsCACertOpaqueSecret),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								Path: apiKeyPath,
							},
						},
						TLS: tcorev1.ServerTLSConfig{
							CACert: &tcorev1.CACertConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: tlsCACertSecretName,
									},
									Key: tlsCACertOpaqueSecretDataKey,
								},
							},
						},
					},
				),
			},
			exp: func() *tcclient.Options {
				tcco := newTCCOptions()
				tcco.APIKey = apiKeySecretDataValue
				tcco.TLSConfig = &tls.Config{
					RootCAs: poolWithValidCACert,
				}
				return tcco
			}(),
		},
		{
			name: "api key with tls CA cert tls secret tpe",
			args: []connection.NewLoaderOption{
				connection.WithKubeClient(
					kcrfake.NewFakeClient(tlsCACertTLSSecret),
				),
				connection.WithConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								Path: apiKeyPath,
							},
						},
						TLS: tcorev1.ServerTLSConfig{
							CACert: &tcorev1.CACertConfig{
								SecretRef: &kcorev1.SecretKeySelector{
									LocalObjectReference: kcorev1.LocalObjectReference{
										Name: tlsCACertSecretName,
									},
									// NOTE(jaypipes): Deliberately leave Key
									// field empty since this Secret is of type
									// kubernetes.io/tls and we want to verify
									// the autoselecting of the fixed `ca.crt`
									// data key.
								},
							},
						},
					},
				),
			},
			exp: func() *tcclient.Options {
				tcco := newTCCOptions()
				tcco.APIKey = apiKeySecretDataValue
				tcco.TLSConfig = &tls.Config{
					RootCAs: poolWithValidCACert,
				}
				return tcco
			}(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			loader := connection.NewLoader(tc.args...)
			got, err := loader.TemporalCloudClientOptions()
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				return
			}
			require.Nil(err)
			require.NotNil(got)
			require.EqualExportedValues(tc.exp, got)

			if tc.exp.TLSConfig != nil {
				require.NotNil(got.TLSConfig)
				require.True(
					tc.exp.TLSConfig.RootCAs.Equal(
						got.TLSConfig.RootCAs,
					),
				)
			}
		})
	}
}
