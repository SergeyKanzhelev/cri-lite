// Package proxy_test provides tests for the proxy package.
package proxy_test

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
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

func (s *fakeRuntimeService) GetContainerEvents(req *runtimeapi.GetEventsRequest, stream runtimeapi.RuntimeService_GetContainerEventsServer) error {
	events := []*runtimeapi.ContainerEventResponse{
		{ContainerId: "container1", ContainerEventType: runtimeapi.ContainerEventType_CONTAINER_CREATED_EVENT},
		{ContainerId: "container2", ContainerEventType: runtimeapi.ContainerEventType_CONTAINER_STARTED_EVENT},
	}
	for _, event := range events {
		if err := stream.Send(event); err != nil {
			return err
		}
	}

	return nil
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

	sockDir := t.TempDir()

	defer func() {
		if err := os.RemoveAll(sockDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	fakeRuntimeSocket := sockDir + "/fake-runtime.sock"
	proxySocket := sockDir + "/proxy.sock"

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
		dialer := &net.Dialer{Timeout: 10 * time.Millisecond}

		conn, err := dialer.DialContext(ctx, "unix", proxySocket)
		if err == nil {
			if err := conn.Close(); err != nil {
				t.Logf("Failed to close connection: %v", err)
			}

			break
		}

		select {
		case <-ctx.Done():
			t.Fatalf("Proxy server did not start in time: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}

	conn, err := grpc.NewClient("unix://"+proxySocket, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithConnectParams(grpc.ConnectParams{
		Backoff:           backoff.DefaultConfig,
		MinConnectTimeout: 250 * time.Millisecond,
	}))
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("Failed to close connection: %v", err)
		}
	}()

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

type metadataCapturingFakeRuntimeService struct {
	fakeRuntimeService

	md metadata.MD
}

func (s *metadataCapturingFakeRuntimeService) Version(ctx context.Context, req *runtimeapi.VersionRequest) (*runtimeapi.VersionResponse, error) {
	s.md, _ = metadata.FromIncomingContext(ctx)

	return s.fakeRuntimeService.Version(ctx, req)
}

func TestMetadataPropagation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// 1. Backend setup
	backendLis := bufconn.Listen(bufSize)
	backendGrpcServer := grpc.NewServer()
	fakeRuntime := &metadataCapturingFakeRuntimeService{}
	runtimeapi.RegisterRuntimeServiceServer(backendGrpcServer, fakeRuntime)

	go func() {
		if err := backendGrpcServer.Serve(backendLis); err != nil {
			t.Errorf("Backend server exited with error: %v", err)
		}
	}()

	defer backendGrpcServer.Stop()

	// 2. Proxy setup
	backendConn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return backendLis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial backend bufnet: %v", err)
	}

	defer func() {
		if err := backendConn.Close(); err != nil {
			t.Logf("Failed to close backend connection: %v", err)
		}
	}()

	proxyServer := &proxy.Server{}
	p := policy.NewReadOnlyPolicy()
	proxyServer.SetPolicy(p)
	proxyServer.SetRuntimeClient(runtimeapi.NewRuntimeServiceClient(backendConn))
	proxyServer.SetImageClient(runtimeapi.NewImageServiceClient(backendConn))

	proxyLis := bufconn.Listen(bufSize)
	proxyGrpcServer := grpc.NewServer(grpc.UnaryInterceptor(p.UnaryInterceptor()))
	runtimeapi.RegisterRuntimeServiceServer(proxyGrpcServer, proxyServer)

	go func() {
		if err := proxyGrpcServer.Serve(proxyLis); err != nil {
			t.Errorf("Proxy server exited with error: %v", err)
		}
	}()

	defer proxyGrpcServer.Stop()

	// 3. Client setup
	testUserAgent := "my-test-client/1.0"

	proxyConn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return proxyLis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUserAgent(testUserAgent))
	if err != nil {
		t.Fatalf("Failed to dial proxy bufnet: %v", err)
	}

	defer func() {
		if err := proxyConn.Close(); err != nil {
			t.Logf("Failed to close proxy connection: %v", err)
		}
	}()

	runtimeClient := runtimeapi.NewRuntimeServiceClient(proxyConn)

	// 4. The actual test
	md := metadata.Pairs("baggage", "my-baggage")
	ctx = metadata.NewOutgoingContext(ctx, md)

	_, err = runtimeClient.Version(ctx, &runtimeapi.VersionRequest{})
	if err != nil {
		t.Fatalf("Version failed: %v", err)
	}

	if len(fakeRuntime.md.Get("x-forwarded-user-agent")) == 0 || !strings.Contains(fakeRuntime.md.Get("x-forwarded-user-agent")[0], testUserAgent) {
		t.Errorf("x-forwarded-user-agent not propagated correctly, got: %v", fakeRuntime.md.Get("x-forwarded-user-agent"))
	}
	// TODO: since this test is not using the NewServer() codepath, the user-agent
	// is not being set by default. Re-enable this check once we switch to using
	// NewServer() in tests.
	// if len(fakeRuntime.md.Get("user-agent")) == 0 || !strings.Contains(fakeRuntime.md.Get("user-agent")[0], "cri-lite/") {
	//	t.Errorf("user-agent not set correctly, got: %v", fakeRuntime.md.Get("user-agent"))
	//}
	if len(fakeRuntime.md.Get("baggage")) == 0 || fakeRuntime.md.Get("baggage")[0] != "my-baggage" {
		t.Errorf("baggage not propagated, got: %v", fakeRuntime.md.Get("baggage"))
	}
}

func TestGetContainerEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// 1. Backend setup
	backendLis := bufconn.Listen(bufSize)
	backendGrpcServer := grpc.NewServer()
	fakeRuntime := &fakeRuntimeService{}
	runtimeapi.RegisterRuntimeServiceServer(backendGrpcServer, fakeRuntime)

	go func() {
		if err := backendGrpcServer.Serve(backendLis); err != nil {
			t.Errorf("Backend server exited with error: %v", err)
		}
	}()

	defer backendGrpcServer.Stop()

	// 2. Proxy setup
	backendConn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return backendLis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial backend bufnet: %v", err)
	}

	defer func() {
		if err := backendConn.Close(); err != nil {
			t.Logf("Failed to close backend connection: %v", err)
		}
	}()

	proxyServer := &proxy.Server{}
	p := policy.NewReadOnlyPolicy()
	proxyServer.SetPolicy(p)
	proxyServer.SetRuntimeClient(runtimeapi.NewRuntimeServiceClient(backendConn))
	proxyServer.SetImageClient(runtimeapi.NewImageServiceClient(backendConn))

	proxyLis := bufconn.Listen(bufSize)
	proxyGrpcServer := grpc.NewServer(grpc.UnaryInterceptor(p.UnaryInterceptor()), grpc.StreamInterceptor(p.StreamInterceptor()))
	runtimeapi.RegisterRuntimeServiceServer(proxyGrpcServer, proxyServer)

	go func() {
		if err := proxyGrpcServer.Serve(proxyLis); err != nil {
			t.Errorf("Proxy server exited with error: %v", err)
		}
	}()

	defer proxyGrpcServer.Stop()

	// 3. Client setup
	proxyConn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return proxyLis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial proxy bufnet: %v", err)
	}

	defer func() {
		if err := proxyConn.Close(); err != nil {
			t.Logf("Failed to close proxy connection: %v", err)
		}
	}()

	runtimeClient := runtimeapi.NewRuntimeServiceClient(proxyConn)

	// 4. The actual test
	stream, err := runtimeClient.GetContainerEvents(ctx, &runtimeapi.GetEventsRequest{})
	if err != nil {
		t.Fatalf("GetContainerEvents failed: %v", err)
	}

	expectedEvents := []*runtimeapi.ContainerEventResponse{
		{ContainerId: "container1", ContainerEventType: runtimeapi.ContainerEventType_CONTAINER_CREATED_EVENT},
		{ContainerId: "container2", ContainerEventType: runtimeapi.ContainerEventType_CONTAINER_STARTED_EVENT},
	}

	for _, expected := range expectedEvents {
		event, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}

		if event.GetContainerId() != expected.GetContainerId() {
			t.Errorf("expected container id %s, got %s", expected.GetContainerId(), event.GetContainerId())
		}

		if event.GetContainerEventType() != expected.GetContainerEventType() {
			t.Errorf("expected event type %v, got %v", expected.GetContainerEventType(), event.GetContainerEventType())
		}
	}

	_, err = stream.Recv()
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF, got %v", err)
	}
}
