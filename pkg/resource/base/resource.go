package base

import (
	"fmt"
	"strings"
	"time"

	"github.com/temporalio/kube-temporal/pkg/kube/annotation"
	ktresourcebase "github.com/temporalio/kube-temporal/pkg/resource/base"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
)

// Resource is the base class for any Temporal Cloud Operator-managed resource.
//
// Implements pkg/types.Resource.
type Resource struct {
	ktresourcebase.Resource
	defaultDeletionPolicy tcloudcommon.DeletionPolicy
	defaultAdoptionPolicy tcloudcommon.AdoptionPolicy
}

// DeletionPolicy returns the deletion policy for the resource from its
// deletion-policy annotation.
func (r Resource) DeletionPolicy() tcloudcommon.DeletionPolicy {
	dp, ok := annotation.Value(r, tcloudcommon.AnnotationKeyDeletionPolicy)
	if !ok {
		if r.IsAdopted() {
			return tcloudcommon.DeletionPolicyRetain
		}
		return r.defaultDeletionPolicy
	}
	dp = strings.ToLower(strings.TrimSpace(dp))
	return tcloudcommon.DeletionPolicy(dp)
}

// SetDeletionPolicy sets the resource's deletion policy annotation.
func (r *Resource) SetDeletionPolicy(p tcloudcommon.DeletionPolicy) {
	r.setAnnotation(tcloudcommon.AnnotationKeyDeletionPolicy, string(p))
}

// IsAdopted returns true if the resource was adopted by TCO.
func (r Resource) IsAdopted() bool {
	_, found := annotation.Value(r, tcloudcommon.AnnotationKeyAdoptedOn)
	return found
}

// AdoptedOn returns the time that TCO adopted the resource. Returns nil when
// the resource has not been adopted.
//
// Returns an error if the resource adopted-on timestamp was found but was not
// a valid RFC 3339 UTC timestamp string.
func (r Resource) AdoptedOn() (*time.Time, error) {
	annoVal, found := annotation.Value(r, tcloudcommon.AnnotationKeyAdoptedOn)
	if !found {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(annoVal))
	if err != nil {
		return nil, fmt.Errorf(
			"failed parsing adopted-on timestamp %q: %w",
			annoVal, err,
		)
	}
	return &t, nil
}

// SetAdoptedOn sets the resource's adopted on timestamp.
func (r *Resource) SetAdoptedOn(t time.Time) {
	ts := t.UTC().Format(time.RFC3339)
	r.setAnnotation(tcloudcommon.AnnotationKeyAdoptedOn, ts)
}

// AdoptionPolicy returns the adoption policy on the resource. If no adoption
// policy annotation was found on the resource, returns the default adoption
// policy for that kind of resource.
func (r Resource) AdoptionPolicy() tcloudcommon.AdoptionPolicy {
	ap, found := annotation.Value(r, tcloudcommon.AnnotationKeyAdoptionPolicy)
	if !found {
		return r.defaultAdoptionPolicy
	}
	ap = strings.ToLower(strings.TrimSpace(ap))
	return tcloudcommon.AdoptionPolicy(ap)
}

// SetAdoptionPolicy sets the resource's adoption policy annotation.
func (r *Resource) SetAdoptionPolicy(p tcloudcommon.AdoptionPolicy) {
	r.setAnnotation(tcloudcommon.AnnotationKeyAdoptionPolicy, string(p))
}

// setAnnotation sets the supplied annotation, creating the resource's
// annotations map first if it has none. The embedded SetAnnotation writes to
// the existing map and panics when it is nil.
func (r *Resource) setAnnotation(key, value string) {
	if r.Annotations() == nil {
		r.ReplaceAnnotations(annotation.Annotations{})
	}
	r.SetAnnotation(key, value)
}

// PreviouslyReconciled returns true if the Resource has previously been
// successfully reconciled and had the Status.Resource.ID field set to a
// non-empty value.
func (r Resource) PreviouslyReconciled() bool {
	return r.statusResourceID() != ""
}

// ResourceID returns the Temporal Cloud resource identifier for the
// Resource. If the Resource has been previously reconciled, returns
// Status.Resource.ID, otherwise returns the value of the
// `cloud.temporal.io/resource-id` annotation, if any.
func (r Resource) ResourceID() string {
	if id := r.statusResourceID(); id != "" {
		return id
	}
	id, _ := annotation.Value(r, tcloudcommon.AnnotationKeyResourceID)
	return strings.TrimSpace(id)
}

// statusResourceID returns the resource's Status.Resource.ID
func (r Resource) statusResourceID() string {
	// We ignore errors from Unstructured() because we know the Resource has
	// been properly constructed with a thing that implements client.Object and
	// runtime.Object.
	us, _ := r.Unstructured()
	if us == nil {
		return ""
	}
	id, ok, _ := unstructured.NestedString(
		us.Object, "status", "resource", "id",
	)
	if ok {
		return id
	}
	return ""
}

// ResourceStatus returns the common ResourceStatus struct associated with
// the Resource.
func (r Resource) ResourceStatus() tcloudcommon.ResourceStatus {
	// We ignore errors from Unstructured() because we know the Resource has
	// been properly constructed with a thing that implements client.Object and
	// runtime.Object.
	rs := tcloudcommon.ResourceStatus{}
	us, _ := r.Unstructured()
	if us == nil {
		return rs
	}
	raw, ok, _ := unstructured.NestedMap(
		us.Object, "status", "resource",
	)
	if !ok {
		return rs
	}
	out := rs.FromMap(raw)
	return out
}

// ResourceState returns the common ResourceStatus.State field.
func (r Resource) ResourceState() tcloudcommon.ResourceState {
	rs := r.ResourceStatus()
	return rs.State
}

// NewOption modifies the Resource returned from New
type NewOption func(*Resource)

// WithDefaultAdoptionPolicy sets the Resource's default adoption policy.
func WithDefaultAdoptionPolicy(p tcloudcommon.AdoptionPolicy) NewOption {
	return func(r *Resource) {
		r.defaultAdoptionPolicy = p
	}
}

// WithDefaultDeletionPolicy sets the Resource's default deletion policy.
func WithDefaultDeletionPolicy(p tcloudcommon.DeletionPolicy) NewOption {
	return func(r *Resource) {
		r.defaultDeletionPolicy = p
	}
}

// New returns a new base Resource from the supplied GroupVersionKind and
// concrete custom resource.  The supplied custom resource must satisfy both
// apimachinery's runtime.Object interface as well as controller-runtime's
// client.Object interface. If it does not, returns an error.
func New(
	gvk schema.GroupVersionKind,
	cr any,
	opts ...NewOption,
) (*Resource, error) {
	b, err := ktresourcebase.New(gvk, cr)
	if err != nil {
		return nil, err
	}
	r := Resource{
		Resource: *b,
	}
	for _, opt := range opts {
		opt(&r)
	}
	if r.defaultDeletionPolicy == "" {
		r.defaultDeletionPolicy = tcloudcommon.DeletionPolicyDelete
	}
	return &r, nil
}
