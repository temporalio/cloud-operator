package server

import (
	"context"

	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type CreateUserFunc func(
	context.Context,
	*tcloudservicev1.CreateUserRequest,
) (*tcloudservicev1.CreateUserResponse, error)

// WithCreateUser sets the fake server's CreateUser function override.
func WithCreateUser(fn CreateUserFunc) NewOption {
	return func(s *fakeServer) {
		s.createUserFunc = fn
	}
}

// OnCreateUser lets tests validate arguments passed to CreateUser and set its
// response.
func OnCreateUser(
	asserter func(*tcloudservicev1.CreateUserRequest),
	returner func() (*tcloudservicev1.CreateUserResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.CreateUserRequest,
		) (*tcloudservicev1.CreateUserResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.CreateUserResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.CreateUserResponse{}
			}
			return resp, err
		}
		s.createUserFunc = fn
	}
}

// OnCreateUserReturn sets the response returned by CreateUser.
func OnCreateUserReturn(
	resp *tcloudservicev1.CreateUserResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.CreateUserResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.CreateUserRequest,
		) (*tcloudservicev1.CreateUserResponse, error) {
			return resp, err
		}
		s.createUserFunc = fn
	}
}

func (s fakeServer) CreateUser(
	ctx context.Context,
	req *tcloudservicev1.CreateUserRequest,
) (*tcloudservicev1.CreateUserResponse, error) {
	if s.createUserFunc != nil {
		return s.createUserFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.CreateUser(ctx, req)
}

type GetUserFunc func(
	context.Context,
	*tcloudservicev1.GetUserRequest,
) (*tcloudservicev1.GetUserResponse, error)

// WithGetUser sets the fake server's GetUser function override.
func WithGetUser(fn GetUserFunc) NewOption {
	return func(s *fakeServer) {
		s.getUserFunc = fn
	}
}

// OnGetUser lets tests validate arguments passed to GetUser and set its
// response.
func OnGetUser(
	asserter func(*tcloudservicev1.GetUserRequest),
	returner func() (*tcloudservicev1.GetUserResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.GetUserRequest,
		) (*tcloudservicev1.GetUserResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.GetUserResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.GetUserResponse{}
			}
			return resp, err
		}
		s.getUserFunc = fn
	}
}

// OnGetUserReturn sets the response returned by GetUser.
func OnGetUserReturn(
	resp *tcloudservicev1.GetUserResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.GetUserResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.GetUserRequest,
		) (*tcloudservicev1.GetUserResponse, error) {
			return resp, err
		}
		s.getUserFunc = fn
	}
}

func (s fakeServer) GetUser(
	ctx context.Context,
	req *tcloudservicev1.GetUserRequest,
) (*tcloudservicev1.GetUserResponse, error) {
	if s.getUserFunc != nil {
		return s.getUserFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.GetUser(ctx, req)
}

type UpdateUserFunc func(
	context.Context,
	*tcloudservicev1.UpdateUserRequest,
) (*tcloudservicev1.UpdateUserResponse, error)

// WithUpdateUser sets the fake server's UpdateUser function override.
func WithUpdateUser(fn UpdateUserFunc) NewOption {
	return func(s *fakeServer) {
		s.updateUserFunc = fn
	}
}

// OnUpdateUser lets tests validate arguments passed to UpdateUser and set its
// response.
func OnUpdateUser(
	asserter func(*tcloudservicev1.UpdateUserRequest),
	returner func() (*tcloudservicev1.UpdateUserResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.UpdateUserRequest,
		) (*tcloudservicev1.UpdateUserResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.UpdateUserResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.UpdateUserResponse{}
			}
			return resp, err
		}
		s.updateUserFunc = fn
	}
}

// OnUpdateUserReturn sets the response returned by UpdateUser.
func OnUpdateUserReturn(
	resp *tcloudservicev1.UpdateUserResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.UpdateUserResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.UpdateUserRequest,
		) (*tcloudservicev1.UpdateUserResponse, error) {
			return resp, err
		}
		s.updateUserFunc = fn
	}
}

func (s fakeServer) UpdateUser(
	ctx context.Context,
	req *tcloudservicev1.UpdateUserRequest,
) (*tcloudservicev1.UpdateUserResponse, error) {
	if s.updateUserFunc != nil {
		return s.updateUserFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.UpdateUser(ctx, req)
}

type DeleteUserFunc func(
	context.Context,
	*tcloudservicev1.DeleteUserRequest,
) (*tcloudservicev1.DeleteUserResponse, error)

// WithDeleteUser sets the fake server's DeleteUser function override.
func WithDeleteUser(fn DeleteUserFunc) NewOption {
	return func(s *fakeServer) {
		s.deleteUserFunc = fn
	}
}

// OnDeleteUser lets tests validate arguments passed to DeleteUser and set its
// response.
func OnDeleteUser(
	asserter func(*tcloudservicev1.DeleteUserRequest),
	returner func() (*tcloudservicev1.DeleteUserResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.DeleteUserRequest,
		) (*tcloudservicev1.DeleteUserResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.DeleteUserResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.DeleteUserResponse{}
			}
			return resp, err
		}
		s.deleteUserFunc = fn
	}
}

// OnDeleteUserReturn sets the response returned by DeleteUser.
func OnDeleteUserReturn(
	resp *tcloudservicev1.DeleteUserResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.DeleteUserResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.DeleteUserRequest,
		) (*tcloudservicev1.DeleteUserResponse, error) {
			return resp, err
		}
		s.deleteUserFunc = fn
	}
}

func (s fakeServer) DeleteUser(
	ctx context.Context,
	req *tcloudservicev1.DeleteUserRequest,
) (*tcloudservicev1.DeleteUserResponse, error) {
	if s.deleteUserFunc != nil {
		return s.deleteUserFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.DeleteUser(ctx, req)
}
