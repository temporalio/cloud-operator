package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeletionPolicyValidate(t *testing.T) {
	cases := []struct {
		in     DeletionPolicy
		expErr bool
	}{
		{"retain", false},
		{"delete", false},
		{"Retain", false},
		{"RETAIN", false},
		{"Delete", false},
		{"dElEtE", false},
		{"", true},
		{"orphan", true},
		{"retained", true},
		{" retain ", true},
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			err := tc.in.Validate()
			if !tc.expErr {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, `invalid deletion policy "`+string(tc.in)+`". allowed values: retain, delete`)
		})
	}
}

func TestContainsFold(t *testing.T) {
	validValues := []string{"retain", "delete"}
	require.True(t, ContainsFold("retain", validValues))
	require.True(t, ContainsFold("Retain", validValues))
	require.True(t, ContainsFold("DELETE", validValues))
	require.False(t, ContainsFold("orphan", validValues))
	require.False(t, ContainsFold("retain", nil))
}
