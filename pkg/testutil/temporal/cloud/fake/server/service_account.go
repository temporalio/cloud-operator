package server

import (
	"context"

	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type CreateServiceAccountFunc func(
	context.Context,
	*tcloudservicev1.CreateServiceAccountRequest,
) (*tcloudservicev1.CreateServiceAccountResponse, error)

// WithCreateServiceAccount sets the fake server's CreateServiceAccount
// function override.
func WithCreateServiceAccount(fn CreateServiceAccountFunc) NewOption {
	return func(s *fakeServer) {
		s.createServiceAccountFunc = fn
	}
}

// OnCreateServiceAccount lets tests validate arguments passed to
// CreateServiceAccount and set its response.
func OnCreateServiceAccount(
	asserter func(*tcloudservicev1.CreateServiceAccountRequest),
	returner func() (*tcloudservicev1.CreateServiceAccountResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.CreateServiceAccountRequest,
		) (*tcloudservicev1.CreateServiceAccountResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.CreateServiceAccountResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.CreateServiceAccountResponse{}
			}
			return resp, err
		}
		s.createServiceAccountFunc = fn
	}
}

// OnCreateServiceAccountReturn sets the response returned by
// CreateServiceAccount.
func OnCreateServiceAccountReturn(
	resp *tcloudservicev1.CreateServiceAccountResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.CreateServiceAccountResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.CreateServiceAccountRequest,
		) (*tcloudservicev1.CreateServiceAccountResponse, error) {
			return resp, err
		}
		s.createServiceAccountFunc = fn
	}
}

func (s fakeServer) CreateServiceAccount(
	ctx context.Context,
	req *tcloudservicev1.CreateServiceAccountRequest,
) (*tcloudservicev1.CreateServiceAccountResponse, error) {
	if s.createServiceAccountFunc != nil {
		return s.createServiceAccountFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.CreateServiceAccount(ctx, req)
}

type GetServiceAccountFunc func(
	context.Context,
	*tcloudservicev1.GetServiceAccountRequest,
) (*tcloudservicev1.GetServiceAccountResponse, error)

// WithGetServiceAccount sets the fake server's GetServiceAccount function
// override.
func WithGetServiceAccount(fn GetServiceAccountFunc) NewOption {
	return func(s *fakeServer) {
		s.getServiceAccountFunc = fn
	}
}

// OnGetServiceAccount lets tests validate arguments passed to
// GetServiceAccount and set its response.
func OnGetServiceAccount(
	asserter func(*tcloudservicev1.GetServiceAccountRequest),
	returner func() (*tcloudservicev1.GetServiceAccountResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.GetServiceAccountRequest,
		) (*tcloudservicev1.GetServiceAccountResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.GetServiceAccountResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.GetServiceAccountResponse{}
			}
			return resp, err
		}
		s.getServiceAccountFunc = fn
	}
}

// OnGetServiceAccountReturn sets the response returned by GetServiceAccount.
func OnGetServiceAccountReturn(
	resp *tcloudservicev1.GetServiceAccountResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.GetServiceAccountResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.GetServiceAccountRequest,
		) (*tcloudservicev1.GetServiceAccountResponse, error) {
			return resp, err
		}
		s.getServiceAccountFunc = fn
	}
}

func (s fakeServer) GetServiceAccount(
	ctx context.Context,
	req *tcloudservicev1.GetServiceAccountRequest,
) (*tcloudservicev1.GetServiceAccountResponse, error) {
	if s.getServiceAccountFunc != nil {
		return s.getServiceAccountFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.GetServiceAccount(ctx, req)
}

type UpdateServiceAccountFunc func(
	context.Context,
	*tcloudservicev1.UpdateServiceAccountRequest,
) (*tcloudservicev1.UpdateServiceAccountResponse, error)

// WithUpdateServiceAccount sets the fake server's UpdateServiceAccount
// function override.
func WithUpdateServiceAccount(fn UpdateServiceAccountFunc) NewOption {
	return func(s *fakeServer) {
		s.updateServiceAccountFunc = fn
	}
}

// OnUpdateServiceAccount lets tests validate arguments passed to
// UpdateServiceAccount and set its response.
func OnUpdateServiceAccount(
	asserter func(*tcloudservicev1.UpdateServiceAccountRequest),
	returner func() (*tcloudservicev1.UpdateServiceAccountResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.UpdateServiceAccountRequest,
		) (*tcloudservicev1.UpdateServiceAccountResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.UpdateServiceAccountResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.UpdateServiceAccountResponse{}
			}
			return resp, err
		}
		s.updateServiceAccountFunc = fn
	}
}

// OnUpdateServiceAccountReturn sets the response returned by
// UpdateServiceAccount.
func OnUpdateServiceAccountReturn(
	resp *tcloudservicev1.UpdateServiceAccountResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.UpdateServiceAccountResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.UpdateServiceAccountRequest,
		) (*tcloudservicev1.UpdateServiceAccountResponse, error) {
			return resp, err
		}
		s.updateServiceAccountFunc = fn
	}
}

func (s fakeServer) UpdateServiceAccount(
	ctx context.Context,
	req *tcloudservicev1.UpdateServiceAccountRequest,
) (*tcloudservicev1.UpdateServiceAccountResponse, error) {
	if s.updateServiceAccountFunc != nil {
		return s.updateServiceAccountFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.UpdateServiceAccount(ctx, req)
}

type DeleteServiceAccountFunc func(
	context.Context,
	*tcloudservicev1.DeleteServiceAccountRequest,
) (*tcloudservicev1.DeleteServiceAccountResponse, error)

// WithDeleteServiceAccount sets the fake server's DeleteServiceAccount
// function override.
func WithDeleteServiceAccount(fn DeleteServiceAccountFunc) NewOption {
	return func(s *fakeServer) {
		s.deleteServiceAccountFunc = fn
	}
}

// OnDeleteServiceAccount lets tests validate arguments passed to
// DeleteServiceAccount and set its response.
func OnDeleteServiceAccount(
	asserter func(*tcloudservicev1.DeleteServiceAccountRequest),
	returner func() (*tcloudservicev1.DeleteServiceAccountResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.DeleteServiceAccountRequest,
		) (*tcloudservicev1.DeleteServiceAccountResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.DeleteServiceAccountResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.DeleteServiceAccountResponse{}
			}
			return resp, err
		}
		s.deleteServiceAccountFunc = fn
	}
}

// OnDeleteServiceAccountReturn sets the response returned by
// DeleteServiceAccount.
func OnDeleteServiceAccountReturn(
	resp *tcloudservicev1.DeleteServiceAccountResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.DeleteServiceAccountResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.DeleteServiceAccountRequest,
		) (*tcloudservicev1.DeleteServiceAccountResponse, error) {
			return resp, err
		}
		s.deleteServiceAccountFunc = fn
	}
}

func (s fakeServer) DeleteServiceAccount(
	ctx context.Context,
	req *tcloudservicev1.DeleteServiceAccountRequest,
) (*tcloudservicev1.DeleteServiceAccountResponse, error) {
	if s.deleteServiceAccountFunc != nil {
		return s.deleteServiceAccountFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.DeleteServiceAccount(ctx, req)
}
