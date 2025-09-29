// Package policy provides interfaces and implementations for enforcing CRI API access policies.
package policy

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// imageManagementPolicy is a policy that allows only image management CRI calls.
type imageManagementPolicy struct{}

// NewImageManagementPolicy creates a new ImageManagement policy.
//
//nolint:ireturn // This function intentionally returns an interface.
func NewImageManagementPolicy() Policy {
	return &imageManagementPolicy{}
}

// Name implements the Policy interface.
func (p *imageManagementPolicy) Name() string {
	return "imageManagement"
}

// UnaryInterceptor implements the Policy interface.
func (p *imageManagementPolicy) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if info.FullMethod == "/runtime.v1.RuntimeService/Version" {
			return handler(ctx, req)
		}

		if !strings.HasPrefix(info.FullMethod, "/runtime.v1.ImageService/") {
			return nil, status.Errorf(codes.PermissionDenied, "%s: %s", ErrMethodNotAllowed, info.FullMethod)
		}

		return handler(ctx, req)
	}
}
