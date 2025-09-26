// Package proxy provides a CRI proxy that enforces policies on CRI API calls.
package proxy

import (
	"context"
	"errors"
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

// CheckpointContainer implements v1.RuntimeServiceServer.
func (s *Server) CheckpointContainer(ctx context.Context, req *runtimeapi.CheckpointContainerRequest) (*runtimeapi.CheckpointContainerResponse, error) {
	resp, err := s.runtimeClient.CheckpointContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy CheckpointContainer call: %w", err)
	}

	return resp, nil
}

// ContainerStats implements v1.RuntimeServiceServer.
func (s *Server) ContainerStats(ctx context.Context, req *runtimeapi.ContainerStatsRequest) (*runtimeapi.ContainerStatsResponse, error) {
	resp, err := s.runtimeClient.ContainerStats(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ContainerStats call: %w", err)
	}

	return resp, nil
}

// CreateContainer implements v1.RuntimeServiceServer.
func (s *Server) CreateContainer(ctx context.Context, req *runtimeapi.CreateContainerRequest) (*runtimeapi.CreateContainerResponse, error) {
	resp, err := s.runtimeClient.CreateContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy CreateContainer call: %w", err)
	}

	return resp, nil
}

// Exec implements v1.RuntimeServiceServer.
func (s *Server) Exec(ctx context.Context, req *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	resp, err := s.runtimeClient.Exec(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy Exec call: %w", err)
	}

	return resp, nil
}

// ExecSync implements v1.RuntimeServiceServer.
func (s *Server) ExecSync(ctx context.Context, req *runtimeapi.ExecSyncRequest) (*runtimeapi.ExecSyncResponse, error) {
	resp, err := s.runtimeClient.ExecSync(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ExecSync call: %w", err)
	}

	return resp, nil
}

var errGetContainerEventsNotImplemented = errors.New("GetContainerEvents not implemented")

// GetContainerEvents implements v1.RuntimeServiceServer.
func (s *Server) GetContainerEvents(req *runtimeapi.GetEventsRequest, stream grpc.ServerStreamingServer[runtimeapi.ContainerEventResponse]) error {
	return errGetContainerEventsNotImplemented
}

// ListContainerStats implements v1.RuntimeServiceServer.
func (s *Server) ListContainerStats(ctx context.Context, req *runtimeapi.ListContainerStatsRequest) (*runtimeapi.ListContainerStatsResponse, error) {
	resp, err := s.runtimeClient.ListContainerStats(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ListContainerStats call: %w", err)
	}

	return resp, nil
}

// ListMetricDescriptors implements v1.RuntimeServiceServer.
func (s *Server) ListMetricDescriptors(ctx context.Context, req *runtimeapi.ListMetricDescriptorsRequest) (*runtimeapi.ListMetricDescriptorsResponse, error) {
	resp, err := s.runtimeClient.ListMetricDescriptors(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ListMetricDescriptors call: %w", err)
	}

	return resp, nil
}

// ListPodSandboxMetrics implements v1.RuntimeServiceServer.
func (s *Server) ListPodSandboxMetrics(ctx context.Context, req *runtimeapi.ListPodSandboxMetricsRequest) (*runtimeapi.ListPodSandboxMetricsResponse, error) {
	resp, err := s.runtimeClient.ListPodSandboxMetrics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ListPodSandboxMetrics call: %w", err)
	}

	return resp, nil
}

// ListPodSandboxStats implements v1.RuntimeServiceServer.
func (s *Server) ListPodSandboxStats(ctx context.Context, req *runtimeapi.ListPodSandboxStatsRequest) (*runtimeapi.ListPodSandboxStatsResponse, error) {
	resp, err := s.runtimeClient.ListPodSandboxStats(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ListPodSandboxStats call: %w", err)
	}

	return resp, nil
}

// PodSandboxStats implements v1.RuntimeServiceServer.
func (s *Server) PodSandboxStats(ctx context.Context, req *runtimeapi.PodSandboxStatsRequest) (*runtimeapi.PodSandboxStatsResponse, error) {
	resp, err := s.runtimeClient.PodSandboxStats(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy PodSandboxStats call: %w", err)
	}

	return resp, nil
}

// PortForward implements v1.RuntimeServiceServer.
func (s *Server) PortForward(ctx context.Context, req *runtimeapi.PortForwardRequest) (*runtimeapi.PortForwardResponse, error) {
	resp, err := s.runtimeClient.PortForward(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy PortForward call: %w", err)
	}

	return resp, nil
}

// RemoveContainer implements v1.RuntimeServiceServer.
func (s *Server) RemoveContainer(ctx context.Context, req *runtimeapi.RemoveContainerRequest) (*runtimeapi.RemoveContainerResponse, error) {
	resp, err := s.runtimeClient.RemoveContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy RemoveContainer call: %w", err)
	}

	return resp, nil
}

// RemovePodSandbox implements v1.RuntimeServiceServer.
func (s *Server) RemovePodSandbox(ctx context.Context, req *runtimeapi.RemovePodSandboxRequest) (*runtimeapi.RemovePodSandboxResponse, error) {
	resp, err := s.runtimeClient.RemovePodSandbox(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy RemovePodSandbox call: %w", err)
	}

	return resp, nil
}

