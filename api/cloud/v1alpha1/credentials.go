package v1alpha1

import (
	tcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
)

// CredentialsConfig contains instructions as to where Temporal Cloud client
// credentials can be found.
type CredentialsConfig struct {
	// APIKey contains the optional configuration for connecting to a Temporal
	// API server using Temporal API Key authentication.
	APIKey tcorev1.APIKeyCredentialsConfig `json:"apiKey"`
}
