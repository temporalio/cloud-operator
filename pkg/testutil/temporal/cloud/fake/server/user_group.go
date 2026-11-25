package server

import (
	"context"

	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type CreateUserGroupFunc func(
	context.Context,
	*tcloudservicev1.CreateUserGroupRequest,
) (*tcloudservicev1.CreateUserGroupResponse, error)

// WithCreateUserGroup sets the fake server's CreateUserGroup function override.
func WithCreateUserGroup(fn CreateUserGroupFunc) NewOption {
	return func(s *fakeServer) {
		s.createUserGroupFunc = fn
	}
}

// OnCreateUserGroup lets tests validate arguments passed to CreateUserGroup and set its
// response.
func OnCreateUserGroup(
	asserter func(*tcloudservicev1.CreateUserGroupRequest),
	returner func() (*tcloudservicev1.CreateUserGroupResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.CreateUserGroupRequest,
		) (*tcloudservicev1.CreateUserGroupResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.CreateUserGroupResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.CreateUserGroupResponse{}
			}
			return resp, err
		}
		s.createUserGroupFunc = fn
	}
}

// OnCreateUserGroupReturn sets the response returned by CreateUserGroup.
func OnCreateUserGroupReturn(
	resp *tcloudservicev1.CreateUserGroupResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.CreateUserGroupResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.CreateUserGroupRequest,
		) (*tcloudservicev1.CreateUserGroupResponse, error) {
			return resp, err
		}
		s.createUserGroupFunc = fn
	}
}

func (s fakeServer) CreateUserGroup(
	ctx context.Context,
	req *tcloudservicev1.CreateUserGroupRequest,
) (*tcloudservicev1.CreateUserGroupResponse, error) {
	if s.createUserGroupFunc != nil {
		return s.createUserGroupFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.CreateUserGroup(ctx, req)
}

type GetUserGroupFunc func(
	context.Context,
	*tcloudservicev1.GetUserGroupRequest,
) (*tcloudservicev1.GetUserGroupResponse, error)

// WithGetUserGroup sets the fake server's GetUserGroup function override.
func WithGetUserGroup(fn GetUserGroupFunc) NewOption {
	return func(s *fakeServer) {
		s.getUserGroupFunc = fn
	}
}

// OnGetUserGroup lets tests validate arguments passed to GetUserGroup and set its
// response.
func OnGetUserGroup(
	asserter func(*tcloudservicev1.GetUserGroupRequest),
	returner func() (*tcloudservicev1.GetUserGroupResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.GetUserGroupRequest,
		) (*tcloudservicev1.GetUserGroupResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.GetUserGroupResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.GetUserGroupResponse{}
			}
			return resp, err
		}
		s.getUserGroupFunc = fn
	}
}

// OnGetUserGroupReturn sets the response returned by GetUserGroup.
func OnGetUserGroupReturn(
	resp *tcloudservicev1.GetUserGroupResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.GetUserGroupResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.GetUserGroupRequest,
		) (*tcloudservicev1.GetUserGroupResponse, error) {
			return resp, err
		}
		s.getUserGroupFunc = fn
	}
}

func (s fakeServer) GetUserGroup(
	ctx context.Context,
	req *tcloudservicev1.GetUserGroupRequest,
) (*tcloudservicev1.GetUserGroupResponse, error) {
	if s.getUserGroupFunc != nil {
		return s.getUserGroupFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.GetUserGroup(ctx, req)
}

type UpdateUserGroupFunc func(
	context.Context,
	*tcloudservicev1.UpdateUserGroupRequest,
) (*tcloudservicev1.UpdateUserGroupResponse, error)

// WithUpdateUserGroup sets the fake server's UpdateUserGroup function override.
func WithUpdateUserGroup(fn UpdateUserGroupFunc) NewOption {
	return func(s *fakeServer) {
		s.updateUserGroupFunc = fn
	}
}

// OnUpdateUserGroup lets tests validate arguments passed to UpdateUserGroup and set its
// response.
func OnUpdateUserGroup(
	asserter func(*tcloudservicev1.UpdateUserGroupRequest),
	returner func() (*tcloudservicev1.UpdateUserGroupResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.UpdateUserGroupRequest,
		) (*tcloudservicev1.UpdateUserGroupResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.UpdateUserGroupResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.UpdateUserGroupResponse{}
			}
			return resp, err
		}
		s.updateUserGroupFunc = fn
	}
}

// OnUpdateUserGroupReturn sets the response returned by UpdateUserGroup.
func OnUpdateUserGroupReturn(
	resp *tcloudservicev1.UpdateUserGroupResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.UpdateUserGroupResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.UpdateUserGroupRequest,
		) (*tcloudservicev1.UpdateUserGroupResponse, error) {
			return resp, err
		}
		s.updateUserGroupFunc = fn
	}
}

func (s fakeServer) UpdateUserGroup(
	ctx context.Context,
	req *tcloudservicev1.UpdateUserGroupRequest,
) (*tcloudservicev1.UpdateUserGroupResponse, error) {
	if s.updateUserGroupFunc != nil {
		return s.updateUserGroupFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.UpdateUserGroup(ctx, req)
}

type DeleteUserGroupFunc func(
	context.Context,
	*tcloudservicev1.DeleteUserGroupRequest,
) (*tcloudservicev1.DeleteUserGroupResponse, error)

// WithDeleteUserGroup sets the fake server's DeleteUserGroup function override.
func WithDeleteUserGroup(fn DeleteUserGroupFunc) NewOption {
	return func(s *fakeServer) {
		s.deleteUserGroupFunc = fn
	}
}

// OnDeleteUserGroup lets tests validate arguments passed to DeleteUserGroup and set its
// response.
func OnDeleteUserGroup(
	asserter func(*tcloudservicev1.DeleteUserGroupRequest),
	returner func() (*tcloudservicev1.DeleteUserGroupResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.DeleteUserGroupRequest,
		) (*tcloudservicev1.DeleteUserGroupResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.DeleteUserGroupResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.DeleteUserGroupResponse{}
			}
			return resp, err
		}
		s.deleteUserGroupFunc = fn
	}
}

// OnDeleteUserGroupReturn sets the response returned by DeleteUserGroup.
func OnDeleteUserGroupReturn(
	resp *tcloudservicev1.DeleteUserGroupResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.DeleteUserGroupResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.DeleteUserGroupRequest,
		) (*tcloudservicev1.DeleteUserGroupResponse, error) {
			return resp, err
		}
		s.deleteUserGroupFunc = fn
	}
}

func (s fakeServer) DeleteUserGroup(
	ctx context.Context,
	req *tcloudservicev1.DeleteUserGroupRequest,
) (*tcloudservicev1.DeleteUserGroupResponse, error) {
	if s.deleteUserGroupFunc != nil {
		return s.deleteUserGroupFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.DeleteUserGroup(ctx, req)
}
