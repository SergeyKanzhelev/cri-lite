package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"cri-lite/test/framework"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = Describe("cri-lite E2E", func() {
	var (
		f                 *framework.Framework
		realRuntimeClient runtimeapi.RuntimeServiceClient
		realImageClient   runtimeapi.ImageServiceClient
	)

	BeforeEach(func() {
		if os.Getenv("SUDO_UID") == "" {
			Skip("Skipping E2E test: must be run with sudo")
		}

		var err error
		f, err = framework.New()
		Expect(err).NotTo(HaveOccurred())

		err = f.SetupProxy()
		Expect(err).NotTo(HaveOccurred())

		realRuntimeClient, err = f.GetRealRuntimeClient()
		Expect(err).NotTo(HaveOccurred())

		realImageClient, err = f.GetRealImageClient()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		f.TeardownProxy()
	})

	It("should enforce pod-scoped policy", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		By("creating a pod sandbox")
		req := &runtimeapi.RunPodSandboxRequest{
			Config: &runtimeapi.PodSandboxConfig{
				Metadata: &runtimeapi.PodSandboxMetadata{
					Name:      "test-sandbox-" + framework.RandomSuffix(),
					Namespace: "test-namespace",
					Uid:       "test-uid-" + framework.RandomSuffix(),
				},
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
						Privileged: true,
					},
				},
			},
		}
		resp, err := realRuntimeClient.RunPodSandbox(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		podSandboxID := resp.GetPodSandboxId()
		GinkgoLogr.Info("created pod sandbox", "id", podSandboxID)

		defer func() {
			By("cleaning up pod sandbox")
			_, err := realRuntimeClient.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{PodSandboxId: podSandboxID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: podSandboxID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("pulling the busybox image")
		_, err = realImageClient.PullImage(ctx, &runtimeapi.PullImageRequest{
			Image: &runtimeapi.ImageSpec{
				Image: "busybox",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("creating a container")
		crictlPath, err := filepath.Abs("../../crictl")
		Expect(err).NotTo(HaveOccurred())

		containerReq := &runtimeapi.CreateContainerRequest{
			PodSandboxId: podSandboxID,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "test-container",
				},
				Image: &runtimeapi.ImageSpec{
					Image: "busybox",
				},
				Command: []string{"/bin/sleep", "3600"},
				Mounts: []*runtimeapi.Mount{
					{
						HostPath:      crictlPath,
						ContainerPath: "/crictl",
					},
					{
						HostPath:      f.ProxySocket,
						ContainerPath: "/proxy.sock",
					},
				},
			},
			SandboxConfig: req.GetConfig(),
		}
		containerResp, err := realRuntimeClient.CreateContainer(ctx, containerReq)
		Expect(err).NotTo(HaveOccurred())
		containerID := containerResp.GetContainerId()

		defer func() {
			By("cleaning up container")
			_, err := realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: containerID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: containerID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting the container")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: containerID})
		Expect(err).NotTo(HaveOccurred())

		By("execing into the container to run crictl")
		execReq := &runtimeapi.ExecSyncRequest{
			ContainerId: containerID,
			Cmd:         []string{"/crictl", "--runtime-endpoint", "unix:///proxy.sock", "version"},
			Timeout:     10,
		}
		execResp, err := realRuntimeClient.ExecSync(ctx, execReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(execResp.GetExitCode()).To(BeZero())
	})
})
