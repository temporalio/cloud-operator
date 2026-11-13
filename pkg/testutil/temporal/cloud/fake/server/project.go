package server

import (
	"context"

	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type CreateProjectFunc func(
	context.Context,
	*tcloudservicev1.CreateProjectRequest,
) (*tcloudservicev1.CreateProjectResponse, error)

// WithCreateProject sets the fake server's CreateProject
// function override.
func WithCreateProject(fn CreateProjectFunc) NewOption {
	return func(s *fakeServer) {
		s.createProjectFunc = fn
	}
}

// OnCreateProject lets tests validate arguments passed to
// CreateProject and set its response.
func OnCreateProject(
	asserter func(*tcloudservicev1.CreateProjectRequest),
	returner func() (*tcloudservicev1.CreateProjectResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.CreateProjectRequest,
		) (*tcloudservicev1.CreateProjectResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.CreateProjectResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.CreateProjectResponse{}
			}
			return resp, err
		}
		s.createProjectFunc = fn
	}
}

// OnCreateProjectReturn sets the response returned by
// CreateProject.
func OnCreateProjectReturn(
	resp *tcloudservicev1.CreateProjectResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.CreateProjectResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.CreateProjectRequest,
		) (*tcloudservicev1.CreateProjectResponse, error) {
			return resp, err
		}
		s.createProjectFunc = fn
	}
}

func (s fakeServer) CreateProject(
	ctx context.Context,
	req *tcloudservicev1.CreateProjectRequest,
) (*tcloudservicev1.CreateProjectResponse, error) {
	if s.createProjectFunc != nil {
		return s.createProjectFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.CreateProject(ctx, req)
}

type GetProjectFunc func(
	context.Context,
	*tcloudservicev1.GetProjectRequest,
) (*tcloudservicev1.GetProjectResponse, error)

// WithGetProject sets the fake server's GetProject function
// override.
func WithGetProject(fn GetProjectFunc) NewOption {
	return func(s *fakeServer) {
		s.getProjectFunc = fn
	}
}

// OnGetProject lets tests validate arguments passed to
// GetProject and set its response.
func OnGetProject(
	asserter func(*tcloudservicev1.GetProjectRequest),
	returner func() (*tcloudservicev1.GetProjectResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.GetProjectRequest,
		) (*tcloudservicev1.GetProjectResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.GetProjectResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.GetProjectResponse{}
			}
			return resp, err
		}
		s.getProjectFunc = fn
	}
}

// OnGetProjectReturn sets the response returned by GetProject.
func OnGetProjectReturn(
	resp *tcloudservicev1.GetProjectResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.GetProjectResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.GetProjectRequest,
		) (*tcloudservicev1.GetProjectResponse, error) {
			return resp, err
		}
		s.getProjectFunc = fn
	}
}

func (s fakeServer) GetProject(
	ctx context.Context,
	req *tcloudservicev1.GetProjectRequest,
) (*tcloudservicev1.GetProjectResponse, error) {
	if s.getProjectFunc != nil {
		return s.getProjectFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.GetProject(ctx, req)
}

type UpdateProjectFunc func(
	context.Context,
	*tcloudservicev1.UpdateProjectRequest,
) (*tcloudservicev1.UpdateProjectResponse, error)

// WithUpdateProject sets the fake server's UpdateProject
// function override.
func WithUpdateProject(fn UpdateProjectFunc) NewOption {
	return func(s *fakeServer) {
		s.updateProjectFunc = fn
	}
}

// OnUpdateProject lets tests validate arguments passed to
// UpdateProject and set its response.
func OnUpdateProject(
	asserter func(*tcloudservicev1.UpdateProjectRequest),
	returner func() (*tcloudservicev1.UpdateProjectResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.UpdateProjectRequest,
		) (*tcloudservicev1.UpdateProjectResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.UpdateProjectResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.UpdateProjectResponse{}
			}
			return resp, err
		}
		s.updateProjectFunc = fn
	}
}

// OnUpdateProjectReturn sets the response returned by
// UpdateProject.
func OnUpdateProjectReturn(
	resp *tcloudservicev1.UpdateProjectResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.UpdateProjectResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.UpdateProjectRequest,
		) (*tcloudservicev1.UpdateProjectResponse, error) {
			return resp, err
		}
		s.updateProjectFunc = fn
	}
}

func (s fakeServer) UpdateProject(
	ctx context.Context,
	req *tcloudservicev1.UpdateProjectRequest,
) (*tcloudservicev1.UpdateProjectResponse, error) {
	if s.updateProjectFunc != nil {
		return s.updateProjectFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.UpdateProject(ctx, req)
}

type DeleteProjectFunc func(
	context.Context,
	*tcloudservicev1.DeleteProjectRequest,
) (*tcloudservicev1.DeleteProjectResponse, error)

// WithDeleteProject sets the fake server's DeleteProject
// function override.
func WithDeleteProject(fn DeleteProjectFunc) NewOption {
	return func(s *fakeServer) {
		s.deleteProjectFunc = fn
	}
}

// OnDeleteProject lets tests validate arguments passed to
// DeleteProject and set its response.
func OnDeleteProject(
	asserter func(*tcloudservicev1.DeleteProjectRequest),
	returner func() (*tcloudservicev1.DeleteProjectResponse, error),
) NewOption {
	return func(s *fakeServer) {
		fn := func(
			ctx context.Context, req *tcloudservicev1.DeleteProjectRequest,
		) (*tcloudservicev1.DeleteProjectResponse, error) {
			if asserter != nil {
				asserter(req)
			}
			if returner == nil {
				return &tcloudservicev1.DeleteProjectResponse{}, nil
			}
			resp, err := returner()
			if resp == nil && err == nil {
				resp = &tcloudservicev1.DeleteProjectResponse{}
			}
			return resp, err
		}
		s.deleteProjectFunc = fn
	}
}

// OnDeleteProjectReturn sets the response returned by
// DeleteProject.
func OnDeleteProjectReturn(
	resp *tcloudservicev1.DeleteProjectResponse,
	err error,
) NewOption {
	if resp == nil && err == nil {
		resp = &tcloudservicev1.DeleteProjectResponse{}
	}
	return func(s *fakeServer) {
		fn := func(
			context.Context, *tcloudservicev1.DeleteProjectRequest,
		) (*tcloudservicev1.DeleteProjectResponse, error) {
			return resp, err
		}
		s.deleteProjectFunc = fn
	}
}

func (s fakeServer) DeleteProject(
	ctx context.Context,
	req *tcloudservicev1.DeleteProjectRequest,
) (*tcloudservicev1.DeleteProjectResponse, error) {
	if s.deleteProjectFunc != nil {
		return s.deleteProjectFunc(ctx, req)
	}
	return s.UnimplementedCloudServiceServer.DeleteProject(ctx, req)
}
