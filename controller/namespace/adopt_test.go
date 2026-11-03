package namespace

import (
	"testing"

	"github.com/stretchr/testify/require"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

func TestAdoptionPolicy(t *testing.T) {
	const allowed = `. allowed values: adopt, adopt-or-create`
	cases := []struct {
		name        string
		annotations map[string]string
		exp         tcloudcommon.AdoptionPolicy
		expErr      string
	}{
		{
			name: "unset",
		},
		{
			name: "only other annotations",
			annotations: map[string]string{
				tcloudcommon.AnnotationKeyDeletionPolicy: "retain",
			},
		},
		{
			name:        "adopt",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: "adopt"},
			exp:         tcloudcommon.AdoptionPolicyAdopt,
		},
		{
			name:        "adopt-or-create",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: "adopt-or-create"},
			exp:         tcloudcommon.AdoptionPolicyAdoptOrCreate,
		},
		{
			name:        "mixed case is lowercased",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: "Adopt-Or-Create"},
			exp:         tcloudcommon.AdoptionPolicyAdoptOrCreate,
		},
		{
			name:        "whitespace is trimmed",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: " adopt "},
			exp:         tcloudcommon.AdoptionPolicyAdopt,
		},
		{
			name:        "empty value is invalid",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: ""},
			expErr:      `invalid adoption policy ""` + allowed,
		},
		{
			name:        "invalid value",
			annotations: map[string]string{tcloudcommon.AnnotationKeyAdoptionPolicy: " Import "},
			exp:         "import",
			expErr:      `invalid adoption policy "import"` + allowed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := &tcloudv1alpha1.Namespace{}
			cr.Annotations = tc.annotations

			got, err := adoptionPolicy(&res{cr})
			if tc.expErr != "" {
				require.EqualError(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.exp, got)
		})
	}
}
