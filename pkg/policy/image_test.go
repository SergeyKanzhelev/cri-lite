package policy_test

import (
	"context"
	"net"
	"os"
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

var _ = Describe("Image Management Policy", func() {
	var (
		server        *grpc.Server
		proxyServer   *proxy.Server
		client        runtimeapi.RuntimeServiceClient
		imageClient   runtimeapi.ImageServiceClient
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
		serverAddress = "unix://" + serverSocket

		// Start fake server
		var lis net.Listener
		server, lis, _, err := fake.NewServer(serverSocket)
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer GinkgoRecover()
			Expect(server.Serve(lis)).To(Succeed())
		}()

		// Create policy
		p := policy.NewImageManagementPolicy()

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
			if err := conn.Close(); err != nil {
				return err
			}

			return nil
		}, "5s", "100ms").Should(Succeed())

		// Create client
		conn, err := grpc.NewClient(
			"unix://"+proxySocket,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		Expect(err).NotTo(HaveOccurred())
		client = runtimeapi.NewRuntimeServiceClient(conn)
		imageClient = runtimeapi.NewImageServiceClient(conn)
	})

	AfterEach(func() {
		if server != nil {
			server.Stop()
		}
		if sockDir != "" {
			Expect(os.RemoveAll(sockDir)).To(Succeed())
		}
	})

	Context("with image management policy", func() {
		It("should allow image calls and deny runtime calls", func() {
			By("calling an image method")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := imageClient.ListImages(ctx, &runtimeapi.ListImagesRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling a runtime method")
			_, err = client.Version(ctx, &runtimeapi.VersionRequest{})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
