package v1alpha1

import (
	"maps"
	"slices"

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
	// +required
	Spec ServiceAccountSpec `json:"spec"`
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
// Namespace ref replaced by its ID from `refs`.
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

	out := &tcloudidentityv1.ServiceAccountSpec{
		Name:        s.DisplayName,
		Description: s.Description,
	}
	switch {
	case s.AccountScopedAccess != nil:
		out.Access = s.AccountScopedAccess.toProto(namespaceID)
	case s.NamespaceScopedAccess != nil:
		out.NamespaceScopedAccess = &tcloudidentityv1.NamespaceScopedAccess{
			Namespace: namespaceID(s.NamespaceScopedAccess.Namespace),
			Access:    s.NamespaceScopedAccess.Access.ToProto(),
		}
	case s.ProjectScopedAccess != nil:
		out.ProjectScopedAccess = &tcloudidentityv1.ProjectScopedAccess{
			ProjectId: s.ProjectScopedAccess.Project.ID,
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

// AccountScopedAccess describes the access assigned to an account scoped Service Account.
// Maps to Temporal Cloud proto `Access`.
type AccountScopedAccess struct {
	// AccountAccess is the access to the account.
	// Maps to Temporal Cloud proto `Access.account_access`.
	// +required
	AccountAccess *AccountAccess `json:"accountAccess,omitempty"`
	// NamespaceAccesses are the accesses to individual Namespaces.
	// Maps to Temporal Cloud proto `Access.namespace_accesses`.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=1000
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b.__namespace__ == a.__namespace__))",message="each namespace can only appear once"
	NamespaceAccesses []NamespaceScopedAccess `json:"namespaceAccesses,omitempty"`
	// ProjectAccesses are the accesses to individual Projects.
	// Maps to Temporal Cloud proto `Access.project_accesses`.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=1000
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b.project == a.project))",message="each project can only appear once"
	ProjectAccesses []ProjectAccessEntry `json:"projectAccesses,omitempty"`
}

// accountScopedAccessFromProto returns the account scoped access for the
// supplied Temporal Cloud Access.
func accountScopedAccessFromProto(in *tcloudidentityv1.Access) *AccountScopedAccess {
	out := &AccountScopedAccess{
		NamespaceAccesses: namespaceAccessesFromProto(in.GetNamespaceAccesses()),
		ProjectAccesses:   projectAccessesFromProto(in.GetProjectAccesses()),
	}
	if aa := in.GetAccountAccess(); aa != nil {
		var role AccountRole
		role.FromProto(aa.GetRole())
		var customRoles []cloudcommon.ResourceReferenceOrID
		ids := slices.Clone(aa.GetCustomRoles())
		slices.Sort(ids)
		for _, id := range ids {
			customRoles = append(customRoles, cloudcommon.ResourceReferenceOrID{ID: id})
		}
		if role != "" || len(customRoles) > 0 {
			out.AccountAccess = &AccountAccess{Role: role, CustomRoles: customRoles}
		}
	}
	return out
}

// toProto returns the Temporal Cloud Access for the account scoped access,
// with namespaceID giving the ID of each Namespace.
func (a AccountScopedAccess) toProto(
	namespaceID func(cloudcommon.ResourceReferenceOrID) string,
) *tcloudidentityv1.Access {
	out := &tcloudidentityv1.Access{
		NamespaceAccesses: namespaceAccessesToProto(a.NamespaceAccesses, namespaceID),
		ProjectAccesses:   projectAccessesToProto(a.ProjectAccesses),
	}
	if a.AccountAccess != nil {
		out.AccountAccess = a.AccountAccess.toProto()
	}
	return out
}

// AccountAccess describes the access to the account.
type AccountAccess struct {
	// Role is the role on the account.
	// Maps to Temporal Cloud proto `AccountAccess.role`.
	// +required
	Role AccountRole `json:"role,omitempty"`
	// CustomRoles are the custom roles assigned.
	// Maps to Temporal Cloud proto `AccountAccess.custom_roles`.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=1000
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b == a))",message="each custom role can only appear once"
	CustomRoles []cloudcommon.ResourceReferenceOrID `json:"customRoles,omitempty"`
}

