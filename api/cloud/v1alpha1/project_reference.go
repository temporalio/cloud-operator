package v1alpha1

// ProjectReferenceOrID contains *either* a Temporal Cloud Project identifier string
// *or* a ProjectReference that points to a cloud.temporal.io/Project.
//
// +kubebuilder:validation:XValidation:rule="has(self.ref) != has(self.id)",message="exactly one of ref or id must be set"
type ProjectReferenceOrID struct {
	// Ref is an optional field that points at a cloud.temporal.io/Project.
	// +optional
	Ref *ProjectReference `json:"ref,omitempty"`
	// ID is the Temporal Cloud Project resource identifier
	// +optional
	ID string `json:"id,omitempty"`
}

// ProjectReference points at a cloud.temporal.io/Project.
type ProjectReference struct {
	// Namespace contains the optional Kubernetes Namespace in which to find
	// the Project. If empty, the Kubernetes Namespace containing the referring
	// object is used.
	Namespace string `json:"namespace,omitempty"`
	// Name is the name of the Project.
	// +required
	Name string `json:"name"`
}
