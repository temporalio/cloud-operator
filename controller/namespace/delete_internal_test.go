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
		source tcloudcommon.ResourceSource
		exp    tcloudcommon.DeletionPolicy
		expErr string
	}{
		{
			name:   "no annotation on adopted namespace retains",
			source: tcloudcommon.ResourceSourceAdopted,
			exp:    tcloudcommon.DeletionPolicyRetain,
		},
		{
			name: "no annotation on namespace not adopted deletes",
			exp:  tcloudcommon.DeletionPolicyDelete,
		},
		{
			name:   "explicit delete on adopted namespace deletes",
			policy: "delete",
			source: tcloudcommon.ResourceSourceAdopted,
			exp:    tcloudcommon.DeletionPolicyDelete,
		},
		{
			name:   "explicit retain on namespace not adopted retains",
			policy: "retain",
			exp:    tcloudcommon.DeletionPolicyRetain,
		},
		{
			name:   "invalid annotation on adopted namespace does not fall back to retain",
			policy: "destroy",
			source: tcloudcommon.ResourceSourceAdopted,
			exp:    "destroy",
			expErr: `invalid deletion policy "destroy". allowed values: retain, delete`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cr := &tcloudv1alpha1.Namespace{}
			if tc.policy != "" {
				cr.Annotations = map[string]string{
					tcloudcommon.AnnotationKeyDeletionPolicy: tc.policy,
				}
			}
			cr.Status.Source = tc.source

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
