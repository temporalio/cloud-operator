package v1alpha1

import (
	"crypto/md5"

	tcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
)

// CredentialsConfig contains instructions as to where Temporal Cloud client
// credentials can be found.
type CredentialsConfig struct {
	// APIKey contains the optional configuration for connecting to a Temporal
	// API server using Temporal API Key authentication.
	APIKey tcorev1.APIKeyCredentialsConfig `json:"apiKey"`
}

// Hash returns a deterministic MD5 hash of the CredentialsConfig's field
// components.
func (c CredentialsConfig) Hash() []byte {
	out := md5.New()
	out.Write([]byte{'|'})
	out.Write(c.APIKey.Hash())
	return out.Sum(nil)
}
