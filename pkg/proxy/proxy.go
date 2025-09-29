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
	"cri-lite/pkg/version"
)

// Server is the gRPC server for the cri-lite proxy.
type Server struct {
	runtimeapi.UnimplementedRuntimeServiceServer

	runtimeapi.UnimplementedImageServiceServer

	runtimeClient runtimeapi.RuntimeServiceClient
	imageClient   runtimeapi.ImageServiceClient
	policy        policy.Policy
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

func (s *Server) RemoveImage(ctx context.Context, req *runtimeapi.RemoveImageRequest) (*runtimeapi.RemoveImageResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.imageClient.RemoveImage(ctx, req)
}

// ContainerStats implements v1.RuntimeServiceServer.
func (s *Server) ContainerStats(ctx context.Context, req *runtimeapi.ContainerStatsRequest) (*runtimeapi.ContainerStatsResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ContainerStats(ctx, req)
}

// CreateContainer implements v1.RuntimeServiceServer.
func (s *Server) CreateContainer(ctx context.Context, req *runtimeapi.CreateContainerRequest) (*runtimeapi.CreateContainerResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.CreateContainer(ctx, req)
}

// Exec implements v1.RuntimeServiceServer.
func (s *Server) Exec(ctx context.Context, req *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.Exec(ctx, req)
}

// ExecSync implements v1.RuntimeServiceServer.
func (s *Server) ExecSync(ctx context.Context, req *runtimeapi.ExecSyncRequest) (*runtimeapi.ExecSyncResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ExecSync(ctx, req)
}

var errGetContainerEventsNotImplemented = errors.New("GetContainerEvents not implemented")

// GetContainerEvents implements v1.RuntimeServiceServer.
func (s *Server) GetContainerEvents(req *runtimeapi.GetEventsRequest, stream grpc.ServerStreamingServer[runtimeapi.ContainerEventResponse]) error {
	return errGetContainerEventsNotImplemented
}

// ListContainerStats implements v1.RuntimeServiceServer.
func (s *Server) ListContainerStats(ctx context.Context, req *runtimeapi.ListContainerStatsRequest) (*runtimeapi.ListContainerStatsResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ListContainerStats(ctx, req)
}

// ListMetricDescriptors implements v1.RuntimeServiceServer.
func (s *Server) ListMetricDescriptors(ctx context.Context, req *runtimeapi.ListMetricDescriptorsRequest) (*runtimeapi.ListMetricDescriptorsResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ListMetricDescriptors(ctx, req)
}

// ListPodSandboxMetrics implements v1.RuntimeServiceServer.
func (s *Server) ListPodSandboxMetrics(ctx context.Context, req *runtimeapi.ListPodSandboxMetricsRequest) (*runtimeapi.ListPodSandboxMetricsResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ListPodSandboxMetrics(ctx, req)
}

// ListPodSandboxStats implements v1.RuntimeServiceServer.
func (s *Server) ListPodSandboxStats(ctx context.Context, req *runtimeapi.ListPodSandboxStatsRequest) (*runtimeapi.ListPodSandboxStatsResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ListPodSandboxStats(ctx, req)
}

// PodSandboxStats implements v1.RuntimeServiceServer.
func (s *Server) PodSandboxStats(ctx context.Context, req *runtimeapi.PodSandboxStatsRequest) (*runtimeapi.PodSandboxStatsResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.PodSandboxStats(ctx, req)
}

// PortForward implements v1.RuntimeServiceServer.
func (s *Server) PortForward(ctx context.Context, req *runtimeapi.PortForwardRequest) (*runtimeapi.PortForwardResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.PortForward(ctx, req)
}

// RemoveContainer implements v1.RuntimeServiceServer.
func (s *Server) RemoveContainer(ctx context.Context, req *runtimeapi.RemoveContainerRequest) (*runtimeapi.RemoveContainerResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.RemoveContainer(ctx, req)
}

// RemovePodSandbox implements v1.RuntimeServiceServer.
func (s *Server) RemovePodSandbox(ctx context.Context, req *runtimeapi.RemovePodSandboxRequest) (*runtimeapi.RemovePodSandboxResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.RemovePodSandbox(ctx, req)
}

// RuntimeConfig implements v1.RuntimeServiceServer.
func (s *Server) RuntimeConfig(ctx context.Context, req *runtimeapi.RuntimeConfigRequest) (*runtimeapi.RuntimeConfigResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.RuntimeConfig(ctx, req)
}

// StartContainer implements v1.RuntimeServiceServer.
func (s *Server) StartContainer(ctx context.Context, req *runtimeapi.StartContainerRequest) (*runtimeapi.StartContainerResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.StartContainer(ctx, req)
}

// Status implements v1.RuntimeServiceServer.
func (s *Server) Status(ctx context.Context, req *runtimeapi.StatusRequest) (*runtimeapi.StatusResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.Status(ctx, req)
}

// StopContainer implements v1.RuntimeServiceServer.
func (s *Server) StopContainer(ctx context.Context, req *runtimeapi.StopContainerRequest) (*runtimeapi.StopContainerResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.StopContainer(ctx, req)
}

// StopPodSandbox implements v1.RuntimeServiceServer.
func (s *Server) StopPodSandbox(ctx context.Context, req *runtimeapi.StopPodSandboxRequest) (*runtimeapi.StopPodSandboxResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.StopPodSandbox(ctx, req)
}

// UpdateContainerResources implements v1.RuntimeServiceServer.
func (s *Server) UpdateContainerResources(ctx context.Context, req *runtimeapi.UpdateContainerResourcesRequest) (*runtimeapi.UpdateContainerResourcesResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.UpdateContainerResources(ctx, req)
}

// UpdatePodSandboxResources implements v1.RuntimeServiceServer.
func (s *Server) UpdatePodSandboxResources(ctx context.Context, req *runtimeapi.UpdatePodSandboxResourcesRequest) (*runtimeapi.UpdatePodSandboxResourcesResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.UpdatePodSandboxResources(ctx, req)
}

// UpdateRuntimeConfig implements v1.RuntimeServiceServer.
func (s *Server) UpdateRuntimeConfig(ctx context.Context, req *runtimeapi.UpdateRuntimeConfigRequest) (*runtimeapi.UpdateRuntimeConfigResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.UpdateRuntimeConfig(ctx, req)
}

func (s *Server) GetRuntimeClient() runtimeapi.RuntimeServiceClient {
	return s.runtimeClient
}

// GetImageClient returns the underlying image service client.
func (s *Server) GetImageClient() runtimeapi.ImageServiceClient {
	return s.imageClient
}

// SetRuntimeClient sets the underlying runtime service client.
func (s *Server) SetRuntimeClient(client runtimeapi.RuntimeServiceClient) {
	s.runtimeClient = client
}

// SetImageClient sets the underlying image service client.
func (s *Server) SetImageClient(client runtimeapi.ImageServiceClient) {
	s.imageClient = client
}

// SetPolicy sets the policy enforced by the server.
func (s *Server) SetPolicy(p policy.Policy) {
	s.policy = p
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

	var interceptors grpc.UnaryServerInterceptor
	if s.policy != nil {
		interceptors = s.policy.UnaryInterceptor()
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(creds.NewPIDCreds()),
		grpc.UnaryInterceptor(interceptors),
	)

	runtimeapi.RegisterRuntimeServiceServer(grpcServer, s)
	runtimeapi.RegisterImageServiceServer(grpcServer, s)

	return fmt.Errorf("failed to serve grpc server: %w", grpcServer.Serve(lis))
}

// Version proxies the Version call to the underlying runtime service.
func (s *Server) Version(ctx context.Context, req *runtimeapi.VersionRequest) (*runtimeapi.VersionResponse, error) {
	resp, err := s.runtimeClient.Version(ctx, req)
	if err != nil {
		//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
		return nil, err
	}

	resp.RuntimeVersion = fmt.Sprintf("%s via cri-lite (%s)", resp.GetRuntimeVersion(), version.Version)
	resp.RuntimeName = fmt.Sprintf("%s with policy %s", resp.GetRuntimeName(), s.policyNames())

	return resp, nil
}

// ListContainers proxies the ListContainers call to the underlying runtime service.
func (s *Server) ListContainers(ctx context.Context, req *runtimeapi.ListContainersRequest) (*runtimeapi.ListContainersResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ListContainers(ctx, req)
}

// ContainerStatus proxies the ContainerStatus call to the underlying runtime service.
func (s *Server) ContainerStatus(ctx context.Context, req *runtimeapi.ContainerStatusRequest) (*runtimeapi.ContainerStatusResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ContainerStatus(ctx, req)
}

// ListPodSandbox proxies the ListPodSandbox call to the underlying runtime service.
func (s *Server) ListPodSandbox(ctx context.Context, req *runtimeapi.ListPodSandboxRequest) (*runtimeapi.ListPodSandboxResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ListPodSandbox(ctx, req)
}

// PodSandboxStatus proxies the PodSandboxStatus call to the underlying runtime service.
func (s *Server) PodSandboxStatus(ctx context.Context, req *runtimeapi.PodSandboxStatusRequest) (*runtimeapi.PodSandboxStatusResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.PodSandboxStatus(ctx, req)
}

// RunPodSandbox is the most dangerous CRI API call, allowing major escalation of privileges.
// It is explicitly disabled in cri-lite to prevent unprivileged users from creating new pod sandboxes.
// This method MUST NOT be re-enabled or proxied to the underlying runtime.
// Any attempts to modify this to proxy the call will be reverted.
func (s *Server) RunPodSandbox(ctx context.Context, req *runtimeapi.RunPodSandboxRequest) (*runtimeapi.RunPodSandboxResponse, error) {
	return nil, errors.New("RunPodSandbox is disabled by cri-lite for security reasons")
}

// ReopenContainerLog proxies the ReopenContainerLog call to the underlying runtime service.
func (s *Server) ReopenContainerLog(ctx context.Context, req *runtimeapi.ReopenContainerLogRequest) (*runtimeapi.ReopenContainerLogResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.ReopenContainerLog(ctx, req)
}

// Attach proxies the Attach call to the underlying runtime service.
func (s *Server) Attach(ctx context.Context, req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.runtimeClient.Attach(ctx, req)
}

// ListImages proxies the ListImages call to the underlying image service.
func (s *Server) ListImages(ctx context.Context, req *runtimeapi.ListImagesRequest) (*runtimeapi.ListImagesResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.imageClient.ListImages(ctx, req)
}

// ImageStatus proxies the ImageStatus call to the underlying image service.
func (s *Server) ImageStatus(ctx context.Context, req *runtimeapi.ImageStatusRequest) (*runtimeapi.ImageStatusResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.imageClient.ImageStatus(ctx, req)
}

// ImageFsInfo proxies the ImageFsInfo call to the underlying image service.
func (s *Server) ImageFsInfo(ctx context.Context, req *runtimeapi.ImageFsInfoRequest) (*runtimeapi.ImageFsInfoResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.imageClient.ImageFsInfo(ctx, req)
}

// PullImage proxies the PullImage call to the underlying image service.
func (s *Server) PullImage(ctx context.Context, req *runtimeapi.PullImageRequest) (*runtimeapi.PullImageResponse, error) {
	//nolint:wrapcheck // keep the original error to minimize the difference of proxied and non-proxied calls
	return s.imageClient.PullImage(ctx, req)
}

func (s *Server) policyNames() string {
	if s.policy == nil {
		return ""
	}

	return s.policy.Name()
}
