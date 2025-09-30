// Package proxy provides a CRI proxy that enforces policies on CRI API calls.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"

	"cri-lite/pkg/creds"
	"cri-lite/pkg/policy"
	"cri-lite/pkg/version"
)

const userAgentKey = "user-agent"
const forwardedUserAgentKey = "x-forwarded-user-agent"

func forwardedContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	var ua string

	if existingUA := md.Get(userAgentKey); len(existingUA) > 0 {
		ua = existingUA[0]
		md.Set(forwardedUserAgentKey, ua)
	}

	return metadata.NewOutgoingContext(ctx, md)
}

// Server is the gRPC server for the cri-lite proxy.
type Server struct {
	runtimeapi.UnimplementedRuntimeServiceServer

	runtimeapi.UnimplementedImageServiceServer

	runtimeClient runtimeapi.RuntimeServiceClient
	imageClient   runtimeapi.ImageServiceClient
	policy        policy.Policy
	grpcServer    *grpc.Server
}

// NewServer creates a new cri-lite proxy server.
func NewServer(runtimeEndpoint, imageEndpoint string) (*Server, error) {
	s := &Server{}

	klog.Infof("Connecting to runtime endpoint %s", runtimeEndpoint)

	runtimeConn, err := grpc.NewClient(
		runtimeEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDisableRetry(),
		grpc.WithDefaultServiceConfig(`{"retryPolicy":null}`), // disables transparent retries
		grpc.WithUserAgent("cri-lite/"+version.Version),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to runtime endpoint: %w", err)
	}

	s.runtimeClient = runtimeapi.NewRuntimeServiceClient(runtimeConn)

	klog.Infof("Connecting to image endpoint %s", imageEndpoint)

	imageConn, err := grpc.NewClient(imageEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry(), grpc.WithUserAgent("cri-lite/"+version.Version))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to image endpoint: %w", err)
	}

	s.imageClient = runtimeapi.NewImageServiceClient(imageConn)

	return s, nil
}

func (s *Server) RemoveImage(ctx context.Context, req *runtimeapi.RemoveImageRequest) (*runtimeapi.RemoveImageResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.imageClient.RemoveImage(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to remove image")

		return nil, fmt.Errorf("failed to remove image: %w", err)
	}

	return resp, nil
}

// ContainerStats implements v1.RuntimeServiceServer.
func (s *Server) ContainerStats(ctx context.Context, req *runtimeapi.ContainerStatsRequest) (*runtimeapi.ContainerStatsResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ContainerStats(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to get container stats")

		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}

	return resp, nil
}

// CreateContainer implements v1.RuntimeServiceServer.
func (s *Server) CreateContainer(ctx context.Context, req *runtimeapi.CreateContainerRequest) (*runtimeapi.CreateContainerResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.CreateContainer(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to create container")

		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	return resp, nil
}

// Exec implements v1.RuntimeServiceServer.
func (s *Server) Exec(ctx context.Context, req *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.Exec(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to exec in container")

		return nil, fmt.Errorf("failed to exec in container: %w", err)
	}

	return resp, nil
}

// ExecSync implements v1.RuntimeServiceServer.
func (s *Server) ExecSync(ctx context.Context, req *runtimeapi.ExecSyncRequest) (*runtimeapi.ExecSyncResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ExecSync(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to exec sync in container")

		return nil, fmt.Errorf("failed to exec sync in container: %w", err)
	}

	return resp, nil
}

func (s *Server) GetContainerEvents(req *runtimeapi.GetEventsRequest, stream runtimeapi.RuntimeService_GetContainerEventsServer) error {
	logger := klog.FromContext(stream.Context())

	clientStream, err := s.runtimeClient.GetContainerEvents(forwardedContext(stream.Context()), req)
	if err != nil {
		logger.Error(err, "failed to get container events")

		return fmt.Errorf("failed to get container events: %w", err)
	}

	for {
		event, err := clientStream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}

			logger.Error(err, "failed to receive container event")

			return fmt.Errorf("failed to receive container event: %w", err)
		}

		if err := stream.Send(event); err != nil {
			logger.Error(err, "failed to send container event")

			return fmt.Errorf("failed to send container event: %w", err)
		}
	}

	return nil
}