// toProto returns the Temporal Cloud AccountAccess for the account access.
func (a AccountAccess) toProto() *tcloudidentityv1.AccountAccess {
	var ids []string
	for _, customRole := range a.CustomRoles {
		ids = append(ids, customRole.ID)
	}
	slices.Sort(ids)

	role := a.Role.ToProto()
	if role == tcloudidentityv1.AccountAccess_ROLE_UNSPECIFIED && len(ids) == 0 {
		return nil
	}
	return &tcloudidentityv1.AccountAccess{
		Role:        role,
		CustomRoles: ids,
	}
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

// FromProto sets the role from the supplied Temporal Cloud account role.
func (r *AccountRole) FromProto(in tcloudidentityv1.AccountAccess_Role) {
	switch in {
	case tcloudidentityv1.AccountAccess_ROLE_OWNER:
		*r = AccountRoleOwner
	case tcloudidentityv1.AccountAccess_ROLE_ADMIN:
		*r = AccountRoleAdmin
	case tcloudidentityv1.AccountAccess_ROLE_DEVELOPER:
		*r = AccountRoleDeveloper
	case tcloudidentityv1.AccountAccess_ROLE_FINANCE_ADMIN:
		*r = AccountRoleFinanceAdmin
	case tcloudidentityv1.AccountAccess_ROLE_READ:
		*r = AccountRoleRead
	case tcloudidentityv1.AccountAccess_ROLE_METRICS_READ:
		*r = AccountRoleMetricsRead
	default:
		*r = ""
	}
}

// ToProto returns the Temporal Cloud account role for the role.
func (r AccountRole) ToProto() tcloudidentityv1.AccountAccess_Role {
	switch r {
	case AccountRoleOwner:
		return tcloudidentityv1.AccountAccess_ROLE_OWNER
	case AccountRoleAdmin:
		return tcloudidentityv1.AccountAccess_ROLE_ADMIN
	case AccountRoleDeveloper:
		return tcloudidentityv1.AccountAccess_ROLE_DEVELOPER
	case AccountRoleFinanceAdmin:
		return tcloudidentityv1.AccountAccess_ROLE_FINANCE_ADMIN
	case AccountRoleRead:
		return tcloudidentityv1.AccountAccess_ROLE_READ
	case AccountRoleMetricsRead:
		return tcloudidentityv1.AccountAccess_ROLE_METRICS_READ
	default:
		return tcloudidentityv1.AccountAccess_ROLE_UNSPECIFIED
	}
}

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

// namespaceAccessesFromProto returns the entries for the supplied Temporal
// Cloud namespace access map.
func namespaceAccessesFromProto(
	in map[string]*tcloudidentityv1.NamespaceAccess,
) []NamespaceScopedAccess {
	var out []NamespaceScopedAccess
	for _, id := range slices.Sorted(maps.Keys(in)) {
		var p NamespacePermission
		p.FromProto(in[id].GetPermission())
		if p == "" {
			continue
		}
		out = append(out, NamespaceScopedAccess{
			Namespace: cloudcommon.ResourceReferenceOrID{ID: id},
			Access:    NamespaceAccess{Permission: p},
		})
	}
	return out
}

// namespaceAccessesToProto returns the Temporal Cloud namespace access map for
// the supplied entries, with namespaceID giving the ID of each Namespace.
func namespaceAccessesToProto(
	entries []NamespaceScopedAccess,
	namespaceID func(cloudcommon.ResourceReferenceOrID) string,
) map[string]*tcloudidentityv1.NamespaceAccess {
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]*tcloudidentityv1.NamespaceAccess, len(entries))
	for _, e := range entries {
		out[namespaceID(e.Namespace)] = e.Access.ToProto()
	}
	return out
}

// NamespaceAccess describes the access to a Namespace.
// Maps to Temporal Cloud proto `NamespaceAccess`.
type NamespaceAccess struct {
	// Permission is the permission on the Namespace.
	// Maps to Temporal Cloud proto `NamespaceAccess.permission`.
	// +required
	Permission NamespacePermission `json:"permission"`
}

