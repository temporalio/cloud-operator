package server

import (
	"net"

	"github.com/temporalio/cloud-operator/pkg/testutil"
	tcloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
	"google.golang.org/grpc"
)

type fakeServer struct {
	grpcServer *grpc.Server
	lis        net.Listener
	tcloudservicev1.UnimplementedCloudServiceServer

	getNamespaceFunc      GetNamespaceFunc
	getNamespacesFunc     GetNamespacesFunc
	updateNamespaceFunc   UpdateNamespaceFunc
	deleteNamespaceFunc   DeleteNamespaceFunc
	getAsyncOperationFunc GetAsyncOperationFunc
}

// HostPort returns the address of the TCP localhost port that the fake server
// is listening on.
func (s fakeServer) HostPort() string {
	return s.lis.Addr().String()
}

// Client returns a Temporal Cloud client.Client object set up to connect to
// the fake Temporal Cloud server.
func (s fakeServer) Client() (*tcclient.Client, error) {
	opts := testutil.NewCloudClientOptions()
	opts.AllowInsecure = true
	opts.HostPort = s.HostPort()
	opts.APIKey = testutil.FakeTemporalCloudAPIKeyContents
	return tcclient.New(*opts)
}

// Start starts the gRPC server in a new goroutine and returns a stopper
// function.
func (s *fakeServer) Start() func() {
	if s.grpcServer == nil {
		panic("called Start on nil fakeServer")
	}
	go s.grpcServer.Serve(s.lis)
	return s.grpcServer.Stop
}

type NewOption func(*fakeServer)

// New returns a fully functional gRPC server that handles requests to the
// Temporal Cloud API.
func New(opts ...NewOption) (*fakeServer, error) {
	s := fakeServer{}
	for _, opt := range opts {
		opt(&s)
	}

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	s.grpcServer = grpc.NewServer()
	s.lis = lis

	tcloudservicev1.RegisterCloudServiceServer(s.grpcServer, &s)

	return &s, nil
}