// ListContainerStats implements v1.RuntimeServiceServer.
func (s *Server) ListContainerStats(ctx context.Context, req *runtimeapi.ListContainerStatsRequest) (*runtimeapi.ListContainerStatsResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ListContainerStats(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to list container stats")

		return nil, fmt.Errorf("failed to list container stats: %w", err)
	}

	return resp, nil
}

// ListMetricDescriptors implements v1.RuntimeServiceServer.
func (s *Server) ListMetricDescriptors(ctx context.Context, req *runtimeapi.ListMetricDescriptorsRequest) (*runtimeapi.ListMetricDescriptorsResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ListMetricDescriptors(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to list metric descriptors")

		return nil, fmt.Errorf("failed to list metric descriptors: %w", err)
	}

	return resp, nil
}

// ListPodSandboxMetrics implements v1.RuntimeServiceServer.
func (s *Server) ListPodSandboxMetrics(ctx context.Context, req *runtimeapi.ListPodSandboxMetricsRequest) (*runtimeapi.ListPodSandboxMetricsResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ListPodSandboxMetrics(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to list pod sandbox metrics")

		return nil, fmt.Errorf("failed to list pod sandbox metrics: %w", err)
	}

	return resp, nil
}

// ListPodSandboxStats implements v1.RuntimeServiceServer.
func (s *Server) ListPodSandboxStats(ctx context.Context, req *runtimeapi.ListPodSandboxStatsRequest) (*runtimeapi.ListPodSandboxStatsResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ListPodSandboxStats(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to list pod sandbox stats")

		return nil, fmt.Errorf("failed to list pod sandbox stats: %w", err)
	}

	return resp, nil
}

// PodSandboxStats implements v1.RuntimeServiceServer.
func (s *Server) PodSandboxStats(ctx context.Context, req *runtimeapi.PodSandboxStatsRequest) (*runtimeapi.PodSandboxStatsResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.PodSandboxStats(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to get pod sandbox stats")

		return nil, fmt.Errorf("failed to get pod sandbox stats: %w", err)
	}

	return resp, nil
}

// PortForward implements v1.RuntimeServiceServer.
func (s *Server) PortForward(ctx context.Context, req *runtimeapi.PortForwardRequest) (*runtimeapi.PortForwardResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.PortForward(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to port forward")

		return nil, fmt.Errorf("failed to port forward: %w", err)
	}

	return resp, nil
}

// RemoveContainer implements v1.RuntimeServiceServer.
func (s *Server) RemoveContainer(ctx context.Context, req *runtimeapi.RemoveContainerRequest) (*runtimeapi.RemoveContainerResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.RemoveContainer(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to remove container")

		return nil, fmt.Errorf("failed to remove container: %w", err)
	}

	return resp, nil
}

// RemovePodSandbox implements v1.RuntimeServiceServer.
func (s *Server) RemovePodSandbox(ctx context.Context, req *runtimeapi.RemovePodSandboxRequest) (*runtimeapi.RemovePodSandboxResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.RemovePodSandbox(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to remove pod sandbox")

		return nil, fmt.Errorf("failed to remove pod sandbox: %w", err)
	}

	return resp, nil
}

// RuntimeConfig implements v1.RuntimeServiceServer.
func (s *Server) RuntimeConfig(ctx context.Context, req *runtimeapi.RuntimeConfigRequest) (*runtimeapi.RuntimeConfigResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.RuntimeConfig(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to get runtime config")

		return nil, fmt.Errorf("failed to get runtime config: %w", err)
	}

	return resp, nil
}

