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

type GetNamespacesFunc func(
	context.Context,
	*tcloudservicev1.GetNamespacesRequest,
) (*tcloudservicev1.GetNamespacesResponse, error)

type GetNamespacesAssertFunc func(*tcloudservicev1.GetNamespacesRequest)

// WithGetNamespaces sets the fake server's GetNamespaces function override.
func WithGetNamespaces(fn GetNamespacesFunc) NewOption {
	return func(s *fakeServer) {
		s.getNamespacesFunc = fn
	}
}

// OnGetNamespacesAssert allows the test author to supply a function that
// checks arguments passed to the GetNamespaces method are expected.
func OnGetNamespacesAssert(
	asserter GetNamespacesAssertFunc,
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.GetNamespacesRequest,
		) (*tcloudservicev1.GetNamespacesResponse, error) {
			asserter(req)
			return nil, nil
		}
		s.getNamespacesFunc = fn
	}
}

// OnGetNamespacesReturn allows the test author to set the returned value from
// the GetNamespaces method.
func OnGetNamespacesReturn(
	resp *tcloudservicev1.GetNamespacesResponse,
	err error,
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.GetNamespacesRequest,
		) (*tcloudservicev1.GetNamespacesResponse, error) {
			return resp, err
		}
		s.getNamespacesFunc = fn
	}
}

func (s fakeServer) GetNamespaces(
	ctx context.Context,
	req *tcloudservicev1.GetNamespacesRequest,
) (*tcloudservicev1.GetNamespacesResponse, error) {
	if s.getNamespacesFunc != nil {
		return s.getNamespacesFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.GetNamespaces(ctx, req)
}

type DeleteNamespaceFunc func(
	context.Context,
	*tcloudservicev1.DeleteNamespaceRequest,
) (*tcloudservicev1.DeleteNamespaceResponse, error)

// WithDeleteNamespace sets the fake server's DeleteNamespace function
// override.
func WithDeleteNamespace(fn DeleteNamespaceFunc) NewOption {
	return func(s *fakeServer) {
		s.deleteNamespaceFunc = fn
	}
}

// OnDeleteNamespace lets tests validate arguments passed to DeleteNamespace
// and set its response.
func OnDeleteNamespace(
	asserter func(*tcloudservicev1.DeleteNamespaceRequest),
	returner func() (*tcloudservicev1.DeleteNamespaceResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.DeleteNamespaceRequest,
		) (*tcloudservicev1.DeleteNamespaceResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.DeleteNamespaceResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.DeleteNamespaceResponse{}
			}
			return resp, err
		}
		s.deleteNamespaceFunc = fn
	}
}

// OnDeleteNamespaceReturn sets the response returned by DeleteNamespace.
func OnDeleteNamespaceReturn(
	resp *tcloudservicev1.DeleteNamespaceResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.DeleteNamespaceResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.DeleteNamespaceRequest,
		) (*tcloudservicev1.DeleteNamespaceResponse, error) {
			return resp, err
		}
		s.deleteNamespaceFunc = fn
	}
}

func (s fakeServer) DeleteNamespace(
	ctx context.Context,
	req *tcloudservicev1.DeleteNamespaceRequest,
) (*tcloudservicev1.DeleteNamespaceResponse, error) {
	if s.deleteNamespaceFunc != nil {
		return s.deleteNamespaceFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.DeleteNamespace(ctx, req)
}

type UpdateNamespaceFunc func(
	context.Context,
	*tcloudservicev1.UpdateNamespaceRequest,
) (*tcloudservicev1.UpdateNamespaceResponse, error)

// WithUpdateNamespace sets the fake server's UpdateNamespace function
// override.
func WithUpdateNamespace(fn UpdateNamespaceFunc) NewOption {
	return func(s *fakeServer) {
		s.updateNamespaceFunc = fn
	}
}

// OnUpdateNamespace lets tests validate arguments passed to UpdateNamespace
// and set its response.
func OnUpdateNamespace(
	asserter func(*tcloudservicev1.UpdateNamespaceRequest),
	returner func() (*tcloudservicev1.UpdateNamespaceResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.UpdateNamespaceRequest,
		) (*tcloudservicev1.UpdateNamespaceResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.UpdateNamespaceResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.UpdateNamespaceResponse{}
			}
			return resp, err
		}
		s.updateNamespaceFunc = fn
	}
}

// OnUpdateNamespaceReturn sets the response returned by UpdateNamespace.
func OnUpdateNamespaceReturn(
	resp *tcloudservicev1.UpdateNamespaceResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.UpdateNamespaceResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.UpdateNamespaceRequest,
		) (*tcloudservicev1.UpdateNamespaceResponse, error) {
			return resp, err
		}
		s.updateNamespaceFunc = fn
	}
}

func (s fakeServer) UpdateNamespace(
	ctx context.Context,
	req *tcloudservicev1.UpdateNamespaceRequest,
) (*tcloudservicev1.UpdateNamespaceResponse, error) {
	if s.updateNamespaceFunc != nil {
		return s.updateNamespaceFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.UpdateNamespace(ctx, req)
}
