package v1alpha1

import (
	tcloudidentityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmprlcorev1 "github.com/temporalio/kube-temporal/api/core/v1"
	"github.com/temporalio/kube-temporal/pkg/kube/transform"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// ServiceAccount represents an instance of a Temporal Cloud Service Account.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.resource.id`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.resource.state`
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ServiceAccount struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Service Account.
	// +optional
	Spec ServiceAccountSpec `json:"spec,omitzero"`
	// +optional
	// Status describes the current status of a Service Account.
	Status ServiceAccountStatus `json:"status,omitempty"`
}

// FromProto returns a ServiceAccount from the supplied Temporal Cloud
// ServiceAccount proto message.
func (sa ServiceAccount) FromProto(in *tcloudidentityv1.ServiceAccount) *ServiceAccount {
	out := ServiceAccount{
		TypeMeta:   sa.TypeMeta,
		ObjectMeta: sa.ObjectMeta,
	}
	out.Status.References = *sa.Status.References.DeepCopy()

	if in == nil {
		return &out
	}

	var spec ServiceAccountSpec
	out.Spec = spec.FromProto(in.GetSpec())

	out.Status.Resource.ID = in.GetId()
	out.Status.Resource.Version = in.GetResourceVersion()
	out.Status.Resource.State.FromProto(in.GetState())
	out.Status.Resource.AsyncOperationID = in.GetAsyncOperationId()
	out.Status.Resource.CreatedTime = transform.TimeFromProto(in.GetCreatedTime())
	out.Status.Resource.LastModifiedTime = transform.TimeFromProto(in.GetLastModifiedTime())
	return &out
}

