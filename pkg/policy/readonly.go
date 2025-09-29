// Package policy provides interfaces and implementations for enforcing CRI API access policies.
package policy

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// readOnlyPolicy is a policy that allows only read-only CRI calls.
type readOnlyPolicy struct{}

// NewReadOnlyPolicy creates a new ReadOnly policy.
//
//nolint:ireturn // This function intentionally returns an interface.
func NewReadOnlyPolicy() Policy {
	return &readOnlyPolicy{}
}

// Name implements the Policy interface.
func (p *readOnlyPolicy) Name() string {
	return "readonly"
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
			"/runtime.v1.RuntimeService/Version":             true,
			"/runtime.v1.RuntimeService/Status":              true,
			"/runtime.v1.RuntimeService/ListContainers":      true,
			"/runtime.v1.RuntimeService/ContainerStatus":     true,
			"/runtime.v1.RuntimeService/ListPodSandbox":      true,
			"/runtime.v1.RuntimeService/PodSandboxStatus":    true,
			"/runtime.v1.RuntimeService/ContainerStats":      true,
			"/runtime.v1.RuntimeService/ListContainerStats":  true,
			"/runtime.v1.RuntimeService/PodSandboxStats":     true,
			"/runtime.v1.RuntimeService/ListPodSandboxStats": true,
			"/runtime.v1.ImageService/ListImages":            true,
			"/runtime.v1.ImageService/ImageStatus":           true,
			"/runtime.v1.ImageService/ImageFsInfo":           true,
		}

		if !allowedMethods[info.FullMethod] {
			return nil, status.Errorf(codes.PermissionDenied, "%s: %s", ErrMethodNotAllowed, info.FullMethod)
		}

		return handler(ctx, req)
	}
}