// RuntimeConfig implements v1.RuntimeServiceServer.
func (s *Server) RuntimeConfig(ctx context.Context, req *runtimeapi.RuntimeConfigRequest) (*runtimeapi.RuntimeConfigResponse, error) {
	resp, err := s.runtimeClient.RuntimeConfig(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy RuntimeConfig call: %w", err)
	}

	return resp, nil
}

// StartContainer implements v1.RuntimeServiceServer.
func (s *Server) StartContainer(ctx context.Context, req *runtimeapi.StartContainerRequest) (*runtimeapi.StartContainerResponse, error) {
	resp, err := s.runtimeClient.StartContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy StartContainer call: %w", err)
	}

	return resp, nil
}

// Status implements v1.RuntimeServiceServer.
func (s *Server) Status(ctx context.Context, req *runtimeapi.StatusRequest) (*runtimeapi.StatusResponse, error) {
	resp, err := s.runtimeClient.Status(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy Status call: %w", err)
	}

	return resp, nil
}

// StopContainer implements v1.RuntimeServiceServer.
func (s *Server) StopContainer(ctx context.Context, req *runtimeapi.StopContainerRequest) (*runtimeapi.StopContainerResponse, error) {
	resp, err := s.runtimeClient.StopContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy StopContainer call: %w", err)
	}

	return resp, nil
}

// StopPodSandbox implements v1.RuntimeServiceServer.
func (s *Server) StopPodSandbox(ctx context.Context, req *runtimeapi.StopPodSandboxRequest) (*runtimeapi.StopPodSandboxResponse, error) {
	resp, err := s.runtimeClient.StopPodSandbox(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy StopPodSandbox call: %w", err)
	}

	return resp, nil
}

// UpdateContainerResources implements v1.RuntimeServiceServer.
func (s *Server) UpdateContainerResources(ctx context.Context, req *runtimeapi.UpdateContainerResourcesRequest) (*runtimeapi.UpdateContainerResourcesResponse, error) {
	resp, err := s.runtimeClient.UpdateContainerResources(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy UpdateContainerResources call: %w", err)
	}

	return resp, nil
}

// UpdatePodSandboxResources implements v1.RuntimeServiceServer.
func (s *Server) UpdatePodSandboxResources(ctx context.Context, req *runtimeapi.UpdatePodSandboxResourcesRequest) (*runtimeapi.UpdatePodSandboxResourcesResponse, error) {
	resp, err := s.runtimeClient.UpdatePodSandboxResources(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy UpdatePodSandboxResources call: %w", err)
	}

	return resp, nil
}

// UpdateRuntimeConfig implements v1.RuntimeServiceServer.
func (s *Server) UpdateRuntimeConfig(ctx context.Context, req *runtimeapi.UpdateRuntimeConfigRequest) (*runtimeapi.UpdateRuntimeConfigResponse, error) {
	resp, err := s.runtimeClient.UpdateRuntimeConfig(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy UpdateRuntimeConfig call: %w", err)
	}

	return resp, nil
}

// NewServer creates a new cri-lite proxy server.
func NewServer(runtimeEndpoint, imageEndpoint string) (*Server, error) {
	s := &Server{}

	runtimeConn, err := grpc.NewClient(runtimeEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to runtime endpoint: %w", err)
	}

	s.runtimeClient = runtimeapi.NewRuntimeServiceClient(runtimeConn)

	imageConn, err := grpc.NewClient(imageEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to image endpoint: %w", err)
	}

	s.imageClient = runtimeapi.NewImageServiceClient(imageConn)

	return s, nil
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

// RunPodSandbox proxies the RunPodSandbox call to the underlying runtime service.
func (s *Server) RunPodSandbox(ctx context.Context, req *runtimeapi.RunPodSandboxRequest) (*runtimeapi.RunPodSandboxResponse, error) {
	resp, err := s.runtimeClient.RunPodSandbox(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy RunPodSandbox call: %w", err)
	}

	return resp, nil
}

// ReopenContainerLog proxies the ReopenContainerLog call to the underlying runtime service.
func (s *Server) ReopenContainerLog(ctx context.Context, req *runtimeapi.ReopenContainerLogRequest) (*runtimeapi.ReopenContainerLogResponse, error) {
	resp, err := s.runtimeClient.ReopenContainerLog(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy ReopenContainerLog call: %w", err)
	}

	return resp, nil
}

// Attach proxies the Attach call to the underlying runtime service.
func (s *Server) Attach(ctx context.Context, req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error) {
	resp, err := s.runtimeClient.Attach(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy Attach call: %w", err)
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

// RemoveImage proxies the RemoveImage call to the underlying image service.
func (s *Server) RemoveImage(ctx context.Context, req *runtimeapi.RemoveImageRequest) (*runtimeapi.RemoveImageResponse, error) {
	resp, err := s.imageClient.RemoveImage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to proxy RemoveImage call: %w", err)
	}

	return resp, nil
}
