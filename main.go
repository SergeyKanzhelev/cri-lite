// cri-lite is a CRI proxy that enforces policies on CRI API calls.
package main

import (
	"flag"
	"log"

	"cri-lite/pkg/config"
	"cri-lite/pkg/policy"
	"cri-lite/pkg/proxy"
	"cri-lite/pkg/version"
)

func main() {
	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	runtimeEndpoint := flag.String("runtime-endpoint", "", "Endpoint of CRI runtime service")
	imageEndpoint := flag.String("image-endpoint", "", "Endpoint of CRI image service")
	flag.StringVar(runtimeEndpoint, "r", "", "Endpoint of CRI runtime service (shorthand)")
	flag.StringVar(imageEndpoint, "i", "", "Endpoint of CRI image service (shorthand)")
	showVersion := flag.Bool("version", false, "Show version")
	flag.BoolVar(showVersion, "v", false, "Show version (shorthand)")
	flag.Parse()

	if *showVersion {
		log.Printf("cri-lite version %s", version.Version)

		return
	}

	cfg, err := config.LoadFile(*configFile)
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded successfully from %s", *configFile)

	// Override config with flags if provided.
	if *runtimeEndpoint != "" {
		cfg.RuntimeEndpoint = *runtimeEndpoint
	}

	if *imageEndpoint != "" {
		cfg.ImageEndpoint = *imageEndpoint
	}

	log.Printf("Using runtime endpoint: %s", cfg.RuntimeEndpoint)
	log.Printf("Using image endpoint: %s", cfg.ImageEndpoint)

	for _, endpoint := range cfg.Endpoints {
		go startEndpoint(endpoint, cfg)
	}

	// Keep the main goroutine alive.
	select {}
}

func startEndpoint(endpoint config.Endpoint, cfg *config.Config) {
	log.Printf("Starting server for endpoint: %s", endpoint.Endpoint)

	server, err := proxy.NewServer(cfg.RuntimeEndpoint, cfg.ImageEndpoint)
	if err != nil {
		log.Fatalf("failed to create server for endpoint %s: %v", endpoint.Endpoint, err)
	}

	var p policy.Policy

	switch endpoint.Policy.Name {
	case "ReadOnly":
		p = policy.NewReadOnlyPolicy()
	case "ImageManagement":
		p = policy.NewImageManagementPolicy()
	case "PodScoped":
		var (
			podSandboxID            string
			podSandboxFromCallerPID bool
		)

		if val, ok := endpoint.Policy.Attributes["pod-sandbox-id"]; ok {
			podSandboxID, ok = val.(string)
			if !ok {
				log.Fatalf("pod-sandbox-id must be a string for endpoint %s", endpoint.Endpoint)
			}
		}

		if val, ok := endpoint.Policy.Attributes["pod-sandbox-from-caller-pid"]; ok {
			podSandboxFromCallerPID, ok = val.(bool)
			if !ok {
				log.Fatalf("pod-sandbox-from-caller-pid must be a boolean for endpoint %s", endpoint.Endpoint)
			}
		}

		p = policy.NewPodScopedPolicy(podSandboxID, podSandboxFromCallerPID, server.GetRuntimeClient())
	default:
		log.Fatalf("unknown policy: %s", endpoint.Policy.Name)
	}

	server.SetPolicy(p)

	err = server.Start(endpoint.Endpoint)
	if err != nil {
		log.Fatalf("failed to start server for endpoint %s: %v", endpoint.Endpoint, err)
	}
}
