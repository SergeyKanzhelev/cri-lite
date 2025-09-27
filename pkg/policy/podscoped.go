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

		resp, err := handler(ctx, req)
		if err != nil {
			return nil, err
		}

		if r, ok := resp.(*runtimeapi.ListContainersResponse); ok {
			var containers []*runtimeapi.Container

			for _, c := range r.GetContainers() {
				if c.GetPodSandboxId() == podSandboxID {
					containers = append(containers, c)
				}
			}

			r.Containers = containers
		}

		return resp, nil
	}
}

func (p *podScopedPolicy) getPodSandboxIDFromPID(ctx context.Context, pid int32) (string, error) {
	log.Printf("mapping pid %d to sandbox id", pid)

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
		// This regex is designed to extract a container ID from a cgroup line.
		r := regexp.MustCompile(`([0-9a-f]{64})`)

		matches := r.FindStringSubmatch(line)
		if len(matches) == 2 {
			containerID := matches[1]
			log.Printf("found container id %q for pid %d", containerID, pid)
			// HACK: We need to get the full container ID from the runtime.
			// This is a placeholder for that logic.
			return p.getPodSandboxIDFromContainerID(ctx, containerID)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read cgroup file: %w", err)
	}

	return "", fmt.Errorf("failed to find container ID for pid %d", pid)
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
		return status.Errorf(codes.PermissionDenied, "%s: container %s does not belong to pod sandbox %s", ErrMethodNotAllowed, containerID, expectedPodSandboxID)
	}

	return nil
}

func (p *podScopedPolicy) verifyRequest(ctx context.Context, req interface{}, podSandboxID string) error {
	switch r := req.(type) {
	case *runtimeapi.ListContainersRequest:
		if r.GetFilter() == nil {
			r.Filter = &runtimeapi.ContainerFilter{
				PodSandboxId: podSandboxID,
			}
		} else {
			if r.GetFilter().GetPodSandboxId() != "" && r.GetFilter().GetPodSandboxId() != podSandboxID {
				return status.Errorf(codes.PermissionDenied, "%s: ListContainersRequest.Filter.PodSandboxId does not match", ErrMethodNotAllowed)
			}

			r.Filter.PodSandboxId = podSandboxID
		}
	case *runtimeapi.ListContainerStatsRequest:
		if r.GetFilter() == nil {
			r.Filter = &runtimeapi.ContainerStatsFilter{
				PodSandboxId: podSandboxID,
			}
		} else {
			if r.GetFilter().GetPodSandboxId() != "" && r.GetFilter().GetPodSandboxId() != podSandboxID {
				return status.Errorf(codes.PermissionDenied, "%s: ListContainerStatsRequest.Filter.PodSandboxId does not match", ErrMethodNotAllowed)
			}

			r.Filter.PodSandboxId = podSandboxID
		}
	case *runtimeapi.ListPodSandboxStatsRequest:
		if r.GetFilter() == nil {
			r.Filter = &runtimeapi.PodSandboxStatsFilter{
				Id: podSandboxID,
			}
		} else {
			if r.GetFilter().GetId() != "" && r.GetFilter().GetId() != podSandboxID {
				return status.Errorf(codes.PermissionDenied, "%s: ListPodSandboxStatsRequest.Filter.Id does not match", ErrMethodNotAllowed)
			}

			r.Filter.Id = podSandboxID
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
	case *runtimeapi.StopPodSandboxRequest:
		if r.GetPodSandboxId() != podSandboxID {
			return status.Errorf(codes.PermissionDenied, "%s: StopPodSandboxRequest.PodSandboxId does not match", ErrMethodNotAllowed)
		}
	case *runtimeapi.RemovePodSandboxRequest:
		if r.GetPodSandboxId() != podSandboxID {
			return status.Errorf(codes.PermissionDenied, "%s: RemovePodSandboxRequest.PodSandboxId does not match", ErrMethodNotAllowed)
		}
	case *runtimeapi.PodSandboxStatusRequest:
		if r.GetPodSandboxId() != podSandboxID {
			return status.Errorf(codes.PermissionDenied, "%s: PodSandboxStatusRequest.PodSandboxId does not match", ErrMethodNotAllowed)
		}
	case *runtimeapi.ContainerStatusRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.ExecSyncRequest:
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
		err := p.verifyContainerPodSandboxID(ctx, r.GetPodSandboxId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.UpdateContainerResourcesRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.ContainerStatsRequest:
		err := p.verifyContainerPodSandboxID(ctx, r.GetContainerId(), podSandboxID)
		if err != nil {
			return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
		}
	case *runtimeapi.PodSandboxStatsRequest:
		if r.GetPodSandboxId() != podSandboxID {
			return status.Errorf(codes.PermissionDenied, "%s: PodSandboxStatsRequest.PodSandboxId does not match", ErrMethodNotAllowed)
		}
	case *runtimeapi.UpdatePodSandboxResourcesRequest:
		if r.GetPodSandboxId() != podSandboxID {
			return status.Errorf(codes.PermissionDenied, "%s: UpdatePodSandboxResourcesRequest.PodSandboxId does not match", ErrMethodNotAllowed)
		}
	}

	return nil
}
