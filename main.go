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
		go func(endpoint config.Endpoint) {
			log.Printf("Starting server for endpoint: %s", endpoint.Endpoint)

			server, err := proxy.NewServer(cfg.RuntimeEndpoint, cfg.ImageEndpoint)
			if err != nil {
				log.Fatalf("failed to create server for endpoint %s: %v", endpoint.Endpoint, err)
			}

			var policies []policy.Policy

			for _, p := range endpoint.Policies {
				switch p {
				case "ReadOnly":
					policies = append(policies, policy.NewReadOnlyPolicy())
				case "ImageManagement":
					policies = append(policies, policy.NewImageManagementPolicy())
				case "PodScoped":
					policies = append(policies, policy.NewPodScopedPolicy(endpoint.PodSandboxID, endpoint.PodSandboxFromCallerPID, server.GetRuntimeClient()))
				default:
					log.Fatalf("unknown policy: %s", p)
				}
			}

			server.SetPolicies(policies)

			err = server.Start(endpoint.Endpoint)
			if err != nil {
				log.Fatalf("failed to start server for endpoint %s: %v", endpoint.Endpoint, err)
			}
		}(endpoint)
	}

	// Keep the main goroutine alive.
	select {}
}
