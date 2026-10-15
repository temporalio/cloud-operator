package common

// LifecycleSpec configures lifecycle settings for a Temporal Cloud resource.
// Maps to Temporal Cloud proto `LifecycleSpec`.
type LifecycleSpec struct {
	// EnableDeleteProtection prevents the resource from being deleted.
	// Maps to Temporal Cloud proto `LifecycleSpec.enable_delete_protection`.
	// +optional
	EnableDeleteProtection bool `json:"enableDeleteProtection,omitempty"`
}
