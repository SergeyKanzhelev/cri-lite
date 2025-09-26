// Package policy provides interfaces and implementations for enforcing CRI API access policies.
package policy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// ErrMethodNotAllowed is returned when a CRI method is not allowed by a policy.
var ErrMethodNotAllowed = errors.New("method not allowed by policy")

// Policy is the interface for a cri-lite policy.
type Policy interface {
	// UnaryInterceptor returns a gRPC unary interceptor that enforces the policy.
	UnaryInterceptor() grpc.UnaryServerInterceptor
}

// readOnlyPolicy is a policy that allows only read-only CRI calls.
type readOnlyPolicy struct{}

// NewReadOnlyPolicy creates a new ReadOnly policy.
//
//nolint:ireturn // This function intentionally returns an interface.
func NewReadOnlyPolicy() Policy {
	return &readOnlyPolicy{}
}

// UnaryInterceptor implements the Policy interface.
func (p *readOnlyPolicy) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// List of allowed read-only methods.
		allowedMethods := map[string]bool{
			"/runtime.v1.RuntimeService/Version":          true,
			"/runtime.v1.RuntimeService/ListContainers":   true,
			"/runtime.v1.RuntimeService/ContainerStatus":  true,
			"/runtime.v1.RuntimeService/ListPodSandbox":   true,
			"/runtime.v1.RuntimeService/PodSandboxStatus": true,
			"/runtime.v1.ImageService/ListImages":         true,
			"/runtime.v1.ImageService/ImageStatus":        true,
			"/runtime.v1.ImageService/ImageFsInfo":        true,
		}

		if !allowedMethods[info.FullMethod] {
			return nil, status.Errorf(codes.PermissionDenied, "%s: %s", ErrMethodNotAllowed, info.FullMethod)
		}

		return handler(ctx, req)
	}
}

// imageManagementPolicy is a policy that allows only image management CRI calls.
type imageManagementPolicy struct{}

// NewImageManagementPolicy creates a new ImageManagement policy.
//
//nolint:ireturn // This function intentionally returns an interface.
func NewImageManagementPolicy() Policy {
	return &imageManagementPolicy{}
}

// UnaryInterceptor implements the Policy interface.
func (p *imageManagementPolicy) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if !strings.HasPrefix(info.FullMethod, "/runtime.v1.ImageService/") {
			return nil, status.Errorf(codes.PermissionDenied, "%s: %s", ErrMethodNotAllowed, info.FullMethod)
		}

		return handler(ctx, req)
	}
}

// podScopedPolicy is a policy that restricts access to a single pod sandbox.
type podScopedPolicy struct {
	podSandboxID            string
	podSandboxFromCallerPID bool
	runtimeClient           runtimeapi.RuntimeServiceClient
}

// NewPodScopedPolicy creates a new PodScoped policy.
//
//nolint:ireturn // This function intentionally returns an interface.
func NewPodScopedPolicy(podSandboxID string, podSandboxFromCallerPID bool, runtimeClient runtimeapi.RuntimeServiceClient) Policy {
	return &podScopedPolicy{
		podSandboxID:            podSandboxID,
		podSandboxFromCallerPID: podSandboxFromCallerPID,
		runtimeClient:           runtimeClient,
	}
}

