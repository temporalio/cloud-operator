package connection_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/temporalio/resource-operator/pkg/temporal/cloud/connection"
	"github.com/temporalio/resource-operator/pkg/temporal/cloud/connection/loader"
	"github.com/temporalio/resource-operator/pkg/testutil"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
)

const (
	testCloudAPIKey = "test-api-key"
)

func TestNew(t *testing.T) {
	tccOpts := testutil.NewCloudClientOptions()
	tccOpts.APIKey = testCloudAPIKey
	tcc, err := tcclient.New(*tccOpts)
	require.Nil(t, err)

	load := loader.New(
		loader.WithClientOptions(*tccOpts),
	)

	cases := []struct {
		name      string
		args      []connection.NewOption
		expClient *tcclient.Client
		expLoader *loader.Loader
		expErr    string
	}{
		{
			name:   "fail if no loader or tcclient provided",
			args:   []connection.NewOption{},
			expErr: "either use WithLoader or WithClient",
		},
		{
			name: "fail if both loader and tcclient provided",
			args: []connection.NewOption{
				connection.WithClient(tcc),
				connection.WithLoader(load),
			},
			expErr: "either use WithLoader or WithClient",
		},
		{
			name: "temporal cloud client override",
			args: []connection.NewOption{
				connection.WithClient(tcc),
			},
			expClient: tcc,
			expLoader: nil,
		},
		{
			name: "loader",
			args: []connection.NewOption{
				connection.WithLoader(load),
			},
			expClient: nil,
			expLoader: &load,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)
			got, err := connection.New(tc.args...)
			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				return
			}
			require.NotNil(got)
			if tc.expClient != nil {
				gotTCC, err := got.Client()
				require.Nil(err)
				require.NotNil(gotTCC)
				require.Equal(tc.expClient, gotTCC)
			}
			gotLoader := got.Loader()
			if tc.expLoader == nil {
				require.Nil(gotLoader)
			} else {
				require.NotNil(gotLoader)
				gotTCCOpts, err := gotLoader.ClientOptions()
				require.Nil(err)
				expClientOpts, err := tc.expLoader.ClientOptions()
				require.Nil(err)
				require.EqualExportedValues(expClientOpts, gotTCCOpts)
			}
		})
	}
}
