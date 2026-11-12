package common

// ResourceReferenceOrID contains *either* a Temporal Cloud resource identifier string
// *or* a ResourceReference that points to a cloud.temporal.io custom resource.
//
// +kubebuilder:validation:XValidation:rule="has(self.ref) != has(self.id)",message="exactly one of ref or id must be set"
type ResourceReferenceOrID struct {
	// Ref is an optional field that points at a cloud.temporal.io custom resource
	// +optional
	Ref *ResourceReference `json:"ref,omitempty"`
	// ID is the Temporal Cloud resource identifier.
	// +optional
	ID string `json:"id,omitempty"`
}

// ResourceReference points at a cloud.temporal.io custom resource.
type ResourceReference struct {
	// Namespace contains the optional Kubernetes Namespace in which to find
	// the custom resource. If empty, the Kubernetes Namespace containing the
	// referring object is used.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Name is the name of the custom resource.
	// +required
	Name string `json:"name"`
}
