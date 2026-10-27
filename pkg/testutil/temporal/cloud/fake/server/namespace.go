package server

import (
	"context"

	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type GetNamespaceFunc func(
	context.Context,
	*tcloudservicev1.GetNamespaceRequest,
) (*tcloudservicev1.GetNamespaceResponse, error)

type GetNamespaceAssertFunc func(*tcloudservicev1.GetNamespaceRequest)

// WithGetNamespace sets the fake server's GetNamespace function override.
func WithGetNamespace(fn GetNamespaceFunc) NewOption {
	return func(s *fakeServer) {
		s.getNamespaceFunc = fn
	}
}

// OnGetNamespaceAssert allows the test author to supply a function that checks
// arguments passed to the GetNamespace method are expected.
func OnGetNamespaceAssert(
	asserter GetNamespaceAssertFunc,
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.GetNamespaceRequest,
		) (*tcloudservicev1.GetNamespaceResponse, error) {
			asserter(req)
			return nil, nil
		}
		s.getNamespaceFunc = fn
	}
}

// OnGetNamespaceReturn allows the test author to set the returned value from
// the GetNamespace method.
func OnGetNamespaceReturn(
	resp *tcloudservicev1.GetNamespaceResponse,
	err error,
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.GetNamespaceRequest,
		) (*tcloudservicev1.GetNamespaceResponse, error) {
			return resp, err
		}
		s.getNamespaceFunc = fn
	}
}

func (s fakeServer) GetNamespace(
	ctx context.Context,
	req *tcloudservicev1.GetNamespaceRequest,
) (*tcloudservicev1.GetNamespaceResponse, error) {
	if s.getNamespaceFunc != nil {
		return s.getNamespaceFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.GetNamespace(ctx, req)
}
