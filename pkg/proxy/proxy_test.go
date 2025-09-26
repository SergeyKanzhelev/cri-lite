package proxy_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"cri-lite/pkg/fake"
	"cri-lite/pkg/policy"
	"cri-lite/pkg/proxy"
)



func TestProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Proxy Suite")
}

var _ = Describe("Proxy", func() {
	var (
		server        *grpc.Server
		proxyServer   *proxy.Server
		client        runtimeapi.RuntimeServiceClient
		err           error
		proxySocket   string
		serverSocket  string
		serverAddress string
		sockDir       string
	)

	BeforeEach(func() {
		sockDir, err = os.MkdirTemp("", "cri-lite-test")
		Expect(err).NotTo(HaveOccurred())
		serverSocket = createSocket(sockDir)
		proxySocket = createSocket(sockDir)
		serverAddress = fmt.Sprintf("unix://%s", serverSocket)

		// Start fake server
		var lis net.Listener
		server, lis, err = fake.NewServer(serverSocket)
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer GinkgoRecover()
			Expect(server.Serve(lis)).To(Succeed())
		}()

		// Create policy
		p, err := policy.NewFromConfigData(&policy.Config{ReadOnly: true})
		Expect(err).NotTo(HaveOccurred())

		// Start proxy
		proxyServer, err = proxy.NewServer(serverAddress, serverAddress)
		Expect(err).NotTo(HaveOccurred())
		proxyServer.SetPolicies([]policy.Policy{p})
		go func() {
			defer GinkgoRecover()
			Expect(proxyServer.Start(proxySocket)).To(Succeed())
		}()

		// Wait for the proxy to be ready
		Eventually(func() error {
			conn, err := net.Dial("unix", proxySocket)
			if err != nil {
				return err
			}
			conn.Close()
			return nil
		}, "5s", "100ms").Should(Succeed())

		// Create client
		conn, err := grpc.NewClient(
			"unix://"+proxySocket,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		Expect(err).NotTo(HaveOccurred())
		client = runtimeapi.NewRuntimeServiceClient(conn)
	})

	AfterEach(func() {
		if server != nil {
			server.Stop()
		}
		// if proxyServer != nil {
		// 	proxyServer.Stop()
		// }
		if sockDir != "" {
			Expect(os.RemoveAll(sockDir)).To(Succeed())
		}
	})

	Context("with readonly policy", func() {
		It("should allow readonly calls and deny write calls", func() {
			By("calling a readonly method")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := client.Version(ctx, &runtimeapi.VersionRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling a write method")
			_, err = client.RunPodSandbox(ctx, &runtimeapi.RunPodSandboxRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))
		})
	})
})

func createSocket(sockDir string) string {
	f, err := os.CreateTemp(sockDir, "socket-*.sock")
	Expect(err).NotTo(HaveOccurred())
	path := f.Name()
	Expect(f.Close()).To(Succeed())
	Expect(os.Remove(path)).To(Succeed())
	return path
}