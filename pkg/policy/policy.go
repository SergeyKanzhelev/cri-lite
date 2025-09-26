// Package policy provides interfaces and implementations for enforcing CRI API access policies.
package policy

import (
	"errors"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

var (
	ErrMethodNotAllowed  = errors.New("method not allowed by policy")
	ErrUnknownPolicyType = errors.New("unknown policy type")
)

// Policy is the interface for a cri-lite policy.
type Policy interface {
	// UnaryInterceptor returns a gRPC unary interceptor that enforces the policy.
	UnaryInterceptor() grpc.UnaryServerInterceptor
}

// Config is the configuration for a policy.
type Config struct {
	ReadOnly bool `yaml:"readonly"`
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