// ServiceAccountSpec describes the desired state of a Temporal Cloud Service Account.
//
// Ref: https://github.com/temporalio/cloud-api/blob/v0.24.0/temporal/api/cloud/identity/v1/message.proto
// +k8s:openapi-gen=true
// +kubebuilder:validation:XValidation:rule="[has(self.accountScopedAccess), has(self.namespaceScopedAccess), has(self.projectScopedAccess)].filter(x, x).size() == 1",message="exactly one of accountScopedAccess, namespaceScopedAccess or projectScopedAccess must be set"
// +kubebuilder:validation:XValidation:rule="has(self.accountScopedAccess) == has(oldSelf.accountScopedAccess) && has(self.namespaceScopedAccess) == has(oldSelf.namespaceScopedAccess) && has(self.projectScopedAccess) == has(oldSelf.projectScopedAccess)",message="the access scope cannot be changed after creation"
// +kubebuilder:validation:XValidation:rule="!has(self.namespaceScopedAccess) || !has(oldSelf.namespaceScopedAccess) || self.namespaceScopedAccess.__namespace__ == oldSelf.namespaceScopedAccess.__namespace__",message="namespaceScopedAccess.namespace cannot be changed after creation",fieldPath=".namespaceScopedAccess.namespace"
// +kubebuilder:validation:XValidation:rule="!has(self.projectScopedAccess) || !has(oldSelf.projectScopedAccess) || self.projectScopedAccess.project == oldSelf.projectScopedAccess.project",message="projectScopedAccess.project cannot be changed after creation",fieldPath=".projectScopedAccess.project"
type ServiceAccountSpec struct {
	// DisplayName is the name of the Service Account in Temporal Cloud.
	// It is mutable and must be unique within the account.
	// Maps to Temporal Cloud proto `ServiceAccountSpec.name`.
	// +required
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`
	// Description is a human readable description of the Service Account.
	// Maps to Temporal Cloud proto `ServiceAccountSpec.description`.
	// +optional
	Description string `json:"description,omitempty"`
	// AccountScopedAccess makes this an account scoped Service Account.
	// Maps to Temporal Cloud proto `ServiceAccountSpec.access`.
	// +optional
	AccountScopedAccess *AccountScopedAccess `json:"accountScopedAccess,omitempty"`
	// NamespaceScopedAccess makes this a Service Account scoped to a single
	// Namespace.
	// Maps to Temporal Cloud proto `ServiceAccountSpec.namespace_scoped_access`.
	// +optional
	NamespaceScopedAccess *NamespaceScopedAccess `json:"namespaceScopedAccess,omitempty"`
	// ProjectScopedAccess makes this a Service Account scoped to a single
	// Project.
	// Maps to Temporal Cloud proto `ServiceAccountSpec.project_scoped_access`.
	// +optional
	ProjectScopedAccess *ProjectScopedAccess `json:"projectScopedAccess,omitempty"`
}

// FromProto returns a ServiceAccountSpec from the supplied Temporal Cloud
// ServiceAccountSpec proto message.
func (s ServiceAccountSpec) FromProto(in *tcloudidentityv1.ServiceAccountSpec) ServiceAccountSpec {
	out := ServiceAccountSpec{
		DisplayName: in.GetName(),
		Description: in.GetDescription(),
	}
	switch {
	case in.GetNamespaceScopedAccess() != nil:
		nsa := in.GetNamespaceScopedAccess()
		out.NamespaceScopedAccess = &NamespaceScopedAccess{
			Namespace: cloudcommon.ResourceReferenceOrID{ID: nsa.GetNamespace()},
		}
		out.NamespaceScopedAccess.Access.Permission.FromProto(nsa.GetAccess().GetPermission())
	case in.GetProjectScopedAccess() != nil:
		psa := in.GetProjectScopedAccess()
		out.ProjectScopedAccess = &ProjectScopedAccess{
			Project:           cloudcommon.ResourceReferenceOrID{ID: psa.GetProjectId()},
			NamespaceAccesses: namespaceAccessesFromProto(psa.GetNamespaceAccesses()),
		}
		out.ProjectScopedAccess.Access.Role.FromProto(psa.GetAccess().GetRole())
	default:
		// Temporal Cloud requires one of the three accesses, so a service
		// account with neither namespace nor project scoped access is
		// account scoped, even if Temporal Cloud omits its empty access.
		out.AccountScopedAccess = accountScopedAccessFromProto(in.GetAccess())
	}
	return out
}

// ToProto returns the Temporal Cloud ServiceAccountSpec for the spec, with each
// Namespace and Project ref replaced by its ID from `refs`.
func (s ServiceAccountSpec) ToProto(
	referrerNamespace string,
	refs ServiceAccountReferences,
) *tcloudidentityv1.ServiceAccountSpec {
	namespaceID := func(namespace cloudcommon.ResourceReferenceOrID) string {
		if namespace.Ref == nil {
			return namespace.ID
		}
		id, _ := refs.NamespaceID(*namespace.Ref, referrerNamespace)
		return id
	}
	projectID := func(project cloudcommon.ResourceReferenceOrID) string {
		if project.Ref == nil {
			return project.ID
		}
		id, _ := refs.ProjectID(*project.Ref, referrerNamespace)
		return id
	}

	out := &tcloudidentityv1.ServiceAccountSpec{
		Name:        s.DisplayName,
		Description: s.Description,
	}
	switch {
	case s.AccountScopedAccess != nil:
		out.Access = s.AccountScopedAccess.toProto(namespaceID, projectID)
	case s.NamespaceScopedAccess != nil:
		out.NamespaceScopedAccess = &tcloudidentityv1.NamespaceScopedAccess{
			Namespace: namespaceID(s.NamespaceScopedAccess.Namespace),
			Access:    s.NamespaceScopedAccess.Access.ToProto(),
		}
	case s.ProjectScopedAccess != nil:
		out.ProjectScopedAccess = &tcloudidentityv1.ProjectScopedAccess{
			ProjectId: projectID(s.ProjectScopedAccess.Project),
			Access: &tcloudidentityv1.ProjectAccess{
				Role: s.ProjectScopedAccess.Access.Role.ToProto(),
			},
			NamespaceAccesses: namespaceAccessesToProto(
				s.ProjectScopedAccess.NamespaceAccesses, namespaceID,
			),
		}
	}
	return out
}

// ProjectScopedAccess describes the access of a Service Account scoped to a
// single Project.
// Maps to Temporal Cloud proto `ProjectScopedAccess`.
type ProjectScopedAccess struct {
	// Project is the Project the Service Account is scoped to.
	// Maps to Temporal Cloud proto `ProjectScopedAccess.project_id`.
	// +required
	Project cloudcommon.ResourceReferenceOrID `json:"project"`
	// Access is the access to the Project.
	// Maps to Temporal Cloud proto `ProjectScopedAccess.access`.
	// +required
	Access ProjectAccess `json:"access"`
	// NamespaceAccesses are the accesses to individual Namespaces.
	// Maps to Temporal Cloud proto `ProjectScopedAccess.namespace_accesses`.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=1000
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b.__namespace__ == a.__namespace__))",message="each namespace can only appear once"
	NamespaceAccesses []NamespaceScopedAccess `json:"namespaceAccesses,omitempty"`
}

// ServiceAccountStatus describes the current status of a Temporal Cloud
// Service Account.
// +k8s:openapi-gen=true
type ServiceAccountStatus struct {
	tmprlcorev1.StatusBase `json:",inline"`
	// Resource contains common information about the Temporal Cloud resource
	// that this CR represents.
	// +optional
	Resource cloudcommon.ResourceStatus `json:"resource,omitempty"`
	// References contains the Temporal Cloud resource IDs of related resources.
	// +optional
	References ServiceAccountReferences `json:"references,omitempty"`
}

// ServiceAccountReferences contains the Temporal Cloud IDs of the custom
// resources referenced by the spec.
type ServiceAccountReferences struct {
	// Namespaces are the Namespace CRs referenced by the spec that have a
	// Temporal Cloud ID.
	// +optional
	// +listType=map
	// +listMapKey=namespace
	// +listMapKey=name
	Namespaces []cloudcommon.ResolvedReference `json:"namespaces,omitempty"`
	// Projects are the Project CRs referenced by the spec that have a
	// Temporal Cloud ID.
	// +optional
	// +listType=map
	// +listMapKey=namespace
	// +listMapKey=name
	Projects []cloudcommon.ResolvedReference `json:"projects,omitempty"`
}

// NamespaceID returns the Temporal Cloud ID of the Namespace CR that the
// supplied reference points to, and whether it has been resolved.
func (r ServiceAccountReferences) NamespaceID(
	ref cloudcommon.ResourceReference,
	referrerNamespace string,
) (string, bool) {
	return cloudcommon.ResolvedID(r.Namespaces, ref, referrerNamespace)
}

// ProjectID returns the Temporal Cloud ID of the Project CR that the supplied
// reference points to, and whether it has been resolved.
func (r ServiceAccountReferences) ProjectID(
	ref cloudcommon.ResourceReference,
	referrerNamespace string,
) (string, bool) {
	return cloudcommon.ResolvedID(r.Projects, ref, referrerNamespace)
}

// ServiceAccountList is a list of ServiceAccount resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ServiceAccount `json:"items"`
}
