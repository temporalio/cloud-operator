package server

import (
	"context"

	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type GetAsyncOperationFunc func(
	context.Context,
	*tcloudservicev1.GetAsyncOperationRequest,
) (*tcloudservicev1.GetAsyncOperationResponse, error)

// WithGetAsyncOperation sets the fake server's GetAsyncOperation function
// override.
func WithGetAsyncOperation(fn GetAsyncOperationFunc) NewOption {
	return func(s *fakeServer) {
		s.getAsyncOperationFunc = fn
	}
}

// OnGetAsyncOperation lets tests validate arguments passed to GetAsyncOperation
// and set its response.
func OnGetAsyncOperation(
	asserter func(*tcloudservicev1.GetAsyncOperationRequest),
	returner func() (*tcloudservicev1.GetAsyncOperationResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.GetAsyncOperationRequest,
		) (*tcloudservicev1.GetAsyncOperationResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.GetAsyncOperationResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.GetAsyncOperationResponse{}
			}
			return resp, err
		}
		s.getAsyncOperationFunc = fn
	}
}

// OnGetAsyncOperationReturn sets the response returned by GetAsyncOperation.
func OnGetAsyncOperationReturn(
	resp *tcloudservicev1.GetAsyncOperationResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.GetAsyncOperationResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.GetAsyncOperationRequest,
		) (*tcloudservicev1.GetAsyncOperationResponse, error) {
			return resp, err
		}
		s.getAsyncOperationFunc = fn
	}
}

func (s fakeServer) GetAsyncOperation(
	ctx context.Context,
	req *tcloudservicev1.GetAsyncOperationRequest,
) (*tcloudservicev1.GetAsyncOperationResponse, error) {
	if s.getAsyncOperationFunc != nil {
		return s.getAsyncOperationFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.GetAsyncOperation(ctx, req)
}
