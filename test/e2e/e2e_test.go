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

	It("should be able to stop other containers in the same pod", func() {
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

		By("creating a victim container")
		victimContainerReq := &runtimeapi.CreateContainerRequest{
			PodSandboxId: podSandboxID,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "victim-container",
				},
				Image: &runtimeapi.ImageSpec{
					Image: "busybox",
				},
				Command: []string{"/bin/sleep", "3600"},
			},
			SandboxConfig: req.GetConfig(),
		}
		victimContainerResp, err := realRuntimeClient.CreateContainer(ctx, victimContainerReq)
		Expect(err).NotTo(HaveOccurred())
		victimContainerID := victimContainerResp.GetContainerId()

		defer func() {
			By("cleaning up victim container")
			// Don't check for error, container might be already stopped.
			_, _ = realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: victimContainerID})
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: victimContainerID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting the victim container")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: victimContainerID})
		Expect(err).NotTo(HaveOccurred())

		By("creating an attacker container")
		crictlPath, err := filepath.Abs("../../crictl")
		Expect(err).NotTo(HaveOccurred())

		attackerContainerReq := &runtimeapi.CreateContainerRequest{
			PodSandboxId: podSandboxID,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "attacker-container",
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
		attackerContainerResp, err := realRuntimeClient.CreateContainer(ctx, attackerContainerReq)
		Expect(err).NotTo(HaveOccurred())
		attackerContainerID := attackerContainerResp.GetContainerId()

		defer func() {
			By("cleaning up attacker container")
			_, err := realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: attackerContainerID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: attackerContainerID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting the attacker container")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: attackerContainerID})
		Expect(err).NotTo(HaveOccurred())

		By("execing into the container to run crictl")
		execReq := &runtimeapi.ExecSyncRequest{
			ContainerId: attackerContainerID,
			Cmd:         []string{"/crictl", "--runtime-endpoint", "unix:///proxy.sock", "stop", victimContainerID},
			Timeout:     10,
		}
		execResp, err := realRuntimeClient.ExecSync(ctx, execReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(execResp.GetExitCode()).To(BeZero())

		By("verifying the victim container is stopped")
		statusReq := &runtimeapi.ContainerStatusRequest{
			ContainerId: victimContainerID,
		}
		statusResp, err := realRuntimeClient.ContainerStatus(ctx, statusReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(statusResp.GetStatus().GetState()).To(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))
	})

	It("should not be able to stop containers in other pods", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		By("creating a victim pod sandbox")
		victimReq := &runtimeapi.RunPodSandboxRequest{
			Config: &runtimeapi.PodSandboxConfig{
				Metadata: &runtimeapi.PodSandboxMetadata{
					Name:      "victim-sandbox-" + framework.RandomSuffix(),
					Namespace: "test-namespace",
					Uid:       "test-uid-" + framework.RandomSuffix(),
				},
			},
		}
		victimResp, err := realRuntimeClient.RunPodSandbox(ctx, victimReq)
		Expect(err).NotTo(HaveOccurred())
		victimPodSandboxID := victimResp.GetPodSandboxId()
		GinkgoLogr.Info("created victim pod sandbox", "id", victimPodSandboxID)

		defer func() {
			By("cleaning up victim pod sandbox")
			_, err := realRuntimeClient.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{PodSandboxId: victimPodSandboxID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: victimPodSandboxID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("pulling the busybox image")
		_, err = realImageClient.PullImage(ctx, &runtimeapi.PullImageRequest{
			Image: &runtimeapi.ImageSpec{
				Image: "busybox",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("creating a victim container")
		victimContainerReq := &runtimeapi.CreateContainerRequest{
			PodSandboxId: victimPodSandboxID,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "victim-container",
				},
				Image: &runtimeapi.ImageSpec{
					Image: "busybox",
				},
				Command: []string{"/bin/sleep", "3600"},
			},
			SandboxConfig: victimReq.GetConfig(),
		}
		victimContainerResp, err := realRuntimeClient.CreateContainer(ctx, victimContainerReq)
		Expect(err).NotTo(HaveOccurred())
		victimContainerID := victimContainerResp.GetContainerId()

		defer func() {
			By("cleaning up victim container")
			// Don't check for error, container might be already stopped.
			_, _ = realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: victimContainerID})
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: victimContainerID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting the victim container")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: victimContainerID})
		Expect(err).NotTo(HaveOccurred())

		By("creating an attacker pod sandbox")
		attackerReq := &runtimeapi.RunPodSandboxRequest{
			Config: &runtimeapi.PodSandboxConfig{
				Metadata: &runtimeapi.PodSandboxMetadata{
					Name:      "attacker-sandbox-" + framework.RandomSuffix(),
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
		attackerResp, err := realRuntimeClient.RunPodSandbox(ctx, attackerReq)
		Expect(err).NotTo(HaveOccurred())
		attackerPodSandboxID := attackerResp.GetPodSandboxId()
		GinkgoLogr.Info("created attacker pod sandbox", "id", attackerPodSandboxID)

		defer func() {
			By("cleaning up attacker pod sandbox")
			_, err := realRuntimeClient.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{PodSandboxId: attackerPodSandboxID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: attackerPodSandboxID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("creating an attacker container")
		crictlPath, err := filepath.Abs("../../crictl")
		Expect(err).NotTo(HaveOccurred())

		attackerContainerReq := &runtimeapi.CreateContainerRequest{
			PodSandboxId: attackerPodSandboxID,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "attacker-container",
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
			SandboxConfig: attackerReq.GetConfig(),
		}
		attackerContainerResp, err := realRuntimeClient.CreateContainer(ctx, attackerContainerReq)
		Expect(err).NotTo(HaveOccurred())
		attackerContainerID := attackerContainerResp.GetContainerId()

		defer func() {
			By("cleaning up attacker container")
			_, err := realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: attackerContainerID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: attackerContainerID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting the attacker container")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: attackerContainerID})
		Expect(err).NotTo(HaveOccurred())

		By("execing into the container to run crictl")
		execReq := &runtimeapi.ExecSyncRequest{
			ContainerId: attackerContainerID,
			Cmd:         []string{"/crictl", "--runtime-endpoint", "unix:///proxy.sock", "stop", victimContainerID},
			Timeout:     10,
		}
		execResp, err := realRuntimeClient.ExecSync(ctx, execReq)
		GinkgoLogr.Info("crictl stdout", "stdout", string(execResp.GetStdout()))
		GinkgoLogr.Info("crictl stderr", "stderr", string(execResp.GetStderr()))
		Expect(string(execResp.GetStderr())).To(ContainSubstring("method not allowed by policy"))

		By("verifying the victim container is still running")
		statusReq := &runtimeapi.ContainerStatusRequest{
			ContainerId: victimContainerID,
		}
		statusResp, err := realRuntimeClient.ContainerStatus(ctx, statusReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(statusResp.GetStatus().GetState()).To(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))
	})
})
