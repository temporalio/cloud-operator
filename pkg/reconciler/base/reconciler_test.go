package base_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	conmgr "github.com/temporalio/cloud-operator/pkg/temporal/cloud/connection/manager"
	"github.com/temporalio/cloud-operator/pkg/testutil"
	tcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	kubeNamespace = "kube-temporal"
)

func TestReconciler_CloudClientFor(t *testing.T) {

	tnsName := "tmprl-ns"
	tns := func() *tcloudv1alpha1.Namespace {
		ns := &tcloudv1alpha1.Namespace{}
		ns.Name = tnsName
		ns.Namespace = kubeNamespace
		return ns
	}

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
		name    string
		r       *base.Reconciler
		subject any
		exp     tcloudservicev1.CloudServiceClient
		expErr  string
	}{
		{
			name:    "fail nil cloud connection manager or default client",
			r:       base.New(),
			subject: namespace.NewResource(tns()),
			expErr:  "connection manager nil and no default client set",
		},
		{
			name: "yields default client when no manager",
			r: base.New(
				base.WithDefaultCloudClient(*tcc),
			),
			subject: namespace.NewResource(tns()),
			exp:     tcc.CloudService(),
		},
		{
			name: "fail if subject not resource, string or conn spec",
			r: base.New(
				base.WithKubeClient(
					testutil.NewFakeClient(tns()),
				),
			),
			subject: int64(1),
			expErr:  "unsupported type int64",
		},
		{
			name: "fail if spec subject not in manager",
			r: base.New(
				base.WithDefaultCloudConnectionSpec(
					tcloudv1alpha1.ConnectionSpec{
						HostPort: "host:port",
					},
				),
			),
			subject: &tcloudv1alpha1.ConnectionSpec{
				HostPort: "differenthost:port",
				Credentials: tcloudv1alpha1.CredentialsConfig{
					APIKey: tcorev1.APIKeyCredentialsConfig{
						Path: apiKeyPath,
					},
				},
			},
			expErr: "no cloud client registered for spec",
		},
		{
			name: "spec subject yields registered client",
			r: func() *base.Reconciler {
				b := base.New(
					base.WithKubeClient(
						testutil.NewFakeClient(tns()),
					),
				)
				ccm := b.CloudConnectionManager()
				_ = ccm.Register(
					&tcloudv1alpha1.ConnectionSpec{
						HostPort: "host:port",
						Credentials: tcloudv1alpha1.CredentialsConfig{
							APIKey: tcorev1.APIKeyCredentialsConfig{
								Path: apiKeyPath,
							},
						},
					},
					conmgr.RegisterWithClient(tcc),
				)
				return b
			}(),
			subject: &tcloudv1alpha1.ConnectionSpec{
				HostPort: "host:port",
				Credentials: tcloudv1alpha1.CredentialsConfig{
					APIKey: tcorev1.APIKeyCredentialsConfig{
						Path: apiKeyPath,
					},
				},
			},
			exp: tcc.CloudService(),
		},
		{
			name: "string subject yields registered client",
			r: func() *base.Reconciler {
				b := base.New(
					base.WithKubeClient(
						testutil.NewFakeClient(tns()),
					),
				)
				ccm := b.CloudConnectionManager()
				_ = ccm.Register(
					"host:port",
					conmgr.RegisterWithClient(tcc),
				)
				return b
			}(),
			subject: "host:port",
			exp:     tcc.CloudService(),
		},
		{
			name: "fail if resource subject missing hostport annotation",
			r: func() *base.Reconciler {
				b := base.New(
					base.WithKubeClient(
						testutil.NewFakeClient(tns()),
					),
				)
				ccm := b.CloudConnectionManager()
				_ = ccm.Register(
					"host:port",
					conmgr.RegisterWithClient(tcc),
				)
				return b
			}(),
			subject: namespace.NewResource(tns()),
			expErr:  "no \"cloud.temporal.io/hostport\" annotation on resource",
		},
		{
			name: "resource subject with annotation yields registered client",
			r: func() *base.Reconciler {
				b := base.New(
					base.WithKubeClient(
						testutil.NewFakeClient(tns()),
					),
				)
				ccm := b.CloudConnectionManager()
				_ = ccm.Register(
					"host:port",
					conmgr.RegisterWithClient(tcc),
				)
				return b
			}(),
			subject: func() any {
				res := namespace.NewResource(tns())
				ro := res.RuntimeObject()
				metaAccessor := meta.NewAccessor()
				annos := map[string]string{
					tcloudcommon.AnnotationKeyTemporalCloudHostPort: "host:port",
				}
				metaAccessor.SetAnnotations(ro, annos)
				return res
			}(),

			exp: tcc.CloudService(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			ctx := context.TODO()

			got, err := tc.r.CloudClientFor(ctx, tc.subject)
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
