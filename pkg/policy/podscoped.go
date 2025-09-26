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

var (
	ErrContainerIDNotFound          = errors.New("failed to find container ID in cgroup file")
	ErrUnexpectedNumberOfContainers = errors.New("unexpected number of containers")
	ErrContainerNotInPod            = errors.New("container does not belong to pod sandbox")
)

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
			peerInfo, ok := peer.FromContext(ctx)
			if !ok {
				return nil, status.Errorf(codes.InvalidArgument, "failed to get peer from context")
			}

			authInfo, ok := peerInfo.AuthInfo.(interface{ GetPID() int32 })
			if !ok {
				return nil, status.Errorf(codes.InvalidArgument, "failed to get auth info from context")
			}

			log.Printf("peer PID: %d", authInfo.GetPID())

			var err error

			podSandboxID, err = p.getPodSandboxIDFromPID(ctx, authInfo.GetPID())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get pod sandbox ID from PID: %v", err)
			}
		}

		err := p.verifyRequest(ctx, req, podSandboxID)
		if err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

func (p *podScopedPolicy) getPodSandboxIDFromPID(ctx context.Context, pid int32) (string, error) {
	cgroupFile, err := os.Open(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", fmt.Errorf("failed to open cgroup file: %w", err)
	}

	defer func() {
		err := cgroupFile.Close()
		if err != nil {
			log.Printf("failed to close cgroup file: %v", err)
		}
	}()

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
			return p.getPodSandboxIDFromContainerID(ctx, containerID)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read cgroup file: %w", err)
	}

	return "", ErrContainerIDNotFound
}

func (p *podScopedPolicy) getPodSandboxIDFromContainerID(ctx context.Context, containerID string) (string, error) {
	resp, err := p.runtimeClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{
		Filter: &runtimeapi.ContainerFilter{
			Id: containerID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	if len(resp.GetContainers()) != 1 {
		return "", fmt.Errorf("%w: expected 1, got %d", ErrUnexpectedNumberOfContainers, len(resp.GetContainers()))
	}

	return resp.GetContainers()[0].GetPodSandboxId(), nil
}

func (p *podScopedPolicy) verifyContainerPodSandboxID(ctx context.Context, containerID, expectedPodSandboxID string) error {
	podSandboxID, err := p.getPodSandboxIDFromContainerID(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get pod sandbox ID from container ID: %w", err)
	}

	if podSandboxID != expectedPodSandboxID {
		return fmt.Errorf("%w: container %q, pod sandbox %q", ErrContainerNotInPod, containerID, expectedPodSandboxID)
	}

	return nil
}

func (p *podScopedPolicy) verifyRequest(ctx context.Context, req interface{}, podSandboxID string) error {
	switch r := req.(type) {
	case *runtimeapi.ListContainersRequest:
		if r.GetFilter() != nil && r.GetFilter().GetPodSandboxId() != "" && r.GetFilter().GetPodSandboxId() != podSandboxID {
			return status.Errorf(codes.PermissionDenied, "%s: ListContainersRequest.Filter.PodSandboxId does not match", ErrMethodNotAllowed)
		}
	case *runtimeapi.CreateContainerRequest:
		if r.GetPodSandboxId() != podSandboxID {
			return status.Errorf(codes.PermissionDenied, "%s: CreateContainerRequest.PodSandboxId does not match", ErrMethodNotAllowed)
		}
	case *runtimeapi.StartContainerRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.StopContainerRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.RemoveContainerRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.ExecSyncRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.ExecRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.AttachRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.PortForwardRequest:
		if r.GetPodSandboxId() != podSandboxID {
			return status.Errorf(codes.PermissionDenied, "%s: PortForwardRequest.PodSandboxId does not match", ErrMethodNotAllowed)
		}
	case *runtimeapi.ContainerStatsRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.ListContainerStatsRequest:
		if r.GetFilter() != nil && r.GetFilter().GetPodSandboxId() != "" && r.GetFilter().GetPodSandboxId() != podSandboxID {
			return status.Errorf(codes.PermissionDenied, "%s: ListContainerStatsRequest.Filter.PodSandboxId does not match", ErrMethodNotAllowed)
		}
	case *runtimeapi.UpdateContainerResourcesRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	}

	return nil
}
