// Package proxy_test provides tests for the proxy package.
package proxy_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"cri-lite/pkg/policy"
	"cri-lite/pkg/proxy"
	"cri-lite/pkg/version"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

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
	ctx := context.Background()
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	runtimeapi.RegisterRuntimeServiceServer(s, &fakeRuntimeService{})

	go func() {
		err := s.Serve(lis)
		if err != nil {
			t.Errorf("Server exited with error: %v", err)

			return
		}
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	proxyServer.SetPolicies([]policy.Policy{policy.NewReadOnlyPolicy()})
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
