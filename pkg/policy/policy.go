// Package policy provides interfaces and implementations for enforcing CRI API access policies.
package policy

import (
	"context"
	"errors"
	"fmt"
	"log"
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
			// HACK: This is a placeholder for getting the pod sandbox ID from the PID.
			// In a real implementation, this would involve a lookup mechanism.
			podSandboxID = fmt.Sprintf("pid-%d", authInfo.GetPID())
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
			// HACK: Check the container's pod sandbox ID.
		case *runtimeapi.StopContainerRequest:
			// HACK: Check the container's pod sandbox ID.
		case *runtimeapi.RemoveContainerRequest:
			// HACK: Check the container's pod sandbox ID.
		case *runtimeapi.ExecSyncRequest:
			// HACK: Check the container's pod sandbox ID.
		case *runtimeapi.ExecRequest:
			// HACK: Check the container's pod sandbox ID.
		case *runtimeapi.AttachRequest:
			// HACK: Check the container's pod sandbox ID.
		case *runtimeapi.PortForwardRequest:
			// HACK: Check the container's pod sandbox ID.
		case *runtimeapi.ContainerStatsRequest:
			// HACK: Check the container's pod sandbox ID.
		case *runtimeapi.ListContainerStatsRequest:
			// HACK: Check the container's pod sandbox ID.
		case *runtimeapi.UpdateContainerResourcesRequest:
			// HACK: Check the container's pod sandbox ID.
		}

		return handler(ctx, req)
	}
}
