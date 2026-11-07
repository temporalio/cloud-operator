package v1alpha1_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tcloudprojectv1 "go.temporal.io/cloud-sdk/api/project/v1"
	tcloudresourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	"github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

func fullProjectSpec() v1alpha1.ProjectSpec {
	return v1alpha1.ProjectSpec{
		DisplayName: "My Project",
		Description: "the full project",
		Lifecycle:   &cloudcommon.LifecycleSpec{EnableDeleteProtection: true},
	}
}

func fullProtoProjectSpec() *tcloudprojectv1.ProjectSpec {
	return &tcloudprojectv1.ProjectSpec{
		DisplayName: "My Project",
		Description: "the full project",
		Lifecycle:   &tcloudprojectv1.LifecycleSpec{EnableDeleteProtection: true},
	}
}

func projectWithSpec(spec v1alpha1.ProjectSpec) *v1alpha1.Project {
	p := &v1alpha1.Project{Spec: spec}
	p.Namespace = "my-ns"
	p.Name = "my-project"
	return p
}

func TestProjectFromProto_Spec(t *testing.T) {
	cases := []struct {
		name string
		in   *tcloudprojectv1.Project
		exp  v1alpha1.ProjectSpec
	}{
		{
			name: "nil proto",
		},
		{
			name: "nil proto spec",
			in:   &tcloudprojectv1.Project{Id: "proj-1"},
		},
		{
			name: "unset lifecycle stays nil",
			in: &tcloudprojectv1.Project{
				Spec: &tcloudprojectv1.ProjectSpec{DisplayName: "My Project"},
			},
			exp: v1alpha1.ProjectSpec{DisplayName: "My Project"},
		},
		{
			name: "every field",
			in:   &tcloudprojectv1.Project{Spec: fullProtoProjectSpec()},
			exp:  fullProjectSpec(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			p := projectWithSpec(v1alpha1.ProjectSpec{})

			got := p.FromProto(tc.in)

			require.Equal(tt, tc.exp, got.Spec)
		})
	}
}

func TestProjectFromProto_Status(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)
	modified := time.Date(2026, 2, 3, 4, 5, 6, 7, time.UTC)

	cases := []struct {
		name string
		in   *tcloudprojectv1.Project
		exp  cloudcommon.ResourceStatus
	}{
		{
			name: "nil project",
		},
		{
			name: "every field",
			in: &tcloudprojectv1.Project{
				Id:               "proj-1",
				ResourceVersion:  "rv-1",
				State:            tcloudresourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
				AsyncOperationId: "op-1",
				CreatedTime:      timestamppb.New(created),
				LastModifiedTime: timestamppb.New(modified),
			},
			exp: cloudcommon.ResourceStatus{
				ID:               "proj-1",
				Version:          "rv-1",
				State:            cloudcommon.ResourceStateActive,
				AsyncOperationID: "op-1",
				CreatedTime:      &metav1.Time{Time: created},
				LastModifiedTime: &metav1.Time{Time: modified},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			p := projectWithSpec(v1alpha1.ProjectSpec{})

			got := p.FromProto(tc.in)

			require.Equal(tc.exp, got.Status.Resource)
			// Object metadata is left unchanged.
			require.Equal("my-project", got.Name)
			require.Equal("my-ns", got.Namespace)
		})
	}
}

func TestProjectToProto(t *testing.T) {
	cases := []struct {
		name string
		in   *v1alpha1.Project
		exp  *tcloudprojectv1.Project
	}{
		{
			name: "unset lifecycle stays nil",
			in:   projectWithSpec(v1alpha1.ProjectSpec{DisplayName: "My Project"}),
			exp: &tcloudprojectv1.Project{
				Spec: &tcloudprojectv1.ProjectSpec{DisplayName: "My Project"},
			},
		},
		{
			name: "every field",
			in:   projectWithSpec(fullProjectSpec()),
			exp:  &tcloudprojectv1.Project{Spec: fullProtoProjectSpec()},
		},
		{
			// ToProto sets only the fields that are sent to Temporal Cloud, so
			// status is never copied onto the proto.
			name: "status is not sent",
			in: func() *v1alpha1.Project {
				p := projectWithSpec(v1alpha1.ProjectSpec{DisplayName: "My Project"})
				p.Status.Resource.ID = "proj-1"
				p.Status.Resource.Version = "rv-1"
				return p
			}(),
			exp: &tcloudprojectv1.Project{
				Spec: &tcloudprojectv1.ProjectSpec{DisplayName: "My Project"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			got := tc.in.ToProto()
			require.True(tt, proto.Equal(tc.exp, got), "expected %v, got %v", tc.exp, got)
		})
	}
}

func TestProjectLifecycleConverters_Nil(t *testing.T) {
	require.Nil(t, v1alpha1.ProjectLifecycleFromProto(nil))
	require.Nil(t, v1alpha1.ProjectLifecycleToProto(nil))
}
