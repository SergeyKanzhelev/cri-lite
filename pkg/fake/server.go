// Package fake provides a fake CRI server for testing purposes.
package fake

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// Server is a fake CRI server for testing.
type Server struct {
	runtimeapi.RuntimeServiceServer
	runtimeapi.ImageServiceServer

	containers      []*runtimeapi.Container
	stats           []*runtimeapi.ContainerStats
	podSandboxStats []*runtimeapi.PodSandboxStats
}

// NewServer creates a new fake CRI server.
func NewServer(socketPath string) (server *grpc.Server, listener net.Listener, fakeServer *Server, err error) {
	lc := net.ListenConfig{}

	lis, err := lc.Listen(context.Background(), "unix", socketPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to listen on socket: %w", err)
	}

	s := &Server{
		containers: []*runtimeapi.Container{
			{
				Id:           "test-container-id",
				PodSandboxId: "test-sandbox-id",
				Metadata: &runtimeapi.ContainerMetadata{
					Name:    "test-container",
					Attempt: 1,
				},
				Image: &runtimeapi.ImageSpec{
					Image: "test-image",
				},
				State: runtimeapi.ContainerState_CONTAINER_RUNNING,
			},
		},
	}
	grpcServer := grpc.NewServer()
	runtimeapi.RegisterRuntimeServiceServer(grpcServer, s)
	runtimeapi.RegisterImageServiceServer(grpcServer, s)

	return grpcServer, lis, s, nil
}

// SetContainers sets the list of containers for the fake server.
func (s *Server) SetContainers(containers []*runtimeapi.Container) {
	s.containers = containers
}

// SetContainerStats sets the list of container stats for the fake server.
func (s *Server) SetContainerStats(stats []*runtimeapi.ContainerStats) {
	s.stats = stats
}

// SetPodSandboxStats sets the list of pod sandbox stats for the fake server.
func (s *Server) SetPodSandboxStats(stats []*runtimeapi.PodSandboxStats) {
	s.podSandboxStats = stats
}

// Version returns a fake version.
func (s *Server) Version(_ context.Context, _ *runtimeapi.VersionRequest) (*runtimeapi.VersionResponse, error) {
	return &runtimeapi.VersionResponse{
		Version:           "1.0.0",
		RuntimeName:       "fake-runtime",
		RuntimeVersion:    "1.0.0",
		RuntimeApiVersion: "v1",
	}, nil
}

// ListContainers returns a fake list of containers.
func (s *Server) ListContainers(_ context.Context, req *runtimeapi.ListContainersRequest) (*runtimeapi.ListContainersResponse, error) {
	if req.GetFilter() == nil {
		return &runtimeapi.ListContainersResponse{
			Containers: s.containers,
		}, nil
	}

	filtered := make([]*runtimeapi.Container, 0, len(s.containers))

	for _, c := range s.containers {
		if req.GetFilter().GetId() != "" && c.GetId() != req.GetFilter().GetId() {
			continue
		}

		if req.GetFilter().GetPodSandboxId() != "" && c.GetPodSandboxId() != req.GetFilter().GetPodSandboxId() {
			continue
		}

		filtered = append(filtered, c)
	}

	return &runtimeapi.ListContainersResponse{
		Containers: filtered,
	}, nil
}

// ContainerStatus returns a fake container status.
func (s *Server) ContainerStatus(_ context.Context, req *runtimeapi.ContainerStatusRequest) (*runtimeapi.ContainerStatusResponse, error) {
	return &runtimeapi.ContainerStatusResponse{
		Status: &runtimeapi.ContainerStatus{
			Id: req.GetContainerId(),
			Metadata: &runtimeapi.ContainerMetadata{
				Name:    "test-container",
				Attempt: 1,
			},
			Image: &runtimeapi.ImageSpec{
				Image: "test-image",
			},
			State: runtimeapi.ContainerState_CONTAINER_RUNNING,
		},
	}, nil
}

// RunPodSandbox is a fake implementation.
func (s *Server) RunPodSandbox(_ context.Context, _ *runtimeapi.RunPodSandboxRequest) (*runtimeapi.RunPodSandboxResponse, error) {
	return &runtimeapi.RunPodSandboxResponse{
		PodSandboxId: "test-sandbox-id",
	}, nil
}

// ListImages returns a fake list of images.
func (s *Server) ListImages(_ context.Context, _ *runtimeapi.ListImagesRequest) (*runtimeapi.ListImagesResponse, error) {
	return &runtimeapi.ListImagesResponse{
		Images: []*runtimeapi.Image{
			{
				Id:       "sha256:12345",
				RepoTags: []string{"fake-image:latest"},
			},
		},
	}, nil
}

