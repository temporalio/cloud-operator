package manager_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	tcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	tcloudv1alpha1 "github.com/temporalio/resource-operator/api/cloud/v1alpha1"
	"github.com/temporalio/resource-operator/pkg/temporal/cloud/connection/manager"
	"github.com/temporalio/resource-operator/pkg/testutil"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
)

func TestManager_Register(t *testing.T) {
	tcc, err := testutil.NewFakeCloudClient()
	require.Nil(t, err)

	cases := []struct {
		name    string
		newArgs []manager.NewOption
		subject any
		regOpts []manager.RegisterOption
		expErr  string
	}{
		{
			name:    "fail if subject is neither string nor conn spec",
			newArgs: []manager.NewOption{},
			subject: int64(1),
			expErr:  "unsupported type int64",
		},
		{
			name:    "fail if string subject has no reg opts",
			newArgs: []manager.NewOption{},
			subject: "host:port",
			expErr:  "connection specs or temporal cloud client required to register HostPort",
		},
		{
			name:    "fail if string subject has unmatched conn spec",
			newArgs: []manager.NewOption{},
			subject: "host:port",
			regOpts: []manager.RegisterOption{
				manager.RegisterWithConnectionSpecs(
					&tcloudv1alpha1.ConnectionSpec{
						HostPort: "differenthost:port",
					},
				),
			},
			expErr: "connection spec HostPort \"differenthost:port\" does not match subject \"host:port\"",
		},
		{
			name:    "string subject with conn spec",
			newArgs: []manager.NewOption{},
			subject: "host:port",
			regOpts: []manager.RegisterOption{
				manager.RegisterWithConnectionSpecs(
					&tcloudv1alpha1.ConnectionSpec{
						HostPort: "host:port",
					},
				),
			},
		},
		{
			name:    "string subject with client override",
			newArgs: []manager.NewOption{},
			subject: "host:port",
			regOpts: []manager.RegisterOption{
				manager.RegisterWithClient(tcc),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			m := manager.New(tc.newArgs...)

			err := m.Register(tc.subject, tc.regOpts...)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				return
			}
			require.Nil(err)
		})
	}
}

type registerFunc func(*manager.Manager) error

func TestManager_Client(t *testing.T) {
	tcc, err := testutil.NewFakeCloudClient()
	require.Nil(t, err)

	apiKeyDir := t.TempDir()
	apiKeyPath := filepath.Join(apiKeyDir, "api-key")

	err = os.WriteFile(
		apiKeyPath,
		[]byte(testutil.FakeTemporalCloudAPIKeyContents),
		0644,
	)
	require.Nil(t, err)

	cases := []struct {
		name     string
		m        *manager.Manager
		subject  any
		register []registerFunc
		expErr   string
		exp      *tcclient.Client
	}{
		{
			name:    "fail if subject is neither string nor conn spec",
			m:       manager.New(),
			subject: int64(1),
			expErr:  "unsupported type int64",
		},
		{
			name:    "fail if string subject has no client or spec registered",
			m:       manager.New(),
			subject: "host:port",
			expErr:  "no client or connection specs found for HostPort",
		},
		{
			name:    "fails string subject with invalid conn spec",
			m:       manager.New(),
			subject: "host:port",
			register: []registerFunc{
				func(m *manager.Manager) error {
					return m.Register(
						"host:port",
						manager.RegisterWithConnectionSpecs(
							&tcloudv1alpha1.ConnectionSpec{
								HostPort: "host:port",
								// This spec is not valid because it must contain
								// APIKey credentials
							},
						),
					)
				},
			},
			expErr: "APIKeyCredentialsConfig: either SecretRef or Path must be non-empty",
		},
		{
			name:    "string subject yields registered client",
			m:       manager.New(),
			subject: "host:port",
			register: []registerFunc{
				func(m *manager.Manager) error {
					return m.Register(
						"host:port",
						manager.RegisterWithClient(tcc),
					)
				},
			},
			exp: tcc,
		},
		{
			name: "fails spec subject with invalid conn spec",
			m:    manager.New(),
			subject: &tcloudv1alpha1.ConnectionSpec{
				HostPort: "host:port",
			},
			register: []registerFunc{
				func(m *manager.Manager) error {
					return m.Register(
						"host:port",
						manager.RegisterWithConnectionSpecs(
							&tcloudv1alpha1.ConnectionSpec{
								HostPort: "host:port",
								// This spec is not valid because it must contain
								// APIKey credentials
							},
						),
					)
				},
			},
			expErr: "APIKeyCredentialsConfig: either SecretRef or Path must be non-empty",
		},
		{
			name: "spec subject yields registered client",
			m:    manager.New(),
			subject: &tcloudv1alpha1.ConnectionSpec{
				HostPort: "host:port",
			},
			register: []registerFunc{
				func(m *manager.Manager) error {
					return m.Register(
						&tcloudv1alpha1.ConnectionSpec{
							HostPort: "host:port",
						},
						manager.RegisterWithClient(tcc),
					)
				},
			},
			exp: tcc,
		},
		{
			name: "spec subject yields registered client",
			m:    manager.New(),
			subject: &tcloudv1alpha1.ConnectionSpec{
				HostPort: "host:port",
			},
			register: []registerFunc{
				func(m *manager.Manager) error {
					return m.Register(
						&tcloudv1alpha1.ConnectionSpec{
							HostPort: "host:port",
						},
						manager.RegisterWithClient(tcc),
					)
				},
			},
			exp: tcc,
		},
		{
			name:    "string subject constructs client from registered conn spec",
			m:       manager.New(),
			subject: "host:port",
			register: []registerFunc{
				func(m *manager.Manager) error {
					return m.Register(
						"host:port",
						manager.RegisterWithConnectionSpecs(
							&tcloudv1alpha1.ConnectionSpec{
								HostPort: "host:port",
								Credentials: tcloudv1alpha1.CredentialsConfig{
									APIKey: tcorev1.APIKeyCredentialsConfig{
										Path: apiKeyPath,
									},
								},
							},
						),
					)
				},
			},
			exp: tcc,
		},
		{
			name: "spec subject constructs client from registered conn spec",
			m:    manager.New(),
			subject: &tcloudv1alpha1.ConnectionSpec{
				HostPort: "host:port",
				Credentials: tcloudv1alpha1.CredentialsConfig{
					APIKey: tcorev1.APIKeyCredentialsConfig{
						Path: apiKeyPath,
					},
				},
			},
			register: []registerFunc{
				func(m *manager.Manager) error {
					return m.Register(
						"host:port",
						manager.RegisterWithConnectionSpecs(
							&tcloudv1alpha1.ConnectionSpec{
								HostPort: "host:port",
								Credentials: tcloudv1alpha1.CredentialsConfig{
									APIKey: tcorev1.APIKeyCredentialsConfig{
										Path: apiKeyPath,
									},
								},
							},
						),
					)
				},
			},
			exp: tcc,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			for _, reg := range tc.register {
				err := reg(tc.m)
				require.Nil(err)
			}
			got, err := tc.m.Client(tc.subject)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				return
			}
			require.Nil(err)
			require.EqualExportedValues(tc.exp, got)
		})
	}
}
