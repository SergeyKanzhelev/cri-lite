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

var _ = Describe("PodScoped Policy", func() {
	var (
		server            *grpc.Server
		proxyServer       *proxy.Server
		runtimeClient     runtimeapi.RuntimeServiceClient
		imageClient       runtimeapi.ImageServiceClient
		err               error
		proxySocket       string
		serverSocket      string
		serverAddress     string
		sockDir           string
		podSandboxID      = "test-sandbox-id"
		otherPodSandboxID = "other-sandbox-id"
	)

	BeforeEach(func() {
		sockDir, err = os.MkdirTemp("", "cri-lite-test")
		Expect(err).NotTo(HaveOccurred())
		serverSocket = createSocket(sockDir)
		proxySocket = createSocket(sockDir)
		serverAddress = "unix://" + serverSocket

		// Start fake server
		var lis net.Listener
		server, lis, err = fake.NewServer(serverSocket)
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer GinkgoRecover()
			Expect(server.Serve(lis)).To(Succeed())
		}()

		// Create proxy server instance
		proxyServer, err = proxy.NewServer(serverAddress, serverAddress)
		Expect(err).NotTo(HaveOccurred())

		// Create clients
		conn, err := grpc.NewClient(
			"unix://"+proxySocket,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		Expect(err).NotTo(HaveOccurred())
		runtimeClient = runtimeapi.NewRuntimeServiceClient(conn)
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

	Context("with static pod sandbox ID", func() {
		BeforeEach(func() {
			// Create and set the policy
			p := policy.NewPodScopedPolicy(podSandboxID, false, proxyServer.GetRuntimeClient())
			proxyServer.SetPolicies([]policy.Policy{p})

			// Start the proxy server
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
		})

		It("should allow correctly scoped calls and deny others", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			By("calling an image method (allowed)")
			_, err := imageClient.ListImages(ctx, &runtimeapi.ListImagesRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling a non-scoped runtime method (allowed)")
			_, err = runtimeClient.Version(ctx, &runtimeapi.VersionRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling a correctly scoped runtime method (allowed)")
			_, err = runtimeClient.RunPodSandbox(ctx, &runtimeapi.RunPodSandboxRequest{
				Config: &runtimeapi.PodSandboxConfig{
					Metadata: &runtimeapi.PodSandboxMetadata{
						Name:      "test-sandbox",
						Namespace: "test-namespace",
						Uid:       "test-uid",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("calling an incorrectly scoped runtime method (denied)")
			_, err = runtimeClient.PortForward(ctx, &runtimeapi.PortForwardRequest{
				PodSandboxId: otherPodSandboxID,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))
		})
	})
})
