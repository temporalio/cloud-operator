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

type AddUserGroupMemberFunc func(
	context.Context,
	*tcloudservicev1.AddUserGroupMemberRequest,
) (*tcloudservicev1.AddUserGroupMemberResponse, error)

// WithAddUserGroupMember sets the fake server's AddUserGroupMember function override.
func WithAddUserGroupMember(fn AddUserGroupMemberFunc) NewOption {
	return func(s *fakeServer) {
		s.addUserGroupMemberFunc = fn
	}
}

// OnAddUserGroupMember lets tests validate arguments passed to AddUserGroupMember and set its
// response.
func OnAddUserGroupMember(
	asserter func(*tcloudservicev1.AddUserGroupMemberRequest),
	returner func() (*tcloudservicev1.AddUserGroupMemberResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.AddUserGroupMemberRequest,
		) (*tcloudservicev1.AddUserGroupMemberResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.AddUserGroupMemberResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.AddUserGroupMemberResponse{}
			}
			return resp, err
		}
		s.addUserGroupMemberFunc = fn
	}
}

// OnAddUserGroupMemberReturn sets the response returned by AddUserGroupMember.
func OnAddUserGroupMemberReturn(
	resp *tcloudservicev1.AddUserGroupMemberResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.AddUserGroupMemberResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.AddUserGroupMemberRequest,
		) (*tcloudservicev1.AddUserGroupMemberResponse, error) {
			return resp, err
		}
		s.addUserGroupMemberFunc = fn
	}
}

func (s fakeServer) AddUserGroupMember(
	ctx context.Context,
	req *tcloudservicev1.AddUserGroupMemberRequest,
) (*tcloudservicev1.AddUserGroupMemberResponse, error) {
	if s.addUserGroupMemberFunc != nil {
		return s.addUserGroupMemberFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.AddUserGroupMember(ctx, req)
}

type RemoveUserGroupMemberFunc func(
	context.Context,
	*tcloudservicev1.RemoveUserGroupMemberRequest,
) (*tcloudservicev1.RemoveUserGroupMemberResponse, error)

// WithRemoveUserGroupMember sets the fake server's RemoveUserGroupMember function override.
func WithRemoveUserGroupMember(fn RemoveUserGroupMemberFunc) NewOption {
	return func(s *fakeServer) {
		s.removeUserGroupMemberFunc = fn
	}
}

// OnRemoveUserGroupMember lets tests validate arguments passed to RemoveUserGroupMember and set its
// response.
func OnRemoveUserGroupMember(
	asserter func(*tcloudservicev1.RemoveUserGroupMemberRequest),
	returner func() (*tcloudservicev1.RemoveUserGroupMemberResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.RemoveUserGroupMemberRequest,
		) (*tcloudservicev1.RemoveUserGroupMemberResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.RemoveUserGroupMemberResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.RemoveUserGroupMemberResponse{}
			}
			return resp, err
		}
		s.removeUserGroupMemberFunc = fn
	}
}

// OnRemoveUserGroupMemberReturn sets the response returned by RemoveUserGroupMember.
func OnRemoveUserGroupMemberReturn(
	resp *tcloudservicev1.RemoveUserGroupMemberResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.RemoveUserGroupMemberResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.RemoveUserGroupMemberRequest,
		) (*tcloudservicev1.RemoveUserGroupMemberResponse, error) {
			return resp, err
		}
		s.removeUserGroupMemberFunc = fn
	}
}

func (s fakeServer) RemoveUserGroupMember(
	ctx context.Context,
	req *tcloudservicev1.RemoveUserGroupMemberRequest,
) (*tcloudservicev1.RemoveUserGroupMemberResponse, error) {
	if s.removeUserGroupMemberFunc != nil {
		return s.removeUserGroupMemberFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.RemoveUserGroupMember(ctx, req)
}

type GetUserGroupMembersFunc func(
	context.Context,
	*tcloudservicev1.GetUserGroupMembersRequest,
) (*tcloudservicev1.GetUserGroupMembersResponse, error)

// WithGetUserGroupMembers sets the fake server's GetUserGroupMembers function override.
func WithGetUserGroupMembers(fn GetUserGroupMembersFunc) NewOption {
	return func(s *fakeServer) {
		s.getUserGroupMembersFunc = fn
	}
}

// OnGetUserGroupMembers lets tests validate arguments passed to GetUserGroupMembers and set its
// response.
func OnGetUserGroupMembers(
	asserter func(*tcloudservicev1.GetUserGroupMembersRequest),
	returner func() (*tcloudservicev1.GetUserGroupMembersResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.GetUserGroupMembersRequest,
		) (*tcloudservicev1.GetUserGroupMembersResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.GetUserGroupMembersResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.GetUserGroupMembersResponse{}
			}
			return resp, err
		}
		s.getUserGroupMembersFunc = fn
	}
}

// OnGetUserGroupMembersReturn sets the response returned by GetUserGroupMembers.
func OnGetUserGroupMembersReturn(
	resp *tcloudservicev1.GetUserGroupMembersResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.GetUserGroupMembersResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.GetUserGroupMembersRequest,
		) (*tcloudservicev1.GetUserGroupMembersResponse, error) {
			return resp, err
		}
		s.getUserGroupMembersFunc = fn
	}
}

func (s fakeServer) GetUserGroupMembers(
	ctx context.Context,
	req *tcloudservicev1.GetUserGroupMembersRequest,
) (*tcloudservicev1.GetUserGroupMembersResponse, error) {
	if s.getUserGroupMembersFunc != nil {
		return s.getUserGroupMembersFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.GetUserGroupMembers(ctx, req)
}
