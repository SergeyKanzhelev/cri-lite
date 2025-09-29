// Package policy provides interfaces and implementations for enforcing CRI API access policies.
package policy

import (
	"context"
	"errors"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

var (
	ErrMethodNotAllowed  = errors.New("method not allowed by policy")
	ErrUnknownPolicyType = errors.New("unknown policy type")
)

// Policy is the interface for a cri-lite policy.
type Policy interface {
	// Name returns the name of the policy.
	Name() string
	// UnaryInterceptor returns a gRPC unary server interceptor.
	UnaryInterceptor() grpc.UnaryServerInterceptor
}

// Config is the configuration for a policy.
type Config struct {
	ReadOnly bool `yaml:"read-only"`
}

// NewFromConfig creates a new policy from a config file.
func NewFromConfig(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal policy config: %w", err)
	}

	return NewFromConfigData(&config)
}

// NewFromConfigData creates a new policy from a config struct.
func NewFromConfigData(config *Config) (Policy, error) {
	if config.ReadOnly {
		return NewReadOnlyPolicy(), nil
	}

	return nil, ErrUnknownPolicyType
}

func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	logger := klog.NewKlogr().WithValues("method", info.FullMethod)
	ctx = klog.NewContext(ctx, logger)

	resp, err := handler(ctx, req)
	if err != nil {
		logger.V(4).Error(err, "request denied by policy")
		return nil, err
	}
	logger.V(4).Info("request allowed by policy")
	return resp, nil
}
