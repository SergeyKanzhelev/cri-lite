// cri-lite is a CRI proxy that enforces policies on CRI API calls.
package main

import (
	"flag"
	"strconv"

	"k8s.io/klog/v2"

	"cri-lite/pkg/config"
	"cri-lite/pkg/policy"
	"cri-lite/pkg/proxy"
	"cri-lite/pkg/version"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	runtimeEndpoint := flag.String("runtime-endpoint", "", "Endpoint of CRI runtime service")
	imageEndpoint := flag.String("image-endpoint", "", "Endpoint of CRI image service")
	flag.StringVar(runtimeEndpoint, "r", "", "Endpoint of CRI runtime service (shorthand)")
	flag.StringVar(imageEndpoint, "i", "", "Endpoint of CRI image service (shorthand)")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		klog.Infof("cri-lite version %s", version.Version)

		return
	}

	cfg, err := config.LoadFile(*configFile)
	if err != nil {
		klog.Fatalf("failed to load configuration: %v", err)
	}

	// Set klog verbosity level.
	// The command-line flag has precedence over the config file.
	vFlag := flag.Lookup("v")
	if vFlag.Value.String() == "0" {
		_ = vFlag.Value.Set(strconv.Itoa(cfg.Logging.Verbosity))
	}

	klog.Infof("Configuration loaded successfully from %s", *configFile)

	// Override config with flags if provided.
	if *runtimeEndpoint != "" {
		cfg.RuntimeEndpoint = *runtimeEndpoint
	}

	if *imageEndpoint != "" {
		cfg.ImageEndpoint = *imageEndpoint
	}

	klog.Infof("Using runtime endpoint: %s", cfg.RuntimeEndpoint)
	klog.Infof("Using image endpoint: %s", cfg.ImageEndpoint)

	for _, endpoint := range cfg.Endpoints {
		go startEndpoint(endpoint, cfg)
	}

	// Keep the main goroutine alive.
	select {}
}

func startEndpoint(endpoint config.Endpoint, cfg *config.Config) {
	klog.Infof("Starting server for endpoint: %s", endpoint.Endpoint)

	server, err := proxy.NewServer(cfg.RuntimeEndpoint, cfg.ImageEndpoint)
	if err != nil {
		klog.Fatalf("failed to create server for endpoint %s: %v", endpoint.Endpoint, err)
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
				klog.Fatalf("pod-sandbox-id must be a string for endpoint %s", endpoint.Endpoint)
			}
		}

		if val, ok := endpoint.Policy.Attributes["pod-sandbox-from-caller-pid"]; ok {
			podSandboxFromCallerPID, ok = val.(bool)
			if !ok {
				klog.Fatalf("pod-sandbox-from-caller-pid must be a boolean for endpoint %s", endpoint.Endpoint)
			}
		}

		p = policy.NewPodScopedPolicy(podSandboxID, podSandboxFromCallerPID, server.GetRuntimeClient())
	default:
		klog.Fatalf("unknown policy: %s", endpoint.Policy.Name)
	}

	server.SetPolicy(p)

	err = server.Start(endpoint.Endpoint)
	if err != nil {
		klog.Fatalf("failed to start server for endpoint %s: %v", endpoint.Endpoint, err)
	}
}