// ToProto returns the Temporal Cloud NamespaceAccess for the namespace access.
func (a NamespaceAccess) ToProto() *tcloudidentityv1.NamespaceAccess {
	return &tcloudidentityv1.NamespaceAccess{Permission: a.Permission.ToProto()}
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

// FromProto sets the permission from the supplied Temporal Cloud namespace
// permission.
func (p *NamespacePermission) FromProto(in tcloudidentityv1.NamespaceAccess_Permission) {
	switch in {
	case tcloudidentityv1.NamespaceAccess_PERMISSION_ADMIN:
		*p = NamespacePermissionAdmin
	case tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE:
		*p = NamespacePermissionWrite
	case tcloudidentityv1.NamespaceAccess_PERMISSION_READ:
		*p = NamespacePermissionRead
	default:
		*p = ""
	}
}

// ToProto returns the Temporal Cloud namespace permission for the permission.
func (p NamespacePermission) ToProto() tcloudidentityv1.NamespaceAccess_Permission {
	switch p {
	case NamespacePermissionAdmin:
		return tcloudidentityv1.NamespaceAccess_PERMISSION_ADMIN
	case NamespacePermissionWrite:
		return tcloudidentityv1.NamespaceAccess_PERMISSION_WRITE
	case NamespacePermissionRead:
		return tcloudidentityv1.NamespaceAccess_PERMISSION_READ
	default:
		return tcloudidentityv1.NamespaceAccess_PERMISSION_UNSPECIFIED
	}
}

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

// projectAccessesFromProto returns the entries for the supplied Temporal Cloud
// project access map.
func projectAccessesFromProto(
	in map[string]*tcloudidentityv1.ProjectAccess,
) []ProjectAccessEntry {
	var out []ProjectAccessEntry
	for _, id := range slices.Sorted(maps.Keys(in)) {
		var r ProjectRole
		r.FromProto(in[id].GetRole())
		if r == "" {
			continue
		}
		out = append(out, ProjectAccessEntry{
			Project: cloudcommon.ResourceReferenceOrID{ID: id},
			Access:  ProjectAccess{Role: r},
		})
	}
	return out
}

// projectAccessesToProto returns the Temporal Cloud project access map for the
// supplied entries.
func projectAccessesToProto(
	entries []ProjectAccessEntry,
) map[string]*tcloudidentityv1.ProjectAccess {
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]*tcloudidentityv1.ProjectAccess, len(entries))
	for _, e := range entries {
		out[e.Project.ID] = &tcloudidentityv1.ProjectAccess{
			Role: e.Access.Role.ToProto(),
		}
	}
	return out
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

// FromProto sets the role from the supplied Temporal Cloud project role.
func (r *ProjectRole) FromProto(in tcloudidentityv1.ProjectAccess_ProjectRole) {
	switch in {
	case tcloudidentityv1.ProjectAccess_PROJECT_ROLE_ADMIN:
		*r = ProjectRoleAdmin
	case tcloudidentityv1.ProjectAccess_PROJECT_ROLE_WRITE:
		*r = ProjectRoleWrite
	case tcloudidentityv1.ProjectAccess_PROJECT_ROLE_READ:
		*r = ProjectRoleRead
	case tcloudidentityv1.ProjectAccess_PROJECT_ROLE_LIST:
		*r = ProjectRoleList
	case tcloudidentityv1.ProjectAccess_PROJECT_ROLE_CONTRIBUTE:
		*r = ProjectRoleContribute
	case tcloudidentityv1.ProjectAccess_PROJECT_ROLE_MEMBER:
		*r = ProjectRoleMember
	default:
		*r = ""
	}
}

// ToProto returns the Temporal Cloud project role for the role.
func (r ProjectRole) ToProto() tcloudidentityv1.ProjectAccess_ProjectRole {
	switch r {
	case ProjectRoleAdmin:
		return tcloudidentityv1.ProjectAccess_PROJECT_ROLE_ADMIN
	case ProjectRoleWrite:
		return tcloudidentityv1.ProjectAccess_PROJECT_ROLE_WRITE
	case ProjectRoleRead:
		return tcloudidentityv1.ProjectAccess_PROJECT_ROLE_READ
	case ProjectRoleList:
		return tcloudidentityv1.ProjectAccess_PROJECT_ROLE_LIST
	case ProjectRoleContribute:
		return tcloudidentityv1.ProjectAccess_PROJECT_ROLE_CONTRIBUTE
	case ProjectRoleMember:
		return tcloudidentityv1.ProjectAccess_PROJECT_ROLE_MEMBER
	default:
		return tcloudidentityv1.ProjectAccess_PROJECT_ROLE_UNSPECIFIED
	}
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
	Namespaces []NamespaceResolvedReference `json:"namespaces,omitempty"`
}

// NamespaceID returns the Temporal Cloud ID of the Namespace CR that the
// supplied reference points to, and whether it has been resolved.
func (r ServiceAccountReferences) NamespaceID(
	ref cloudcommon.ResourceReference,
	referrerNamespace string,
) (string, bool) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = referrerNamespace
	}
	for _, resolved := range r.Namespaces {
		if resolved.Namespace == namespace && resolved.Name == ref.Name {
			return resolved.ID, true
		}
	}
	return "", false
}

// NamespaceResolvedReference contains a Kubernetes Namespace and Name for a
// Temporal Cloud Namespace CR along with its resolved Temporal Cloud resource
// identifier.
type NamespaceResolvedReference struct {
	// Namespace is the Kubernetes Namespace containing the Temporal Cloud
	// Namespace CR.
	// +required
	Namespace string `json:"namespace"`
	// Name is the Kubernetes Name of the Temporal Cloud Namespace CR.
	// +required
	Name string `json:"name"`
	// ID is the Temporal Cloud resource identifier.
	// +required
	ID string `json:"id"`
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