// StartContainer implements v1.RuntimeServiceServer.
func (s *Server) StartContainer(ctx context.Context, req *runtimeapi.StartContainerRequest) (*runtimeapi.StartContainerResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.StartContainer(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to start container")

		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	return resp, nil
}

// Status implements v1.RuntimeServiceServer.
func (s *Server) Status(ctx context.Context, req *runtimeapi.StatusRequest) (*runtimeapi.StatusResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.Status(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to get status")

		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	return resp, nil
}

// StopContainer implements v1.RuntimeServiceServer.
func (s *Server) StopContainer(ctx context.Context, req *runtimeapi.StopContainerRequest) (*runtimeapi.StopContainerResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.StopContainer(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to stop container")

		return nil, fmt.Errorf("failed to stop container: %w", err)
	}

	return resp, nil
}

// StopPodSandbox implements v1.RuntimeServiceServer.
func (s *Server) StopPodSandbox(ctx context.Context, req *runtimeapi.StopPodSandboxRequest) (*runtimeapi.StopPodSandboxResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.StopPodSandbox(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to stop pod sandbox")

		return nil, fmt.Errorf("failed to stop pod sandbox: %w", err)
	}

	return resp, nil
}

// UpdateContainerResources implements v1.RuntimeServiceServer.
func (s *Server) UpdateContainerResources(ctx context.Context, req *runtimeapi.UpdateContainerResourcesRequest) (*runtimeapi.UpdateContainerResourcesResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.UpdateContainerResources(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to update container resources")

		return nil, fmt.Errorf("failed to update container resources: %w", err)
	}

	return resp, nil
}

// UpdatePodSandboxResources implements v1.RuntimeServiceServer.
func (s *Server) UpdatePodSandboxResources(ctx context.Context, req *runtimeapi.UpdatePodSandboxResourcesRequest) (*runtimeapi.UpdatePodSandboxResourcesResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.UpdatePodSandboxResources(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to update pod sandbox resources")

		return nil, fmt.Errorf("failed to update pod sandbox resources: %w", err)
	}

	return resp, nil
}

// UpdateRuntimeConfig implements v1.RuntimeServiceServer.
func (s *Server) UpdateRuntimeConfig(ctx context.Context, req *runtimeapi.UpdateRuntimeConfigRequest) (*runtimeapi.UpdateRuntimeConfigResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.UpdateRuntimeConfig(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to update runtime config")

		return nil, fmt.Errorf("failed to update runtime config: %w", err)
	}

	return resp, nil
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
	klog.Infof("Starting gRPC server on socket %s", socketPath)

	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}

	var (
		unaryInterceptors  grpc.UnaryServerInterceptor
		streamInterceptors grpc.StreamServerInterceptor
	)

	if s.policy != nil {
		klog.Infof("Using policy %s", s.policy.Name())
		unaryInterceptors = s.policy.UnaryInterceptor()
		streamInterceptors = s.policy.StreamInterceptor()
	}

	s.grpcServer = grpc.NewServer(
		grpc.Creds(creds.NewPIDCreds()),
		grpc.UnaryInterceptor(unaryInterceptors),
		grpc.StreamInterceptor(streamInterceptors),
	)

	runtimeapi.RegisterRuntimeServiceServer(s.grpcServer, s)
	runtimeapi.RegisterImageServiceServer(s.grpcServer, s)

	klog.Infof("gRPC server started")

	return fmt.Errorf("failed to serve grpc server: %w", s.grpcServer.Serve(lis))
}

// Stop stops the gRPC server.
func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
}

