// Package framework provides a test framework for end-to-end testing of cri-lite.
package framework

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"cri-lite/pkg/policy"
	"cri-lite/pkg/proxy"
)

// Framework handles the setup and teardown of the E2E test environment.
type Framework struct {
	RuntimeEndpoint string
	ProxyServer     *proxy.Server
	ProxySocket     string
	sockDir         string
}

var errProxyFailedToStart = errors.New("proxy server failed to start")

// New sets up the E2E test framework.
func New() (*Framework, error) {
	runtimeEndpoint, err := findRuntimeEndpoint()
	if err != nil {
		return nil, fmt.Errorf("failed to find runtime endpoint: %w", err)
	}

	return &Framework{
			RuntimeEndpoint: runtimeEndpoint,
		},
		nil
}

// SetupProxy creates and starts the cri-lite proxy.
func (f *Framework) SetupProxy() error {
	var err error

	f.sockDir, err = os.MkdirTemp("", "cri-lite-e2e")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	f.ProxySocket = f.createSocket()

	f.ProxyServer, err = proxy.NewServer(f.RuntimeEndpoint, f.RuntimeEndpoint)
	if err != nil {
		return fmt.Errorf("failed to create proxy server: %w", err)
	}

	p := policy.NewPodScopedPolicy("", true, f.ProxyServer.GetRuntimeClient())
	f.ProxyServer.SetPolicy(p)

	go func() {
		err := f.ProxyServer.Start(f.ProxySocket)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start proxy server: %v\n", err)
		}
	}()

	for range 20 {
		dialer := &net.Dialer{}

		conn, err := dialer.DialContext(context.Background(), "unix", f.ProxySocket)
		if err == nil {
			err := conn.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to close connection: %v\n", err)
			}

			return nil
		}

		fmt.Fprintf(os.Stderr, "failed to connect to proxy, retrying: %v\n", err)
		time.Sleep(100 * time.Millisecond)
	}

	return errProxyFailedToStart
}

// TeardownProxy stops the proxy server.
func (f *Framework) TeardownProxy() {
	if f.sockDir != "" {
		err := os.RemoveAll(f.sockDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to remove socket directory: %v\n", err)
		}
	}
}

// GetRuntimeClient returns a client for the CRI runtime service.
func (f *Framework) GetRuntimeClient() (runtimeapi.RuntimeServiceClient, error) {
	conn, err := grpc.NewClient("unix://"+f.ProxySocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	return runtimeapi.NewRuntimeServiceClient(conn), nil
}

// GetImageClient returns a client for the CRI image service.
func (f *Framework) GetImageClient() (runtimeapi.ImageServiceClient, error) {
	conn, err := grpc.NewClient("unix://"+f.ProxySocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	return runtimeapi.NewImageServiceClient(conn), nil
}

// GetRealRuntimeClient returns a client for the CRI runtime service that connects directly to the real runtime.
func (f *Framework) GetRealRuntimeClient() (runtimeapi.RuntimeServiceClient, error) {
	conn, err := grpc.NewClient(f.RuntimeEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to real runtime: %w", err)
	}

	return runtimeapi.NewRuntimeServiceClient(conn), nil
}

// GetRealImageClient returns a client for the CRI image service that connects directly to the real runtime.
func (f *Framework) GetRealImageClient() (runtimeapi.ImageServiceClient, error) {
	conn, err := grpc.NewClient(f.RuntimeEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to real runtime: %w", err)
	}

	return runtimeapi.NewImageServiceClient(conn), nil
}

// RandomSuffix returns a random string to append to resource names.
func RandomSuffix() string {
	n, err := rand.Int(rand.Reader, big.NewInt(100000))
	if err != nil {
		panic(err)
	}

	return n.String()
}

// createSocket creates a temporary socket path.
func (f *Framework) createSocket() string {
	tmpfile, err := os.CreateTemp(f.sockDir, "socket-*.sock")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp file: %v", err))
	}

	path := tmpfile.Name()
	if err := tmpfile.Close(); err != nil {
		panic(fmt.Sprintf("failed to close temp file: %v", err))
	}

	if err := os.Remove(path); err != nil {
		panic(fmt.Sprintf("failed to remove temp file: %v", err))
	}

	return path
}

var errNoRuntimeEndpointFound = errors.New("no container runtime endpoint found")

// findRuntimeEndpoint attempts to find a container runtime socket.
func findRuntimeEndpoint() (string, error) {
	endpoints := []string{
		"unix:///var/run/dockershim.sock",
		"unix:///run/containerd/containerd.sock",
		"unix:///run/crio/crio.sock",
	}

	for _, endpoint := range endpoints {
		path := endpoint[len("unix://"):]
		if _, err := os.Stat(path); err == nil {
			return endpoint, nil
		}
	}

	return "", errNoRuntimeEndpointFound
}
