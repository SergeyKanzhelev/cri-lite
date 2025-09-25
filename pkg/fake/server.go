// Package fake provides a fake CRI server for testing purposes.
package fake

import (
	"context"
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// Server is a fake CRI server for testing.
type Server struct {
	runtimeapi.UnimplementedRuntimeServiceServer
	runtimeapi.UnimplementedImageServiceServer
}

// Start starts the fake CRI server on the specified socket.
func (s *Server) Start(socketPath string) error {
	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	lis, err := (&net.ListenConfig{
		Control:   nil,
		KeepAlive: 0,
	}).Listen(context.Background(), "unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}

	grpcServer := grpc.NewServer()
	runtimeapi.RegisterRuntimeServiceServer(grpcServer, s)
	runtimeapi.RegisterImageServiceServer(grpcServer, s)

	err = grpcServer.Serve(lis)
	if err != nil {
		return fmt.Errorf("failed to serve grpc server: %w", err)
	}

	return nil
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
func (s *Server) ListContainers(_ context.Context, _ *runtimeapi.ListContainersRequest) (*runtimeapi.ListContainersResponse, error) {
	return &runtimeapi.ListContainersResponse{
		Containers: []*runtimeapi.Container{
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
