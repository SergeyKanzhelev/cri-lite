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

var _ = Describe("ReadOnly Policy", func() {
	var (
		server        *grpc.Server
		proxyServer   *proxy.Server
		runtimeClient runtimeapi.RuntimeServiceClient
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

	Context("with readonly policy", func() {
		It("should allow readonly calls and deny write calls", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			// Read-only runtime methods
			By("calling Version")
			_, err = runtimeClient.Version(ctx, &runtimeapi.VersionRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling Status")
			_, err = runtimeClient.Status(ctx, &runtimeapi.StatusRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ListContainers")
			_, err = runtimeClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ContainerStatus")
			_, err = runtimeClient.ContainerStatus(ctx, &runtimeapi.ContainerStatusRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ListPodSandbox")
			_, err = runtimeClient.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling PodSandboxStatus")
			_, err = runtimeClient.PodSandboxStatus(ctx, &runtimeapi.PodSandboxStatusRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ContainerStats")
			_, err = runtimeClient.ContainerStats(ctx, &runtimeapi.ContainerStatsRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ListContainerStats")
			_, err = runtimeClient.ListContainerStats(ctx, &runtimeapi.ListContainerStatsRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling PodSandboxStats")
			_, err = runtimeClient.PodSandboxStats(ctx, &runtimeapi.PodSandboxStatsRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ListPodSandboxStats")
			_, err = runtimeClient.ListPodSandboxStats(ctx, &runtimeapi.ListPodSandboxStatsRequest{})
			Expect(err).NotTo(HaveOccurred())

			// Read-only image methods
			By("calling ListImages")
			_, err = imageClient.ListImages(ctx, &runtimeapi.ListImagesRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ImageStatus")
			_, err = imageClient.ImageStatus(ctx, &runtimeapi.ImageStatusRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ImageFsInfo")
			_, err = imageClient.ImageFsInfo(ctx, &runtimeapi.ImageFsInfoRequest{})
			Expect(err).NotTo(HaveOccurred())

			// Write runtime methods
			By("calling RunPodSandbox")
			_, err = runtimeClient.RunPodSandbox(ctx, &runtimeapi.RunPodSandboxRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling StopPodSandbox")
			_, err = runtimeClient.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling RemovePodSandbox")
			_, err = runtimeClient.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling CreateContainer")
			_, err = runtimeClient.CreateContainer(ctx, &runtimeapi.CreateContainerRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling StartContainer")
			_, err = runtimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling StopContainer")
			_, err = runtimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling RemoveContainer")
			_, err = runtimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling UpdateContainerResources")
			_, err = runtimeClient.UpdateContainerResources(ctx, &runtimeapi.UpdateContainerResourcesRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling ExecSync")
			_, err = runtimeClient.ExecSync(ctx, &runtimeapi.ExecSyncRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling Exec")
			_, err = runtimeClient.Exec(ctx, &runtimeapi.ExecRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling Attach")
			_, err = runtimeClient.Attach(ctx, &runtimeapi.AttachRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling PortForward")
			_, err = runtimeClient.PortForward(ctx, &runtimeapi.PortForwardRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling UpdateRuntimeConfig")
			_, err = runtimeClient.UpdateRuntimeConfig(ctx, &runtimeapi.UpdateRuntimeConfigRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			// Write image methods
			By("calling PullImage")
			_, err = imageClient.PullImage(ctx, &runtimeapi.PullImageRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling RemoveImage")
			_, err = imageClient.RemoveImage(ctx, &runtimeapi.RemoveImageRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))
		})
	})
})
