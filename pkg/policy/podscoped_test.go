package policy_test

import (
	"context"
	"io"
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
		mock              *fake.Server
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
		server, lis, mock, err = fake.NewServer(serverSocket)
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
			proxyServer.SetPolicy(p)

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

			By("calling an image method (denied)")
			_, err := imageClient.ListImages(ctx, &runtimeapi.ListImagesRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling a non-scoped runtime method (allowed)")
			_, err = runtimeClient.Version(ctx, &runtimeapi.VersionRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ImageFsInfo (allowed)")
			_, err = imageClient.ImageFsInfo(ctx, &runtimeapi.ImageFsInfoRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling a correctly scoped runtime method (allowed)")
			_, err = runtimeClient.PodSandboxStatus(ctx, &runtimeapi.PodSandboxStatusRequest{
				PodSandboxId: podSandboxID,
			})
			Expect(err).NotTo(HaveOccurred())

			By("calling an incorrectly scoped runtime method (denied)")
			_, err = runtimeClient.PortForward(ctx, &runtimeapi.PortForwardRequest{
				PodSandboxId: otherPodSandboxID,
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))
		})

		It("should deny all image service calls", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			By("calling ListImages (denied)")
			_, err := imageClient.ListImages(ctx, &runtimeapi.ListImagesRequest{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))

			By("calling PullImage (denied)")
			_, err = imageClient.PullImage(ctx, &runtimeapi.PullImageRequest{Image: &runtimeapi.ImageSpec{Image: "busybox"}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("method not allowed by policy"))
		})
	})

	Context("with container list filtering", func() {
		var (
			containerID1 = "container-id-1"
			containerID2 = "container-id-2"
		)
		BeforeEach(func() {
			// Create and set the policy
			p := policy.NewPodScopedPolicy(podSandboxID, false, proxyServer.GetRuntimeClient())
			proxyServer.SetPolicy(p)

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

			// Create containers in the fake runtime
			mock.SetContainers([]*runtimeapi.Container{
				{
					Id:           containerID1,
					PodSandboxId: podSandboxID,
					Metadata: &runtimeapi.ContainerMetadata{
						Name: "container-1",
					},
				},
				{
					Id:           containerID2,
					PodSandboxId: otherPodSandboxID,
					Metadata: &runtimeapi.ContainerMetadata{
						Name: "container-2",
					},
				},
			})
			mock.SetContainerStats([]*runtimeapi.ContainerStats{
				{
					Attributes: &runtimeapi.ContainerAttributes{
						Id: containerID1,
						Metadata: &runtimeapi.ContainerMetadata{
							Name: "container-1",
						},
					},
				},
				{
					Attributes: &runtimeapi.ContainerAttributes{
						Id: containerID2,
						Metadata: &runtimeapi.ContainerMetadata{
							Name: "container-2",
						},
					},
				},
			})
			mock.SetPodSandboxStats([]*runtimeapi.PodSandboxStats{
				{
					Attributes: &runtimeapi.PodSandboxAttributes{
						Id: podSandboxID,
						Metadata: &runtimeapi.PodSandboxMetadata{
							Name: "container-1",
						},
					},
				},
				{
					Attributes: &runtimeapi.PodSandboxAttributes{
						Id: otherPodSandboxID,
						Metadata: &runtimeapi.PodSandboxMetadata{
							Name: "container-2",
						},
					},
				},
			})
		})

		It("should filter ListContainers when runtime returns extra containers", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			resp, err := runtimeClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.GetContainers()).To(HaveLen(1))
			Expect(resp.GetContainers()[0].GetId()).To(Equal(containerID1))
		})

		It("should filter ListContainerStats", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			resp, err := runtimeClient.ListContainerStats(ctx, &runtimeapi.ListContainerStatsRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.GetStats()).To(HaveLen(1))
			Expect(resp.GetStats()[0].GetAttributes().GetMetadata().GetName()).To(Equal("container-1"))
		})

		It("should filter ListPodSandboxStats", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			resp, err := runtimeClient.ListPodSandboxStats(ctx, &runtimeapi.ListPodSandboxStatsRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.GetStats()).To(HaveLen(1))
			Expect(resp.GetStats()[0].GetAttributes().GetMetadata().GetName()).To(Equal("container-1"))
		})

		It("should not filter ListContainers when runtime respects the filter", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			resp, err := runtimeClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{
				Filter: &runtimeapi.ContainerFilter{
					PodSandboxId: podSandboxID,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.GetContainers()).To(HaveLen(1))
			Expect(resp.GetContainers()[0].GetId()).To(Equal(containerID1))
		})

		It("should filter GetContainerEvents to only return events for the specific Pod", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			// Set up fake events in the mock server
			mock.SetEmittedEvents([]*runtimeapi.ContainerEventResponse{
				{ContainerId: containerID1, ContainerEventType: runtimeapi.ContainerEventType_CONTAINER_CREATED_EVENT},
				{ContainerId: containerID2, ContainerEventType: runtimeapi.ContainerEventType_CONTAINER_STARTED_EVENT},
			})

			stream, err := runtimeClient.GetContainerEvents(ctx, &runtimeapi.GetEventsRequest{})
			Expect(err).NotTo(HaveOccurred())

			// Expect only the event for podSandboxID
			event, err := stream.Recv()
			Expect(err).NotTo(HaveOccurred())
			Expect(event.GetContainerId()).To(Equal(containerID1))

			// Expect no more events
			_, err = stream.Recv()
			Expect(err).To(MatchError(io.EOF))
		})
	})
})