// ImageFsInfo returns fake image filesystem information.
func (s *Server) ImageFsInfo(_ context.Context, _ *runtimeapi.ImageFsInfoRequest) (*runtimeapi.ImageFsInfoResponse, error) {
	return &runtimeapi.ImageFsInfoResponse{}, nil
}

// PullImage is a fake implementation.
func (s *Server) PullImage(_ context.Context, _ *runtimeapi.PullImageRequest) (*runtimeapi.PullImageResponse, error) {
	return &runtimeapi.PullImageResponse{
		ImageRef: "sha256:12345",
	}, nil
}

// Status returns a fake status.
func (s *Server) Status(_ context.Context, _ *runtimeapi.StatusRequest) (*runtimeapi.StatusResponse, error) {
	return &runtimeapi.StatusResponse{
		Status: &runtimeapi.RuntimeStatus{
			Conditions: []*runtimeapi.RuntimeCondition{
				{
					Type:   "RuntimeReady",
					Status: true,
				},
			},
		},
	}, nil
}

// ContainerStats returns fake container stats.
func (s *Server) ContainerStats(_ context.Context, _ *runtimeapi.ContainerStatsRequest) (*runtimeapi.ContainerStatsResponse, error) {
	return &runtimeapi.ContainerStatsResponse{}, nil
}

// ListContainerStats returns fake container stats.
func (s *Server) ListContainerStats(_ context.Context, req *runtimeapi.ListContainerStatsRequest) (*runtimeapi.ListContainerStatsResponse, error) {
	if req.GetFilter() == nil {
		return &runtimeapi.ListContainerStatsResponse{
			Stats: s.stats,
		}, nil
	}

	filtered := make([]*runtimeapi.ContainerStats, 0, len(s.stats))

	for _, c := range s.stats {
		if req.GetFilter().GetPodSandboxId() != "" && c.GetAttributes().GetMetadata().GetName() != "container-1" {
			continue
		}

		filtered = append(filtered, c)
	}

	return &runtimeapi.ListContainerStatsResponse{
		Stats: filtered,
	}, nil
}

// PodSandboxStats returns fake pod sandbox stats.
func (s *Server) PodSandboxStats(_ context.Context, _ *runtimeapi.PodSandboxStatsRequest) (*runtimeapi.PodSandboxStatsResponse, error) {
	return &runtimeapi.PodSandboxStatsResponse{}, nil
}

// ListPodSandboxStats returns fake pod sandbox stats.
func (s *Server) ListPodSandboxStats(_ context.Context, req *runtimeapi.ListPodSandboxStatsRequest) (*runtimeapi.ListPodSandboxStatsResponse, error) {
	if req.GetFilter() == nil {
		return &runtimeapi.ListPodSandboxStatsResponse{
			Stats: s.podSandboxStats,
		}, nil
	}

	filtered := make([]*runtimeapi.PodSandboxStats, 0, len(s.podSandboxStats))

	for _, c := range s.podSandboxStats {
		if req.GetFilter().GetId() != "" && c.GetAttributes().GetId() != req.GetFilter().GetId() {
			continue
		}

		filtered = append(filtered, c)
	}

	return &runtimeapi.ListPodSandboxStatsResponse{
		Stats: filtered,
	}, nil
}

// ListPodSandbox returns a fake list of pod sandboxes.
func (s *Server) ListPodSandbox(_ context.Context, _ *runtimeapi.ListPodSandboxRequest) (*runtimeapi.ListPodSandboxResponse, error) {
	return &runtimeapi.ListPodSandboxResponse{}, nil
}

// PodSandboxStatus returns a fake pod sandbox status.
func (s *Server) PodSandboxStatus(_ context.Context, _ *runtimeapi.PodSandboxStatusRequest) (*runtimeapi.PodSandboxStatusResponse, error) {
	return &runtimeapi.PodSandboxStatusResponse{}, nil
}

// ImageStatus returns a fake image status.
func (s *Server) ImageStatus(_ context.Context, _ *runtimeapi.ImageStatusRequest) (*runtimeapi.ImageStatusResponse, error) {
	return &runtimeapi.ImageStatusResponse{}, nil
}

// PortForward is a fake implementation.
func (s *Server) PortForward(_ context.Context, _ *runtimeapi.PortForwardRequest) (*runtimeapi.PortForwardResponse, error) {
	return &runtimeapi.PortForwardResponse{}, nil
}
