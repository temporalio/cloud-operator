package v1alpha1

import (
	tcloudprojectv1 "go.temporal.io/cloud-sdk/api/project/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmprlcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	"github.com/temporalio/kube-temporal/pkg/kube/transform"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// Project represents an instance of a Temporal Cloud Project.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.resource.id`
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.resource.state`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Project struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Project.
	// +required
	Spec ProjectSpec `json:"spec"`
	// +optional
	// Status describes the current status of a Project.
	Status ProjectStatus `json:"status,omitempty"`
}

// FromProto returns a Project from the supplied Temporal Cloud Project proto
// message.
//
// The status fields live on the outer Project message rather than its spec,
// so they are set here. Object metadata is left unchanged.
func (p Project) FromProto(in *tcloudprojectv1.Project) *Project {
	out := Project{
		TypeMeta:   p.TypeMeta,
		ObjectMeta: p.ObjectMeta,
	}

	if in == nil {
		return &out
	}

	var spec ProjectSpec
	out.Spec = spec.FromProto(in.GetSpec())

	out.Status.Resource.ID = in.GetId()
	out.Status.Resource.Version = in.GetResourceVersion()
	out.Status.Resource.State.FromProto(in.GetState())
	out.Status.Resource.AsyncOperationID = in.GetAsyncOperationId()
	out.Status.Resource.CreatedTime = transform.TimeFromProto(in.GetCreatedTime())
	out.Status.Resource.LastModifiedTime = transform.TimeFromProto(in.GetLastModifiedTime())

	return &out
}

// ToProto returns the Temporal Cloud Project that represents the spec. It
// sets only the fields we send to Temporal Cloud, which is just the spec.
// Requests take the project ID and resource version as separate fields, so
// callers read those from Status.Resource. Status is never sent, so it is
// left unset.
func (p Project) ToProto() *tcloudprojectv1.Project {
	return &tcloudprojectv1.Project{
		Spec: p.Spec.toProto(),
	}
}

// ProjectSpec describes the desired state of a Temporal Cloud Project.
//
// Ref: https://github.com/temporalio/cloud-api/blob/main/temporal/api/cloud/project/v1/message.proto
// +k8s:openapi-gen=true
type ProjectSpec struct {
	// DisplayName is the human-readable name of the Project.
	// Maps to Temporal Cloud proto `ProjectSpec.display_name`.
	// +required
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`
	// Description is a human-readable description of the Project.
	// Maps to Temporal Cloud proto `ProjectSpec.description`.
	// +optional
	Description string `json:"description,omitempty"`
	// Lifecycle configures lifecycle settings.
	// Maps to Temporal Cloud proto `ProjectSpec.lifecycle`.
	// +optional
	Lifecycle *cloudcommon.LifecycleSpec `json:"lifecycle,omitempty"`
}

// FromProto returns a ProjectSpec constructed from the supplied Temporal Cloud
// ProjectSpec.
func (s ProjectSpec) FromProto(in *tcloudprojectv1.ProjectSpec) ProjectSpec {
	if in == nil {
		return ProjectSpec{}
	}
	return ProjectSpec{
		DisplayName: in.GetDisplayName(),
		Description: in.GetDescription(),
		Lifecycle:   ProjectLifecycleFromProto(in.GetLifecycle()),
	}
}

// toProto returns the Temporal Cloud ProjectSpec that represents the spec.
// Use [Project.ToProto].
func (s ProjectSpec) toProto() *tcloudprojectv1.ProjectSpec {
	return &tcloudprojectv1.ProjectSpec{
		DisplayName: s.DisplayName,
		Description: s.Description,
		Lifecycle:   ProjectLifecycleToProto(s.Lifecycle),
	}
}

// ProjectLifecycleFromProto converts the supplied Temporal Cloud project
// LifecycleSpec to its CR form, returning nil when it is unset.
//
// It is a function rather than a FromProto method for the same reason as
// [NamespaceLifecycleFromProto]: [cloudcommon.LifecycleSpec] is shared across
// resources, and Temporal Cloud has a separate LifecycleSpec proto per
// resource.
func ProjectLifecycleFromProto(
	in *tcloudprojectv1.LifecycleSpec,
) *cloudcommon.LifecycleSpec {
	if in == nil {
		return nil
	}
	return &cloudcommon.LifecycleSpec{
		EnableDeleteProtection: in.GetEnableDeleteProtection(),
	}
}

// ProjectLifecycleToProto converts the supplied lifecycle settings to a
// Temporal Cloud project LifecycleSpec, returning nil when in is nil.
//
// It is a function rather than a ToProto method for the same reason as
// [NamespaceLifecycleToProto].
func ProjectLifecycleToProto(
	in *cloudcommon.LifecycleSpec,
) *tcloudprojectv1.LifecycleSpec {
	if in == nil {
		return nil
	}
	return &tcloudprojectv1.LifecycleSpec{
		EnableDeleteProtection: in.EnableDeleteProtection,
	}
}

// ProjectStatus describes the current status of a Temporal Cloud project.
// This struct does not contain a FromProto or ToProto function because there is
// no corresponding ProjectStatus struct in the Temporal Cloud API.
// +k8s:openapi-gen=true
type ProjectStatus struct {
	tmprlcorev1.StatusBase `json:",inline"`
	// Resource contains common information about the Temporal Cloud resource
	// that this CR represents.
	Resource cloudcommon.ResourceStatus `json:"resource"`
}

// ProjectList is a list of Project resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Project `json:"items"`
}
