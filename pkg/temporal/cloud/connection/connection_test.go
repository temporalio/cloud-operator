package connection_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/temporalio/resource-operator/pkg/temporal/cloud/connection"
	"github.com/temporalio/resource-operator/pkg/version"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
)

const (
	testCloudAPIKey = "test-api-key"
)

func newTCCOptions() *tcclient.Options {
	return &tcclient.Options{
		UserAgent: version.UserAgent(),
	}
}

func TestNew(t *testing.T) {
	tccOpts := newTCCOptions()
	tccOpts.APIKey = testCloudAPIKey
	tcc, err := tcclient.New(*tccOpts)
	require.Nil(t, err)

	loader := connection.NewLoader(
		connection.WithTemporalCloudClientOptions(*tccOpts),
	)

	cases := []struct {
		name      string
		args      []connection.NewOption
		expTCC    *tcclient.Client
		expLoader *connection.Loader
		expErr    string
	}{
		{
			name:   "fail if no loader or tcclient provided",
			args:   []connection.NewOption{},
			expErr: "either use WithLoader or WithTemporalCloudClient",
		},
		{
			name: "fail if both loader and tcclient provided",
			args: []connection.NewOption{
				connection.WithTemporalCloudClient(tcc),
				connection.WithLoader(loader),
			},
			expErr: "either use WithLoader or WithTemporalCloudClient",
		},
		{
			name: "temporal cloud client override",
			args: []connection.NewOption{
				connection.WithTemporalCloudClient(tcc),
			},
			expTCC:    tcc,
			expLoader: nil,
		},
		{
			name: "loader",
			args: []connection.NewOption{
				connection.WithLoader(loader),
			},
			expTCC:    nil,
			expLoader: &loader,
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
			if tc.expTCC != nil {
				gotTCC, err := got.Client()
				require.Nil(err)
				require.NotNil(gotTCC)
				require.Equal(tc.expTCC, gotTCC)
			}
			gotLoader := got.Loader()
			if tc.expLoader == nil {
				require.Nil(gotLoader)
			} else {
				require.NotNil(gotLoader)
				gotTCCOpts, err := gotLoader.TemporalCloudClientOptions()
				require.Nil(err)
				expTCCOpts, err := tc.expLoader.TemporalCloudClientOptions()
				require.Nil(err)
				require.EqualExportedValues(expTCCOpts, gotTCCOpts)
			}
		})
	}
}
