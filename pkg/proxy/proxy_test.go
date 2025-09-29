// Package proxy_test provides tests for the proxy package.
package proxy_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"cri-lite/pkg/fake"
	"cri-lite/pkg/policy"
	"cri-lite/pkg/proxy"
	"cri-lite/pkg/version"
)

const bufSize = 1024 * 1024

type fakeRuntimeService struct {
	runtimeapi.UnimplementedRuntimeServiceServer
}

func (s *fakeRuntimeService) Version(ctx context.Context, req *runtimeapi.VersionRequest) (*runtimeapi.VersionResponse, error) {
	return &runtimeapi.VersionResponse{
		Version:           "1.2.3",
		RuntimeName:       "fake-runtime",
		RuntimeVersion:    "1.0.0",
		RuntimeApiVersion: "v1alpha2",
	}, nil
}

func TestVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	runtimeapi.RegisterRuntimeServiceServer(s, &fakeRuntimeService{})

	go func() {
		err := s.Serve(lis)
		if err != nil {
			t.Errorf("Server exited with error: %v", err)

			return
		}
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			t.Fatalf("Failed to close connection: %v", err)
		}
	}()

	proxyServer := &proxy.Server{}
	proxyServer.SetPolicy(policy.NewReadOnlyPolicy())
	proxyServer.SetRuntimeClient(runtimeapi.NewRuntimeServiceClient(conn))
	proxyServer.SetImageClient(runtimeapi.NewImageServiceClient(conn))

	resp, err := proxyServer.Version(ctx, &runtimeapi.VersionRequest{})
	if err != nil {
		t.Fatalf("Version failed: %v", err)
	}

	expectedVersion := "1.0.0 via cri-lite (" + version.Version + ")"
	if resp.GetRuntimeVersion() != expectedVersion {
		t.Errorf("expected runtime version %s, got %s", expectedVersion, resp.GetRuntimeVersion())
	}

	expectedRuntimeName := "fake-runtime with policy readonly"
	if resp.GetRuntimeName() != expectedRuntimeName {
		t.Errorf("expected runtime name %s, got %s", expectedRuntimeName, resp.GetRuntimeName())
	}
}

func TestProxyReconnect(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sockDir, err := os.MkdirTemp("", "cri-lite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(sockDir)

	fakeRuntimeSocket := fmt.Sprintf("%s/fake-runtime.sock", sockDir)
	proxySocket := fmt.Sprintf("%s/proxy.sock", sockDir)

	proxyServer, err := proxy.NewServer("unix://"+fakeRuntimeSocket, "unix://"+fakeRuntimeSocket)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}
	proxyServer.SetPolicy(policy.NewReadOnlyPolicy())

	go func() {
		if err := proxyServer.Start(proxySocket); err != nil {
			t.Logf("Proxy server exited: %v", err)
		}
	}()
	defer proxyServer.Stop()

	// Wait for proxy to start
	for {
		conn, err := net.Dial("unix", proxySocket)
		if err == nil {
			conn.Close()
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Proxy server did not start in time: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}

	conn, err := grpc.NewClient("unix://"+proxySocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	runtimeClient := runtimeapi.NewRuntimeServiceClient(conn)

	// 1. Try to connect when fake runtime is down
	_, err = runtimeClient.Version(ctx, &runtimeapi.VersionRequest{})
	if err == nil {
		t.Fatal("Expected error when runtime is down, got nil")
	}
	t.Logf("Got expected error when runtime is down: %v", err)

	// 2. Start fake runtime and check that we can connect
	fakeServer, lis, _, err := fake.NewServer(fakeRuntimeSocket)
	if err != nil {
		t.Fatalf("Failed to create fake server: %v", err)
	}
	go func() {
		if err := fakeServer.Serve(lis); err != nil {
			t.Logf("Fake server exited: %v", err)
		}
	}()

	// Wait for fake server to be up.
	for {
		_, err = runtimeClient.Version(ctx, &runtimeapi.VersionRequest{})
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Failed to connect to fake server in time: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Log("Successfully connected to fake server")

	// 3. Stop fake runtime and check that we get an error
	fakeServer.Stop()

	for {
		_, err = runtimeClient.Version(ctx, &runtimeapi.VersionRequest{})
		if err != nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("Expected error after fake server stopped, got nil in time")
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Logf("Got expected error after fake server stopped: %v", err)

	// 4. Start fake runtime again and check that we can connect
	fakeServer, lis, _, err = fake.NewServer(fakeRuntimeSocket)
	if err != nil {
		t.Fatalf("Failed to create fake server: %v", err)
	}
	go func() {
		if err := fakeServer.Serve(lis); err != nil {
			t.Logf("Fake server exited: %v", err)
		}
	}()
	defer fakeServer.Stop()

	for {
		_, err = runtimeClient.Version(ctx, &runtimeapi.VersionRequest{})
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Failed to reconnect to fake server in time: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Log("Successfully reconnected to fake server")
}