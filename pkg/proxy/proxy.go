// Package proxy provides a CRI proxy that enforces policies on CRI API calls.
package proxy

import (
	"context"
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"cri-lite/pkg/creds"
	"cri-lite/pkg/policy"
)

// Server is the gRPC server for the cri-lite proxy.
type Server struct {
	runtimeapi.UnimplementedRuntimeServiceServer

	runtimeapi.UnimplementedImageServiceServer

	runtimeClient runtimeapi.RuntimeServiceClient
	imageClient   runtimeapi.ImageServiceClient
	policies      []policy.Policy
}

// NewServer creates a new cri-lite proxy server.
func NewServer(runtimeEndpoint, imageEndpoint string) (*Server, error) {
	runtimeConn, err := grpc.NewClient(runtimeEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to runtime endpoint: %w", err)
	}

	imageConn, err := grpc.NewClient(imageEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to image endpoint: %w", err)
	}

	return &Server{
		UnimplementedRuntimeServiceServer: runtimeapi.UnimplementedRuntimeServiceServer{},
		UnimplementedImageServiceServer:   runtimeapi.UnimplementedImageServiceServer{},
		runtimeClient:                     runtimeapi.NewRuntimeServiceClient(runtimeConn),
		imageClient:                       runtimeapi.NewImageServiceClient(imageConn),
	}, nil
}

func (s *Server) GetRuntimeClient() runtimeapi.RuntimeServiceClient {
	return s.runtimeClient
}

// GetImageClient returns the underlying image service client.
func (s *Server) GetImageClient() runtimeapi.ImageServiceClient {
	return s.imageClient
}

// SetPolicies sets the list of policies enforced by the server.
func (s *Server) SetPolicies(policies []policy.Policy) {
	s.policies = policies
}


// Start starts the gRPC server on the specified socket.
func (s *Server) Start(socketPath string) error {
	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}

	interceptors := make([]grpc.UnaryServerInterceptor, 0, len(s.policies))
	for _, p := range s.policies {
		interceptors = append(interceptors, p.UnaryInterceptor())
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(creds.NewPIDCreds()),
		grpc.ChainUnaryInterceptor(interceptors...),
	)

	runtimeapi.RegisterRuntimeServiceServer(grpcServer, s)
	runtimeapi.RegisterImageServiceServer(grpcServer, s)

	return fmt.Errorf("failed to serve grpc server: %w", grpcServer.Serve(lis))
}

// Version proxies the Version call to the underlying runtime service.
func (s *Server) Version(ctx context.Context, req *runtimeapi.VersionRequest) (*runtimeapi.VersionResponse, error) {
	resp, err := s.runtimeClient.Version(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy Version call: %w", err)
	}

	return resp, nil
}

// ListContainers proxies the ListContainers call to the underlying runtime service.
func (s *Server) ListContainers(ctx context.Context, req *runtimeapi.ListContainersRequest) (*runtimeapi.ListContainersResponse, error) {
	resp, err := s.runtimeClient.ListContainers(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ListContainers call: %w", err)
	}

	return resp, nil
}

// ContainerStatus proxies the ContainerStatus call to the underlying runtime service.
func (s *Server) ContainerStatus(ctx context.Context, req *runtimeapi.ContainerStatusRequest) (*runtimeapi.ContainerStatusResponse, error) {
	resp, err := s.runtimeClient.ContainerStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ContainerStatus call: %w", err)
	}

	return resp, nil
}

// ListPodSandbox proxies the ListPodSandbox call to the underlying runtime service.
func (s *Server) ListPodSandbox(ctx context.Context, req *runtimeapi.ListPodSandboxRequest) (*runtimeapi.ListPodSandboxResponse, error) {
	resp, err := s.runtimeClient.ListPodSandbox(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ListPodSandbox call: %w", err)
	}

	return resp, nil
}

// PodSandboxStatus proxies the PodSandboxStatus call to the underlying runtime service.
func (s *Server) PodSandboxStatus(ctx context.Context, req *runtimeapi.PodSandboxStatusRequest) (*runtimeapi.PodSandboxStatusResponse, error) {
	resp, err := s.runtimeClient.PodSandboxStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy PodSandboxStatus call: %w", err)
	}

	return resp, nil
}

// ListImages proxies the ListImages call to the underlying image service.
func (s *Server) ListImages(ctx context.Context, req *runtimeapi.ListImagesRequest) (*runtimeapi.ListImagesResponse, error) {
	resp, err := s.imageClient.ListImages(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ListImages call: %w", err)
	}

	return resp, nil
}

// ImageStatus proxies the ImageStatus call to the underlying image service.
func (s *Server) ImageStatus(ctx context.Context, req *runtimeapi.ImageStatusRequest) (*runtimeapi.ImageStatusResponse, error) {
	resp, err := s.imageClient.ImageStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ImageStatus call: %w", err)
	}

	return resp, nil
}

// ImageFsInfo proxies the ImageFsInfo call to the underlying image service.
func (s *Server) ImageFsInfo(ctx context.Context, req *runtimeapi.ImageFsInfoRequest) (*runtimeapi.ImageFsInfoResponse, error) {
	resp, err := s.imageClient.ImageFsInfo(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ImageFsInfo call: %w", err)
	}

	return resp, nil
}

// PullImage proxies the PullImage call to the underlying image service.
func (s *Server) PullImage(ctx context.Context, req *runtimeapi.PullImageRequest) (*runtimeapi.PullImageResponse, error) {
	resp, err := s.imageClient.PullImage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy PullImage call: %w", err)
	}

	return resp, nil
}
