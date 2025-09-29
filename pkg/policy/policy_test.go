package policy_test

import (
	"context"
	"net"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"cri-lite/pkg/fake"
	"cri-lite/pkg/policy"
	"cri-lite/pkg/proxy"
)

func TestPolicy(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Policy Suite")
}

func createSocket(sockDir string) string {
	f, err := os.CreateTemp(sockDir, "socket-*.sock")
	Expect(err).NotTo(HaveOccurred())

	path := f.Name()
	Expect(f.Close()).To(Succeed())
	Expect(os.Remove(path)).To(Succeed())

	return path
}

func setupTestEnvironment(p policy.Policy) (
	runtimeapi.RuntimeServiceClient,
	runtimeapi.ImageServiceClient,
	func(),
) {
	sockDir, err := os.MkdirTemp("", "cri-lite-test")
	Expect(err).NotTo(HaveOccurred())

	serverSocket := createSocket(sockDir)
	proxySocket := createSocket(sockDir)
	serverAddress := "unix://" + serverSocket

	server := startFakeServer(serverSocket)

	startProxyServer(proxySocket, serverAddress, p)
	runtimeClient, imageClient := createClients(proxySocket)

	cleanup := func() {
		if server != nil {
			server.Stop()
		}

		if sockDir != "" {
			Expect(os.RemoveAll(sockDir)).To(Succeed())
		}
	}

	return runtimeClient, imageClient, cleanup
}

func startFakeServer(serverSocket string) *grpc.Server {
	var lis net.Listener

	server, lis, _, err := fake.NewServer(serverSocket)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()

		Expect(server.Serve(lis)).To(Succeed())
	}()

	return server
}

func startProxyServer(proxySocket, serverAddress string, p policy.Policy) {
	proxyServer, err := proxy.NewServer(serverAddress, serverAddress)
	Expect(err).NotTo(HaveOccurred())
	proxyServer.SetPolicy(p)

	go func() {
		defer GinkgoRecover()

		Expect(proxyServer.Start(proxySocket)).To(Succeed())
	}()

	// Wait for the proxy to be ready
	Eventually(func() error {
		dialer := &net.Dialer{}

		conn, err := dialer.DialContext(
			context.Background(),
			"unix",
			proxySocket,
		)
		if err != nil {
			return err
		}

		if err := conn.Close(); err != nil {
			return err
		}

		return nil
	}, "5s", "100ms").Should(Succeed())
}

func createClients(proxySocket string) (runtimeapi.RuntimeServiceClient, runtimeapi.ImageServiceClient) {
	conn, err := grpc.NewClient(
		"unix://"+proxySocket,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	Expect(err).NotTo(HaveOccurred())

	runtimeClient := runtimeapi.NewRuntimeServiceClient(conn)
	imageClient := runtimeapi.NewImageServiceClient(conn)

	return runtimeClient, imageClient
}
