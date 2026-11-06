package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmprlcorev1 "github.com/temporalio/kube-temporal/api/core/v1"

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
	// +required
	Spec ServiceAccountSpec `json:"spec"`
	// +optional
	// Status describes the current status of a Service Account.
	Status ServiceAccountStatus `json:"status,omitempty"`
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
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b.__namespace__ == a.__namespace__))",message="each namespace can only appear once"
	NamespaceAccesses []NamespaceScopedAccess `json:"namespaceAccesses,omitempty"`
}

// AccountScopedAccess describes the access assigned to an account scoped Service Account.
// Maps to Temporal Cloud proto `Access`.
type AccountScopedAccess struct {
	// AccountAccess is the access to the account.
	// Maps to Temporal Cloud proto `Access.account_access`.
	// +optional
	AccountAccess *AccountAccess `json:"accountAccess,omitempty"`
	// NamespaceAccesses are the accesses to individual Namespaces.
	// Maps to Temporal Cloud proto `Access.namespace_accesses`.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b.__namespace__ == a.__namespace__))",message="each namespace can only appear once"
	NamespaceAccesses []NamespaceScopedAccess `json:"namespaceAccesses,omitempty"`
	// ProjectAccesses are the accesses to individual Projects.
	// Maps to Temporal Cloud proto `Access.project_accesses`.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b.project == a.project))",message="each project can only appear once"
	ProjectAccesses []ProjectAccessEntry `json:"projectAccesses,omitempty"`
}

// AccountAccess describes the access to the account.
type AccountAccess struct {
	// Role is the role on the account.
	// Maps to Temporal Cloud proto `AccountAccess.role`.
	// +optional
	Role AccountRole `json:"role,omitempty"`
	// CustomRoles are the custom roles assigned.
	// Maps to Temporal Cloud proto `AccountAccess.custom_roles`.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b == a))",message="each custom role can only appear once"
	CustomRoles []cloudcommon.ResourceReferenceOrID `json:"customRoles,omitempty"`
}

// AccountRole is a role on the account.
// Maps to Temporal Cloud proto `AccountAccess.Role`.
// +kubebuilder:validation:Enum=Owner;Admin;Developer;FinanceAdmin;Read;MetricsRead
type AccountRole string

const (
	// AccountRoleOwner gives full access to the account, including users,
	// Namespaces and billing.
	AccountRoleOwner AccountRole = "Owner"
	// AccountRoleAdmin gives full access to the account, including users and
	// Namespaces.
	AccountRoleAdmin AccountRole = "Admin"
	// AccountRoleDeveloper gives access to create Namespaces on the account.
	AccountRoleDeveloper AccountRole = "Developer"
	// AccountRoleFinanceAdmin gives read only access and write access for
	// billing.
	AccountRoleFinanceAdmin AccountRole = "FinanceAdmin"
	// AccountRoleRead gives read only access to the account.
	AccountRoleRead AccountRole = "Read"
	// AccountRoleMetricsRead gives read only access to the account metrics.
	AccountRoleMetricsRead AccountRole = "MetricsRead"
)

// NamespaceScopedAccess describes the access to a single Namespace.
// Maps to Temporal Cloud proto `NamespaceScopedAccess`.
type NamespaceScopedAccess struct {
	// Namespace is the Namespace the access applies to.
	// Maps to Temporal Cloud proto `NamespaceScopedAccess.namespace`.
	// +required
	Namespace cloudcommon.ResourceReferenceOrID `json:"namespace"`
	// Access is the access to the Namespace.
	// Maps to Temporal Cloud proto `NamespaceScopedAccess.access`.
	// +required
	Access NamespaceAccess `json:"access"`
}

// NamespaceAccess describes the access to a Namespace.
// Maps to Temporal Cloud proto `NamespaceAccess`.
type NamespaceAccess struct {
	// Permission is the permission on the Namespace.
	// Maps to Temporal Cloud proto `NamespaceAccess.permission`.
	// +required
	Permission NamespacePermission `json:"permission"`
}

// NamespacePermission is a permission on a Namespace.
// Maps to Temporal Cloud proto `NamespaceAccess.Permission`.
// +kubebuilder:validation:Enum=Admin;Write;Read
type NamespacePermission string

const (
	// NamespacePermissionAdmin gives full access to the Namespace, including
	// assigning Namespace access to others.
	NamespacePermissionAdmin NamespacePermission = "Admin"
	// NamespacePermissionWrite gives write access to the Namespace
	// configuration and workflows within the Namespace.
	NamespacePermissionWrite NamespacePermission = "Write"
	// NamespacePermissionRead gives read only access to the Namespace
	// configuration and workflows within the Namespace.
	NamespacePermissionRead NamespacePermission = "Read"
)

