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
}

// NewServer creates a new fake CRI server.
func NewServer(socketPath string) (*grpc.Server, net.Listener, error) {
	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on socket: %w", err)
	}

	s := &Server{}
	grpcServer := grpc.NewServer()
	runtimeapi.RegisterRuntimeServiceServer(grpcServer, s)
	runtimeapi.RegisterImageServiceServer(grpcServer, s)

	return grpcServer, lis, nil
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
	containers := []*runtimeapi.Container{
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
	}

	if req.GetFilter() != nil && req.GetFilter().GetId() != "" {
		for _, c := range containers {
			if c.GetId() == req.GetFilter().GetId() {
				return &runtimeapi.ListContainersResponse{
					Containers: []*runtimeapi.Container{c},
				}, nil
			}
		}

		return &runtimeapi.ListContainersResponse{}, nil
	}

	return &runtimeapi.ListContainersResponse{
		Containers: containers,
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
func (s *Server) ListContainerStats(_ context.Context, _ *runtimeapi.ListContainerStatsRequest) (*runtimeapi.ListContainerStatsResponse, error) {
	return &runtimeapi.ListContainerStatsResponse{}, nil
}

// PodSandboxStats returns fake pod sandbox stats.
func (s *Server) PodSandboxStats(_ context.Context, _ *runtimeapi.PodSandboxStatsRequest) (*runtimeapi.PodSandboxStatsResponse, error) {
	return &runtimeapi.PodSandboxStatsResponse{}, nil
}

// ListPodSandboxStats returns fake pod sandbox stats.
func (s *Server) ListPodSandboxStats(_ context.Context, _ *runtimeapi.ListPodSandboxStatsRequest) (*runtimeapi.ListPodSandboxStatsResponse, error) {
	return &runtimeapi.ListPodSandboxStatsResponse{}, nil
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