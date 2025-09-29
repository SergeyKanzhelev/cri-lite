// Package policy provides interfaces and implementations for enforcing CRI API access policies.
package policy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"
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
func NewPodScopedPolicy(podSandboxID string, podSandboxFromCallerPID bool, runtimeClient runtimeapi.RuntimeServiceClient) Policy {
	return &podScopedPolicy{
		podSandboxID:            podSandboxID,
		podSandboxFromCallerPID: podSandboxFromCallerPID,
		runtimeClient:           runtimeClient,
	}
}

// Name implements the Policy interface.
func (p *podScopedPolicy) Name() string {
	return "podScoped"
}

// UnaryInterceptor implements the Policy interface.
func (p *podScopedPolicy) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		interceptor := func(
			ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (interface{}, error) {
			logger := klog.FromContext(ctx)
			if strings.HasPrefix(info.FullMethod, "/runtime.v1.ImageService/") {
				return handler(ctx, req)
			}

			if !strings.HasPrefix(info.FullMethod, "/runtime.v1.RuntimeService/") {
				return nil, status.Errorf(codes.PermissionDenied, "%s: %s", ErrMethodNotAllowed, info.FullMethod)
			}

			podSandboxID := p.podSandboxID
			if p.podSandboxFromCallerPID {
				peerInfo, isPeer := peer.FromContext(ctx)
				if !isPeer {
					return nil, status.Errorf(codes.InvalidArgument, "failed to get peer from context")
				}

				authInfo, ok := peerInfo.AuthInfo.(interface{ GetPID() int32 })
				if !ok {
					return nil, status.Errorf(codes.InvalidArgument, "failed to get auth info from context")
				}

				logger.V(4).Info("peer PID", "pid", authInfo.GetPID())

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
		return interceptor(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			return loggingInterceptor(ctx, req, info, handler)
		})
	}
}

// TODO: when it will become a problem we should add caching here.
func (p *podScopedPolicy) getPodSandboxIDFromPID(ctx context.Context, pid int32) (string, error) {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("mapping pid to sandbox id", "pid", pid)

	cgroupFile, err := os.Open(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", fmt.Errorf("failed to open cgroup file: %w", err)
	}

	defer func() {
		err := cgroupFile.Close()
		if err != nil {
			logger.Error(err, "failed to close cgroup file")
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
			logger.V(4).Info("found container id for pid", "containerID", containerID, "pid", pid)

			return p.getPodSandboxIDFromContainerID(ctx, containerID)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read cgroup file: %w", err)
	}

	return "", fmt.Errorf("failed to find container ID for pid %d", pid)
}

// TODO: when it will become a problem we should add caching here.
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

func (p *podScopedPolicy) verifyPodSandboxIDMatch(requestedPodSandboxID, expectedPodSandboxID, methodName string) error {
	if requestedPodSandboxID != expectedPodSandboxID {
		return status.Errorf(codes.PermissionDenied, "%s: %s does not match", ErrMethodNotAllowed, methodName)
	}

	return nil
}

func (p *podScopedPolicy) verifyContainerIDBelongsToPod(ctx context.Context, containerID, expectedPodSandboxID string) error {
	err := p.verifyContainerPodSandboxID(ctx, containerID, expectedPodSandboxID)
	if err != nil {
		return status.Errorf(codes.PermissionDenied, "%s: %v", ErrMethodNotAllowed, err)
	}

	return nil
}

func (p *podScopedPolicy) verifyRequest(ctx context.Context, req interface{}, podSandboxID string) error {
	switch r := req.(type) {
	case *runtimeapi.ListContainersRequest:
		return p.verifyListContainersRequest(r, podSandboxID)
	case *runtimeapi.ListContainerStatsRequest:
		return p.verifyListContainerStatsRequest(r, podSandboxID)
	case *runtimeapi.ListPodSandboxStatsRequest:
		return p.verifyListPodSandboxStatsRequest(r, podSandboxID)
	case *runtimeapi.CreateContainerRequest:
		return p.verifyPodSandboxIDMatch(r.GetPodSandboxId(), podSandboxID, "CreateContainerRequest.PodSandboxId")
	case *runtimeapi.StopPodSandboxRequest:
		return p.verifyPodSandboxIDMatch(r.GetPodSandboxId(), podSandboxID, "StopPodSandboxRequest.PodSandboxId")
	case *runtimeapi.RemovePodSandboxRequest:
		return p.verifyPodSandboxIDMatch(r.GetPodSandboxId(), podSandboxID, "RemovePodSandboxRequest.PodSandboxId")
	case *runtimeapi.PodSandboxStatusRequest:
		return p.verifyPodSandboxIDMatch(r.GetPodSandboxId(), podSandboxID, "PodSandboxStatusRequest.PodSandboxId")
	case *runtimeapi.UpdatePodSandboxResourcesRequest:
		return p.verifyPodSandboxIDMatch(r.GetPodSandboxId(), podSandboxID, "UpdatePodSandboxResourcesRequest.PodSandboxId")
	case *runtimeapi.StartContainerRequest:
		return p.verifyContainerIDBelongsToPod(ctx, r.GetContainerId(), podSandboxID)
	case *runtimeapi.StopContainerRequest:
		return p.verifyContainerIDBelongsToPod(ctx, r.GetContainerId(), podSandboxID)
	case *runtimeapi.RemoveContainerRequest:
		return p.verifyContainerIDBelongsToPod(ctx, r.GetContainerId(), podSandboxID)
	case *runtimeapi.ContainerStatusRequest:
		return p.verifyContainerIDBelongsToPod(ctx, r.GetContainerId(), podSandboxID)
	case *runtimeapi.ExecSyncRequest:
		return p.verifyContainerIDBelongsToPod(ctx, r.GetContainerId(), podSandboxID)
	case *runtimeapi.AttachRequest:
		return p.verifyContainerIDBelongsToPod(ctx, r.GetContainerId(), podSandboxID)
	case *runtimeapi.PortForwardRequest:
		return p.verifyContainerIDBelongsToPod(ctx, r.GetPodSandboxId(), podSandboxID)
	case *runtimeapi.UpdateContainerResourcesRequest:
		return p.verifyContainerIDBelongsToPod(ctx, r.GetContainerId(), podSandboxID)
	case *runtimeapi.ContainerStatsRequest:
		return p.verifyContainerIDBelongsToPod(ctx, r.GetContainerId(), podSandboxID)
	default:
		return nil
	}
}

func (p *podScopedPolicy) verifyListContainersRequest(r *runtimeapi.ListContainersRequest, podSandboxID string) error {
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

	return nil
}

func (p *podScopedPolicy) verifyListContainerStatsRequest(r *runtimeapi.ListContainerStatsRequest, podSandboxID string) error {
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

	return nil
}

func (p *podScopedPolicy) verifyListPodSandboxStatsRequest(r *runtimeapi.ListPodSandboxStatsRequest, podSandboxID string) error {
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

	return nil
}
