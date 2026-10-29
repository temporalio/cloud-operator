package namespace

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcloudnamespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// fakeCloudService implements GetNamespaces by returning the supplied pages
// in order. Calling any other method panics via the nil embedded interface.
type fakeCloudService struct {
	tcloudservicev1.CloudServiceClient

	pages []*tcloudservicev1.GetNamespacesResponse
	err   error
	// reqs records a copy of every request, since findNamespaceByName
	// reuses and mutates one request across pages.
	reqs []*tcloudservicev1.GetNamespacesRequest
}

func (f *fakeCloudService) GetNamespaces(
	_ context.Context,
	req *tcloudservicev1.GetNamespacesRequest,
	_ ...grpc.CallOption,
) (*tcloudservicev1.GetNamespacesResponse, error) {
	f.reqs = append(f.reqs, proto.Clone(req).(*tcloudservicev1.GetNamespacesRequest))
	if f.err != nil {
		return nil, f.err
	}
	return f.pages[len(f.reqs)-1], nil
}

func namespaceNamed(name string) *tcloudnamespacev1.Namespace {
	return &tcloudnamespacev1.Namespace{
		Namespace: name + ".acct1",
		Spec:      &tcloudnamespacev1.NamespaceSpec{Name: name},
	}
}

func TestFindNamespaceByName(t *testing.T) {
	cases := []struct {
		name          string
		fake          *fakeCloudService
		expID         string
		expErr        string
		expPageTokens []string
	}{
		{
			name: "found on first page",
			fake: &fakeCloudService{
				pages: []*tcloudservicev1.GetNamespacesResponse{
					{Namespaces: []*tcloudnamespacev1.Namespace{namespaceNamed("my-ns")}},
				},
			},
			expID:         "my-ns.acct1",
			expPageTokens: []string{""},
		},
		{
			name: "found on a later page",
			fake: &fakeCloudService{
				pages: []*tcloudservicev1.GetNamespacesResponse{
					{NextPageToken: "p2"},
					{
						Namespaces:    []*tcloudnamespacev1.Namespace{namespaceNamed("my-ns")},
						NextPageToken: "p3",
					},
				},
			},
			expID:         "my-ns.acct1",
			expPageTokens: []string{"", "p2"},
		},
		{
			name: "ignores names that are not an exact match",
			fake: &fakeCloudService{
				pages: []*tcloudservicev1.GetNamespacesResponse{
					{Namespaces: []*tcloudnamespacev1.Namespace{
						namespaceNamed("my-ns-2"),
						namespaceNamed("my-ns"),
					}},
				},
			},
			expID:         "my-ns.acct1",
			expPageTokens: []string{""},
		},
		{
			name: "nil when no page matches",
			fake: &fakeCloudService{
				pages: []*tcloudservicev1.GetNamespacesResponse{
					{
						Namespaces:    []*tcloudnamespacev1.Namespace{namespaceNamed("other")},
						NextPageToken: "p2",
					},
					{Namespaces: []*tcloudnamespacev1.Namespace{namespaceNamed("my-ns-2")}},
				},
			},
			expPageTokens: []string{"", "p2"},
		},
		{
			name: "nil when there are no namespaces",
			fake: &fakeCloudService{
				pages: []*tcloudservicev1.GetNamespacesResponse{{}},
			},
			expPageTokens: []string{""},
		},
		{
			name:          "returns client errors",
			fake:          &fakeCloudService{err: errors.New("boom")},
			expErr:        "boom",
			expPageTokens: []string{""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			got, err := findNamespaceByName(context.TODO(), tc.fake, "my-ns")

			var tokens []string
			for _, req := range tc.fake.reqs {
				require.Equal("my-ns", req.GetName())
				tokens = append(tokens, req.GetPageToken())
			}
			require.Equal(tc.expPageTokens, tokens)

			if tc.expErr != "" {
				require.ErrorContains(err, tc.expErr)
				return
			}
			require.Nil(err)
			if tc.expID == "" {
				require.Nil(got)
				return
			}
			require.Equal(tc.expID, got.GetNamespace())
		})
	}
}

func TestTagsDiff(t *testing.T) {
	cases := []struct {
		name      string
		desired   map[string]string
		latest    map[string]string
		expUpsert map[string]string
		expRemove []string
	}{
		{
			name: "both empty",
		},
		{
			name:    "equal",
			desired: map[string]string{"a": "1", "b": "2"},
			latest:  map[string]string{"a": "1", "b": "2"},
		},
		{
			name:      "adds new tags",
			desired:   map[string]string{"a": "1", "b": "2"},
			latest:    map[string]string{"a": "1"},
			expUpsert: map[string]string{"b": "2"},
		},
		{
			name:      "updates changed values",
			desired:   map[string]string{"a": "new"},
			latest:    map[string]string{"a": "old"},
			expUpsert: map[string]string{"a": "new"},
		},
		{
			name:      "removes missing tags in sorted order",
			desired:   map[string]string{"a": "1"},
			latest:    map[string]string{"a": "1", "c": "3", "b": "2"},
			expRemove: []string{"b", "c"},
		},
		{
			name:      "removes everything when desired is nil",
			latest:    map[string]string{"a": "1"},
			expRemove: []string{"a"},
		},
		{
			name:      "upserts and removes together",
			desired:   map[string]string{"a": "new", "b": "2"},
			latest:    map[string]string{"a": "old", "c": "3"},
			expUpsert: map[string]string{"a": "new", "b": "2"},
			expRemove: []string{"c"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(tt *testing.T) {
			require := require.New(tt)

			upsert, remove := tagsDiff(tc.desired, tc.latest)

			require.Equal(tc.expUpsert, upsert)
			require.Equal(tc.expRemove, remove)
		})
	}
}
