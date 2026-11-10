package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceReferenceKey(t *testing.T) {
	cases := []struct {
		name string
		in   ResourceReference
		exp  string
	}{
		{
			name: "namespace set",
			in:   ResourceReference{Namespace: "shared", Name: "billing"},
			exp:  "shared/billing",
		},
		{
			name: "no namespace uses the referrer's",
			in:   ResourceReference{Name: "payments"},
			exp:  "apps/payments",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.exp, tc.in.Key("apps"))
		})
	}
}
