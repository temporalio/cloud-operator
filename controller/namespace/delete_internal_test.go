package namespace

import (
	"testing"

	"github.com/stretchr/testify/require"

	tcloudcommon "github.com/temporalio/cloud-operator/api/cloud/common"
	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
)

func TestDeletionPolicy(t *testing.T) {
	cases := []struct {
		name string
		// policy is the deletion-policy annotation value. Empty means no
		// annotation.
		policy string
		// adopted is the adopted annotation value. Empty means no
		// annotation.
		adopted string
		exp     tcloudcommon.DeletionPolicy
		expErr  string
	}{
		{
			name:    "no annotation on adopted namespace retains",
			adopted: "true",
			exp:     tcloudcommon.DeletionPolicyRetain,
		},
		{
			name:    "adopted value is case-insensitive and trimmed",
			adopted: " True ",
			exp:     tcloudcommon.DeletionPolicyRetain,
		},
		{
			name: "no annotation on namespace not adopted deletes",
			exp:  tcloudcommon.DeletionPolicyDelete,
		},
		{
			name:    "adopted false deletes",
			adopted: "false",
			exp:     tcloudcommon.DeletionPolicyDelete,
		},
		{
			name:    "invalid adopted value does not fall back to delete",
			adopted: "yes",
			expErr:  `invalid cloud.temporal.io/adopted annotation "yes". allowed values: true, false`,
		},
		{
			name:    "explicit delete on adopted namespace deletes",
			policy:  "delete",
			adopted: "true",
			exp:     tcloudcommon.DeletionPolicyDelete,
		},
		{
			name:    "explicit retain wins over invalid adopted value",
			policy:  "retain",
			adopted: "yes",
			exp:     tcloudcommon.DeletionPolicyRetain,
		},
		{
			name:   "explicit retain on namespace not adopted retains",
			policy: "retain",
			exp:    tcloudcommon.DeletionPolicyRetain,
		},
		{
			name:    "invalid annotation on adopted namespace does not fall back to retain",
			policy:  "destroy",
			adopted: "true",
			exp:     "destroy",
			expErr:  `invalid deletion policy "destroy". allowed values: retain, delete`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := &tcloudv1alpha1.Namespace{}
			cr.Annotations = map[string]string{}
			if tc.policy != "" {
				cr.Annotations[tcloudcommon.AnnotationKeyDeletionPolicy] = tc.policy
			}
			if tc.adopted != "" {
				cr.Annotations[tcloudcommon.AnnotationKeyAdopted] = tc.adopted
			}

			got, err := deletionPolicy(&res{cr})
			if tc.expErr != "" {
				require.EqualError(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.exp, got)
		})
	}
}
