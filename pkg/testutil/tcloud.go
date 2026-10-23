package testutil

import (
	tcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	"github.com/temporalio/cloud-operator/pkg/version"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
)

const (
	FakeTemporalCloudHostPort       = "fake.saas-api.tmprl.cloud:443"
	FakeTemporalCloudAPIKeyContents = "indy!bad-dates!"
)

// NewFakeCloudConnectionSpec returns a tcloudv1alpha1.ConnectionSpec for
// connecting to a fake Temporal Cloud API endpoint.
func NewFakeCloudConnectionSpec(
	apiKeyPath string,
) *tcloudv1alpha1.ConnectionSpec {
	return &tcloudv1alpha1.ConnectionSpec{
		HostPort: FakeTemporalCloudHostPort,
		Credentials: tcloudv1alpha1.CredentialsConfig{
			APIKey: tcorev1.APIKeyCredentialsConfig{
				Path: apiKeyPath,
			},
		},
	}
}

// NewCloudClientOptions returns new Temporal Cloud client options configured
// with the UserAgent set to the testing user-agent.
func NewCloudClientOptions() *tcclient.Options {
	return &tcclient.Options{
		UserAgent: version.UserAgent(),
	}
}

// NewFakeCloudClient returns a new Temporal Cloud client configured to connect
// to a fake Temporal Cloud API endpoint.
func NewFakeCloudClient() (*tcclient.Client, error) {
	opts := NewCloudClientOptions()
	opts.HostPort = FakeTemporalCloudHostPort
	opts.APIKey = FakeTemporalCloudAPIKeyContents
	return tcclient.New(*opts)
}