// ProjectAccessEntry describes the access to a single Project.
// Maps to an entry of Temporal Cloud proto `Access.project_accesses`.
type ProjectAccessEntry struct {
	// Project is the Project the access applies to.
	// Maps to the key of Temporal Cloud proto `Access.project_accesses`.
	// +required
	Project cloudcommon.ResourceReferenceOrID `json:"project"`
	// Access is the access to the Project.
	// Maps to the value of Temporal Cloud proto `Access.project_accesses`.
	// +required
	Access ProjectAccess `json:"access"`
}

// ProjectAccess describes the access to a Project.
// Maps to Temporal Cloud proto `ProjectAccess`.
type ProjectAccess struct {
	// Role is the role on the Project.
	// Maps to Temporal Cloud proto `ProjectAccess.role`.
	// +required
	Role ProjectRole `json:"role"`
}

// ProjectRole is a role on a Project.
// Maps to Temporal Cloud proto `ProjectAccess.ProjectRole`.
// +kubebuilder:validation:Enum=Admin;Write;Read;List;Contribute;Member
type ProjectRole string

const (
	// ProjectRoleAdmin gives full access to the Project, including managing
	// Project membership.
	ProjectRoleAdmin ProjectRole = "Admin"
	// ProjectRoleWrite gives write access to Project resources.
	ProjectRoleWrite ProjectRole = "Write"
	// ProjectRoleRead gives read only access to Project resources.
	ProjectRoleRead ProjectRole = "Read"
	// ProjectRoleList gives access to list resources within the Project.
	ProjectRoleList ProjectRole = "List"
	// ProjectRoleContribute gives access to contribute to workflows within
	// the Project.
	ProjectRoleContribute ProjectRole = "Contribute"
	// ProjectRoleMember gives basic membership, which grants read only access
	// to the Project.
	ProjectRoleMember ProjectRole = "Member"
)

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

// ServiceAccountReferences contains the Temporal Cloud resource IDs of related
// resources, at the same paths as the spec fields that refer to them.
type ServiceAccountReferences struct {
	// AccountScopedAccess contains the IDs referenced by spec.accountScopedAccess.
	// +optional
	AccountScopedAccess *AccountScopedAccessReferences `json:"accountScopedAccess,omitempty"`
	// NamespaceScopedAccess contains the IDs referenced by
	// spec.namespaceScopedAccess.
	// +optional
	NamespaceScopedAccess *NamespaceScopedAccessReferences `json:"namespaceScopedAccess,omitempty"`
	// ProjectScopedAccess contains the IDs referenced by
	// spec.projectScopedAccess.
	// +optional
	ProjectScopedAccess *ProjectScopedAccessReferences `json:"projectScopedAccess,omitempty"`
}

// AccountScopedAccessReferences contains the IDs referenced by spec.accountScopedAccess.
type AccountScopedAccessReferences struct {
	// AccountAccess contains the IDs referenced by spec.accountScopedAccess.accountAccess.
	// +optional
	AccountAccess *AccountAccessReferences `json:"accountAccess,omitempty"`
	// NamespaceAccesses are the Namespace IDs of
	// spec.accountScopedAccess.namespaceAccesses, in the same order.
	// +optional
	// +listType=atomic
	NamespaceAccesses []string `json:"namespaceAccesses,omitempty"`
	// ProjectAccesses are the Project IDs of spec.accountScopedAccess.projectAccesses, in
	// the same order.
	// +optional
	// +listType=atomic
	ProjectAccesses []string `json:"projectAccesses,omitempty"`
}

// AccountAccessReferences contains the IDs referenced by
// spec.accountScopedAccess.accountAccess.
type AccountAccessReferences struct {
	// CustomRoles are the custom role IDs of
	// spec.accountScopedAccess.accountAccess.customRoles, in the same order.
	// +optional
	// +listType=atomic
	CustomRoles []string `json:"customRoles,omitempty"`
}

// NamespaceScopedAccessReferences contains the IDs referenced by
// spec.namespaceScopedAccess.
type NamespaceScopedAccessReferences struct {
	// Namespace is the Namespace ID of spec.namespaceScopedAccess.namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ProjectScopedAccessReferences contains the IDs referenced by
// spec.projectScopedAccess.
type ProjectScopedAccessReferences struct {
	// Project is the Project ID of spec.projectScopedAccess.project.
	// +optional
	Project string `json:"project,omitempty"`
	// NamespaceAccesses are the Namespace IDs of
	// spec.projectScopedAccess.namespaceAccesses, in the same order.
	// +optional
	// +listType=atomic
	NamespaceAccesses []string `json:"namespaceAccesses,omitempty"`
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