// Version proxies the Version call to the underlying runtime service.
func (s *Server) Version(ctx context.Context, req *runtimeapi.VersionRequest) (*runtimeapi.VersionResponse, error) {
	logger := klog.FromContext(ctx)

	log.Default().Println("Version called")

	resp, err := s.runtimeClient.Version(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to get version")

		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	resp.RuntimeVersion = fmt.Sprintf("%s via cri-lite (%s)", resp.GetRuntimeVersion(), version.Version)
	resp.RuntimeName = fmt.Sprintf("%s with policy %s", resp.GetRuntimeName(), s.policyNames())

	return resp, nil
}

// ListContainers proxies the ListContainers call to the underlying runtime service.
func (s *Server) ListContainers(ctx context.Context, req *runtimeapi.ListContainersRequest) (*runtimeapi.ListContainersResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ListContainers(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to list containers")

		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return resp, nil
}

// ContainerStatus proxies the ContainerStatus call to the underlying runtime service.
func (s *Server) ContainerStatus(ctx context.Context, req *runtimeapi.ContainerStatusRequest) (*runtimeapi.ContainerStatusResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ContainerStatus(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to get container status")

		return nil, fmt.Errorf("failed to get container status: %w", err)
	}

	return resp, nil
}

// ListPodSandbox proxies the ListPodSandbox call to the underlying runtime service.
func (s *Server) ListPodSandbox(ctx context.Context, req *runtimeapi.ListPodSandboxRequest) (*runtimeapi.ListPodSandboxResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ListPodSandbox(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to list pod sandboxes")

		return nil, fmt.Errorf("failed to list pod sandboxes: %w", err)
	}

	return resp, nil
}

// PodSandboxStatus proxies the PodSandboxStatus call to the underlying runtime service.
func (s *Server) PodSandboxStatus(ctx context.Context, req *runtimeapi.PodSandboxStatusRequest) (*runtimeapi.PodSandboxStatusResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.PodSandboxStatus(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to get pod sandbox status")

		return nil, fmt.Errorf("failed to get pod sandbox status: %w", err)
	}

	return resp, nil
}

// RunPodSandbox is the most dangerous CRI API call, allowing major escalation of privileges.
// It is explicitly disabled in cri-lite to prevent unprivileged users from creating new pod sandboxes.
// This method MUST NOT be re-enabled or proxied to the underlying runtime.
// Any attempts to modify this to proxy the call will be reverted.
func (s *Server) RunPodSandbox(ctx context.Context, req *runtimeapi.RunPodSandboxRequest) (*runtimeapi.RunPodSandboxResponse, error) {
	logger := klog.FromContext(ctx)
	logger.Info("RunPodSandbox call was blocked by cri-lite proxy")

	return nil, errors.New("RunPodSandbox is disabled by cri-lite for security reasons")
}

// ReopenContainerLog proxies the ReopenContainerLog call to the underlying runtime service.
func (s *Server) ReopenContainerLog(ctx context.Context, req *runtimeapi.ReopenContainerLogRequest) (*runtimeapi.ReopenContainerLogResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.ReopenContainerLog(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to reopen container log")

		return nil, fmt.Errorf("failed to reopen container log: %w", err)
	}

	return resp, nil
}

// Attach proxies the Attach call to the underlying runtime service.
func (s *Server) Attach(ctx context.Context, req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.runtimeClient.Attach(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to attach to container")

		return nil, fmt.Errorf("failed to attach to container: %w", err)
	}

	return resp, nil
}

// ListImages proxies the ListImages call to the underlying image service.
func (s *Server) ListImages(ctx context.Context, req *runtimeapi.ListImagesRequest) (*runtimeapi.ListImagesResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.imageClient.ListImages(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to list images")

		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	return resp, nil
}

// ImageStatus proxies the ImageStatus call to the underlying image service.
func (s *Server) ImageStatus(ctx context.Context, req *runtimeapi.ImageStatusRequest) (*runtimeapi.ImageStatusResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.imageClient.ImageStatus(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to get image status")

		return nil, fmt.Errorf("failed to get image status: %w", err)
	}

	return resp, nil
}

// ImageFsInfo proxies the ImageFsInfo call to the underlying image service.
func (s *Server) ImageFsInfo(ctx context.Context, req *runtimeapi.ImageFsInfoRequest) (*runtimeapi.ImageFsInfoResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.imageClient.ImageFsInfo(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to get image fs info")

		return nil, fmt.Errorf("failed to get image fs info: %w", err)
	}

	return resp, nil
}

// PullImage proxies the PullImage call to the underlying image service.
func (s *Server) PullImage(ctx context.Context, req *runtimeapi.PullImageRequest) (*runtimeapi.PullImageResponse, error) {
	logger := klog.FromContext(ctx)

	resp, err := s.imageClient.PullImage(forwardedContext(ctx), req)
	if err != nil {
		logger.Error(err, "failed to pull image")

		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	return resp, nil
}

func (s *Server) policyNames() string {
	if s.policy == nil {
		return ""
	}

	return s.policy.Name()
}