// UnaryInterceptor implements the Policy interface.
func (p *podScopedPolicy) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if strings.HasPrefix(info.FullMethod, "/runtime.v1.ImageService/") {
			return handler(ctx, req)
		}

		if !strings.HasPrefix(info.FullMethod, "/runtime.v1.RuntimeService/") {
			return nil, status.Errorf(codes.PermissionDenied, "%s: %s", ErrMethodNotAllowed, info.FullMethod)
		}

		podSandboxID := p.podSandboxID
		if p.podSandboxFromCallerPID {
			peer, ok := peer.FromContext(ctx)
			if !ok {
				return nil, status.Errorf(codes.InvalidArgument, "failed to get peer from context")
			}

			authInfo, ok := peer.AuthInfo.(interface{ GetPID() int32 })
			if !ok {
				return nil, status.Errorf(codes.InvalidArgument, "failed to get auth info from context")
			}

			log.Printf("peer PID: %d", authInfo.GetPID())
			var err error
			podSandboxID, err = p.getPodSandboxIDFromPID(authInfo.GetPID())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get pod sandbox ID from PID: %v", err)
			}
		}

		switch r := req.(type) {
		case *runtimeapi.ListContainersRequest:
			if r.GetFilter() != nil && r.GetFilter().GetPodSandboxId() != "" && r.GetFilter().GetPodSandboxId() != podSandboxID {
				return nil, status.Errorf(codes.PermissionDenied, "%s: ListContainersRequest.Filter.PodSandboxId does not match", ErrMethodNotAllowed)
			}
		case *runtimeapi.CreateContainerRequest:
			if r.GetPodSandboxId() != podSandboxID {
				return nil, status.Errorf(codes.PermissionDenied, "%s: CreateContainerRequest.PodSandboxId does not match", ErrMethodNotAllowed)
			}
		case *runtimeapi.StartContainerRequest:
			if err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID); err != nil {
				return nil, status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
			}
		case *runtimeapi.StopContainerRequest:
			if err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID); err != nil {
				return nil, status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
			}
		case *runtimeapi.RemoveContainerRequest:
			if err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID); err != nil {
				return nil, status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
			}
		case *runtimeapi.ExecSyncRequest:
			if err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID); err != nil {
				return nil, status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
			}
		case *runtimeapi.ExecRequest:
			if err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID); err != nil {
				return nil, status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
			}
		case *runtimeapi.AttachRequest:
			if err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID); err != nil {
				return nil, status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
			}
		case *runtimeapi.PortForwardRequest:
			if r.GetPodSandboxId() != podSandboxID {
				return nil, status.Errorf(codes.PermissionDenied, "%s: PortForwardRequest.PodSandboxId does not match", ErrMethodNotAllowed)
			}
		case *runtimeapi.ContainerStatsRequest:
			if err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID); err != nil {
				return nil, status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
			}
		case *runtimeapi.ListContainerStatsRequest:
			if r.GetFilter() != nil && r.GetFilter().GetPodSandboxId() != "" && r.GetFilter().GetPodSandboxId() != podSandboxID {
				return nil, status.Errorf(codes.PermissionDenied, "%s: ListContainerStatsRequest.Filter.PodSandboxId does not match", ErrMethodNotAllowed)
			}
		case *runtimeapi.UpdateContainerResourcesRequest:
			if err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID); err != nil {
				return nil, status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
			}
		}

		return handler(ctx, req)
	}
}

func (p *podScopedPolicy) getPodSandboxIDFromPID(pid int32) (string, error) {
	cgroupFile, err := os.Open(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", fmt.Errorf("failed to open cgroup file: %w", err)
	}
	defer cgroupFile.Close()

	scanner := bufio.NewScanner(cgroupFile)
	for scanner.Scan() {
		line := scanner.Text()
		// This regex is designed to extract the container ID from a cgroup line.
		// It looks for a pattern that starts with a path, followed by a container ID,
		// and ends with a .scope extension.
		// For example: /kubepods/burstable/pod<POD_ID>/<CONTAINER_ID>
		r := regexp.MustCompile(`\S+/(?:docker|crio)-([0-9a-f]{64})\.scope$`)
		matches := r.FindStringSubmatch(line)
		if len(matches) == 2 {
			containerID := matches[1]
			// HACK: We need to get the full container ID from the runtime.
			// This is a placeholder for that logic.
			return p.getPodSandboxIDFromContainerID(context.Background(), containerID)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read cgroup file: %w", err)
	}

	return "", errors.New("failed to find container ID in cgroup file")
}

func (p *podScopedPolicy) getPodSandboxIDFromContainerID(ctx context.Context, containerID string) (string, error) {
	resp, err := p.runtimeClient.ContainerStatus(ctx, &runtimeapi.ContainerStatusRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get container status: %w", err)
	}

	return resp.GetStatus().GetPodSandboxId(), nil
}

func (p *podScopedPolicy) verifyContainerPodSandboxID(ctx context.Context, containerID, expectedPodSandboxID string) error {
	podSandboxID, err := p.getPodSandboxIDFromContainerID(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get pod sandbox ID from container ID: %w", err)
	}

	if podSandboxID != expectedPodSandboxID {
		return fmt.Errorf("container %q does not belong to pod sandbox %q", containerID, expectedPodSandboxID)
	}

	return nil
}
