package ptr_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/temporalio/cloud-operator/pkg/util/ptr"
)

type spec struct {
	Name  string
	Tags  map[string]string
	Items []string
	Inner *spec
}

func TestNilIfEmpty_Struct(t *testing.T) {
	cases := []struct {
		name   string
		in     *spec
		expNil bool
	}{
		{
			name:   "nil pointer",
			in:     nil,
			expNil: true,
		},
		{
			name:   "zero value",
			in:     &spec{},
			expNil: true,
		},
		{
			// Semantic equality treats empty and nil maps and slices as
			// equal, so a block Temporal Cloud returns with empty collections
			// still counts as empty.
			name:   "empty map and slice",
			in:     &spec{Tags: map[string]string{}, Items: []string{}},
			expNil: true,
		},
		{
			name: "set field",
			in:   &spec{Name: "a"},
		},
		{
			name: "non-empty map",
			in:   &spec{Tags: map[string]string{"a": "1"}},
		},
		{
			name: "non-empty slice",
			in:   &spec{Items: []string{"a"}},
		},
		{
			// A set pointer is not the zero value, even if it points to one.
			name: "pointer to zero value",
			in:   &spec{Inner: &spec{}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			got := ptr.NilIfEmpty(tc.in)

			if tc.expNil {
				require.Nil(got)
				return
			}
			// Returns the same pointer, not a copy.
			require.Same(tc.in, got)
		})
	}
}

func TestNilIfEmpty_Scalar(t *testing.T) {
	require := require.New(t)

	zero, set := 0, 7
	require.Nil(ptr.NilIfEmpty(&zero))
	require.Same(&set, ptr.NilIfEmpty(&set))

	empty, name := "", "a"
	require.Nil(ptr.NilIfEmpty(&empty))
	require.Same(&name, ptr.NilIfEmpty(&name))
}

func TestNilIfEmpty_DoesNotMutate(t *testing.T) {
	require := require.New(t)

	in := &spec{Tags: map[string]string{}}
	ptr.NilIfEmpty(in)

	require.Equal(&spec{Tags: map[string]string{}}, in)
}
