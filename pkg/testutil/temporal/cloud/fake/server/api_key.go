package server

import (
	"context"

	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type CreateApiKeyFunc func(
	context.Context,
	*tcloudservicev1.CreateApiKeyRequest,
) (*tcloudservicev1.CreateApiKeyResponse, error)

// WithCreateApiKey sets the fake server's CreateApiKey function override.
func WithCreateApiKey(fn CreateApiKeyFunc) NewOption {
	return func(s *fakeServer) {
		s.createApiKeyFunc = fn
	}
}

// OnCreateApiKey lets tests validate arguments passed to CreateApiKey and set
// its response.
func OnCreateApiKey(
	asserter func(*tcloudservicev1.CreateApiKeyRequest),
	returner func() (*tcloudservicev1.CreateApiKeyResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.CreateApiKeyRequest,
		) (*tcloudservicev1.CreateApiKeyResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.CreateApiKeyResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.CreateApiKeyResponse{}
			}
			return resp, err
		}
		s.createApiKeyFunc = fn
	}
}

// OnCreateApiKeyReturn sets the response returned by CreateApiKey.
func OnCreateApiKeyReturn(
	resp *tcloudservicev1.CreateApiKeyResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.CreateApiKeyResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.CreateApiKeyRequest,
		) (*tcloudservicev1.CreateApiKeyResponse, error) {
			return resp, err
		}
		s.createApiKeyFunc = fn
	}
}

func (s fakeServer) CreateApiKey(
	ctx context.Context,
	req *tcloudservicev1.CreateApiKeyRequest,
) (*tcloudservicev1.CreateApiKeyResponse, error) {
	if s.createApiKeyFunc != nil {
		return s.createApiKeyFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.CreateApiKey(ctx, req)
}

type GetApiKeyFunc func(
	context.Context,
	*tcloudservicev1.GetApiKeyRequest,
) (*tcloudservicev1.GetApiKeyResponse, error)

// WithGetApiKey sets the fake server's GetApiKey function override.
func WithGetApiKey(fn GetApiKeyFunc) NewOption {
	return func(s *fakeServer) {
		s.getApiKeyFunc = fn
	}
}

// OnGetApiKey lets tests validate arguments passed to GetApiKey and set its
// response.
func OnGetApiKey(
	asserter func(*tcloudservicev1.GetApiKeyRequest),
	returner func() (*tcloudservicev1.GetApiKeyResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.GetApiKeyRequest,
		) (*tcloudservicev1.GetApiKeyResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.GetApiKeyResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.GetApiKeyResponse{}
			}
			return resp, err
		}
		s.getApiKeyFunc = fn
	}
}

// OnGetApiKeyReturn sets the response returned by GetApiKey.
func OnGetApiKeyReturn(
	resp *tcloudservicev1.GetApiKeyResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.GetApiKeyResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.GetApiKeyRequest,
		) (*tcloudservicev1.GetApiKeyResponse, error) {
			return resp, err
		}
		s.getApiKeyFunc = fn
	}
}

func (s fakeServer) GetApiKey(
	ctx context.Context,
	req *tcloudservicev1.GetApiKeyRequest,
) (*tcloudservicev1.GetApiKeyResponse, error) {
	if s.getApiKeyFunc != nil {
		return s.getApiKeyFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.GetApiKey(ctx, req)
}

type UpdateApiKeyFunc func(
	context.Context,
	*tcloudservicev1.UpdateApiKeyRequest,
) (*tcloudservicev1.UpdateApiKeyResponse, error)

// WithUpdateApiKey sets the fake server's UpdateApiKey function override.
func WithUpdateApiKey(fn UpdateApiKeyFunc) NewOption {
	return func(s *fakeServer) {
		s.updateApiKeyFunc = fn
	}
}

// OnUpdateApiKey lets tests validate arguments passed to UpdateApiKey and set
// its response.
func OnUpdateApiKey(
	asserter func(*tcloudservicev1.UpdateApiKeyRequest),
	returner func() (*tcloudservicev1.UpdateApiKeyResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.UpdateApiKeyRequest,
		) (*tcloudservicev1.UpdateApiKeyResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.UpdateApiKeyResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.UpdateApiKeyResponse{}
			}
			return resp, err
		}
		s.updateApiKeyFunc = fn
	}
}

// OnUpdateApiKeyReturn sets the response returned by UpdateApiKey.
func OnUpdateApiKeyReturn(
	resp *tcloudservicev1.UpdateApiKeyResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.UpdateApiKeyResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.UpdateApiKeyRequest,
		) (*tcloudservicev1.UpdateApiKeyResponse, error) {
			return resp, err
		}
		s.updateApiKeyFunc = fn
	}
}

func (s fakeServer) UpdateApiKey(
	ctx context.Context,
	req *tcloudservicev1.UpdateApiKeyRequest,
) (*tcloudservicev1.UpdateApiKeyResponse, error) {
	if s.updateApiKeyFunc != nil {
		return s.updateApiKeyFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.UpdateApiKey(ctx, req)
}
