// Package config provides the configuration structures and loading logic for cri-lite.
package config

import (
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
)

// Config defines the global configuration for cri-lite.
type Config struct {
	RuntimeEndpoint string     `yaml:"runtime-endpoint"`
	ImageEndpoint   string     `yaml:"image-endpoint"`
	Timeout         int        `yaml:"timeout"`
	Debug           bool       `yaml:"debug"`
	Endpoints       []Endpoint `yaml:"endpoints"`
}

// Endpoint defines the configuration for a single cri-lite endpoint.
type Endpoint struct {
	Endpoint                string   `yaml:"endpoint"`
	Policies                []string `yaml:"policies"`
	PodSandboxID            string   `yaml:"pod-sandbox-id,omitempty"`
	PodSandboxFromCallerPID bool     `yaml:"pod-sandbox-from-caller-pid,omitempty"`
}

// LoadFile reads and parses the configuration from a YAML file.
func LoadFile(path string) (*Config, error) {
	//nolint:gosec // The path is controlled by a flag, not user input.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	var config Config

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %q: %w", path, err)
	}

	return &config, nil
}
