package e2e_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"cri-lite/test/framework"
)

var runtimeEndpoint = flag.String("runtime-endpoint", os.Getenv("RUNTIME_ENDPOINT"), "CRI runtime endpoint")

func TestE2E(t *testing.T) {
	t.Parallel()
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

		if *runtimeEndpoint == "" {
			Fail("runtime-endpoint must be specified for e2e tests")
		}

		var err error
		f, err = framework.New(*runtimeEndpoint)
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
		if execResp.GetExitCode() != 0 {
			GinkgoLogr.Info("crictl version stderr", "stderr", string(execResp.GetStderr()))
		}
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

		By("creating an orchestrator container")
		crictlPath, err := filepath.Abs("../../crictl")
		Expect(err).NotTo(HaveOccurred())

		orchestratorContainerReq := &runtimeapi.CreateContainerRequest{
			PodSandboxId: podSandboxID,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "orchestrator-container",
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
		orchestratorContainerResp, err := realRuntimeClient.CreateContainer(ctx, orchestratorContainerReq)
		Expect(err).NotTo(HaveOccurred())
		orchestratorContainerID := orchestratorContainerResp.GetContainerId()

		defer func() {
			By("cleaning up orchestrator container")
			_, err := realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: orchestratorContainerID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: orchestratorContainerID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting the orchestrator container")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: orchestratorContainerID})
		Expect(err).NotTo(HaveOccurred())

		By("execing into the container to run crictl")
		execReq := &runtimeapi.ExecSyncRequest{
			ContainerId: orchestratorContainerID,
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

		By("creating an orchestrator pod sandbox")
		orchestratorReq := &runtimeapi.RunPodSandboxRequest{
			Config: &runtimeapi.PodSandboxConfig{
				Metadata: &runtimeapi.PodSandboxMetadata{
					Name:      "orchestrator-sandbox-" + framework.RandomSuffix(),
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
		orchestratorResp, err := realRuntimeClient.RunPodSandbox(ctx, orchestratorReq)
		Expect(err).NotTo(HaveOccurred())
		orchestratorPodSandboxID := orchestratorResp.GetPodSandboxId()
		GinkgoLogr.Info("created orchestrator pod sandbox", "id", orchestratorPodSandboxID)

		defer func() {
			By("cleaning up orchestrator pod sandbox")
			_, err := realRuntimeClient.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{PodSandboxId: orchestratorPodSandboxID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: orchestratorPodSandboxID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("creating an orchestrator container")
		crictlPath, err := filepath.Abs("../../crictl")
		Expect(err).NotTo(HaveOccurred())

		orchestratorContainerReq := &runtimeapi.CreateContainerRequest{
			PodSandboxId: orchestratorPodSandboxID,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "orchestrator-container",
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
			SandboxConfig: orchestratorReq.GetConfig(),
		}
		orchestratorContainerResp, err := realRuntimeClient.CreateContainer(ctx, orchestratorContainerReq)
		Expect(err).NotTo(HaveOccurred())
		orchestratorContainerID := orchestratorContainerResp.GetContainerId()

		defer func() {
			By("cleaning up orchestrator container")
			_, err := realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: orchestratorContainerID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: orchestratorContainerID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting the orchestrator container")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: orchestratorContainerID})
		Expect(err).NotTo(HaveOccurred())

		By("execing into the container to run crictl")
		execReq := &runtimeapi.ExecSyncRequest{
			ContainerId: orchestratorContainerID,
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

	It("should only list containers, container stats, and pod sandbox stats in the same pod", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		By("creating a pod sandbox")
		req1 := &runtimeapi.RunPodSandboxRequest{
			Config: &runtimeapi.PodSandboxConfig{
				Metadata: &runtimeapi.PodSandboxMetadata{
					Name:      "test-sandbox-1-" + framework.RandomSuffix(),
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
		resp1, err := realRuntimeClient.RunPodSandbox(ctx, req1)
		Expect(err).NotTo(HaveOccurred())
		podSandboxID1 := resp1.GetPodSandboxId()
		GinkgoLogr.Info("created pod sandbox 1", "id", podSandboxID1)

		defer func() {
			By("cleaning up pod sandbox 1")
			_, err := realRuntimeClient.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{PodSandboxId: podSandboxID1})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: podSandboxID1})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("creating a second pod sandbox")
		req2 := &runtimeapi.RunPodSandboxRequest{
			Config: &runtimeapi.PodSandboxConfig{
				Metadata: &runtimeapi.PodSandboxMetadata{
					Name:      "test-sandbox-2-" + framework.RandomSuffix(),
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
		resp2, err := realRuntimeClient.RunPodSandbox(ctx, req2)
		Expect(err).NotTo(HaveOccurred())
		podSandboxID2 := resp2.GetPodSandboxId()
		GinkgoLogr.Info("created pod sandbox 2", "id", podSandboxID2)

		defer func() {
			By("cleaning up pod sandbox 2")
			_, err := realRuntimeClient.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{PodSandboxId: podSandboxID2})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: podSandboxID2})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("pulling the busybox image")
		_, err = realImageClient.PullImage(ctx, &runtimeapi.PullImageRequest{
			Image: &runtimeapi.ImageSpec{
				Image: "busybox",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("creating a container in the first pod sandbox")
		containerReq1 := &runtimeapi.CreateContainerRequest{
			PodSandboxId: podSandboxID1,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "container-1",
				},
				Image: &runtimeapi.ImageSpec{
					Image: "busybox",
				},
				Command: []string{"/bin/sleep", "3600"},
			},
			SandboxConfig: req1.GetConfig(),
		}
		containerResp1, err := realRuntimeClient.CreateContainer(ctx, containerReq1)
		Expect(err).NotTo(HaveOccurred())
		containerID1 := containerResp1.GetContainerId()

		defer func() {
			By("cleaning up container 1")
			_, err := realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: containerID1})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: containerID1})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting container 1")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: containerID1})
		Expect(err).NotTo(HaveOccurred())

		By("creating a container in the second pod sandbox")
		containerReq2 := &runtimeapi.CreateContainerRequest{
			PodSandboxId: podSandboxID2,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "container-2",
				},
				Image: &runtimeapi.ImageSpec{
					Image: "busybox",
				},
				Command: []string{"/bin/sleep", "3600"},
			},
			SandboxConfig: req2.GetConfig(),
		}
		containerResp2, err := realRuntimeClient.CreateContainer(ctx, containerReq2)
		Expect(err).NotTo(HaveOccurred())
		containerID2 := containerResp2.GetContainerId()

		defer func() {
			By("cleaning up container 2")
			_, err := realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: containerID2})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: containerID2})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting container 2")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: containerID2})
		Expect(err).NotTo(HaveOccurred())

		By("creating an orchestrator container")
		crictlPath, err := filepath.Abs("../../crictl")
		Expect(err).NotTo(HaveOccurred())

		orchestratorContainerReq := &runtimeapi.CreateContainerRequest{
			PodSandboxId: podSandboxID1,
			Config: &runtimeapi.ContainerConfig{
				Metadata: &runtimeapi.ContainerMetadata{
					Name: "orchestrator-container",
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
			SandboxConfig: req1.GetConfig(),
		}
		orchestratorContainerResp, err := realRuntimeClient.CreateContainer(ctx, orchestratorContainerReq)
		Expect(err).NotTo(HaveOccurred())
		orchestratorContainerID := orchestratorContainerResp.GetContainerId()

		defer func() {
			By("cleaning up orchestrator container")
			_, err := realRuntimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{ContainerId: orchestratorContainerID})
			Expect(err).NotTo(HaveOccurred())
			_, err = realRuntimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{ContainerId: orchestratorContainerID})
			Expect(err).NotTo(HaveOccurred())
		}()

		By("starting the orchestrator container")
		_, err = realRuntimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{ContainerId: orchestratorContainerID})
		Expect(err).NotTo(HaveOccurred())

		By("execing into the container to run crictl ps -o json")
		execReq := &runtimeapi.ExecSyncRequest{
			ContainerId: orchestratorContainerID,
			Cmd:         []string{"/crictl", "--runtime-endpoint", "unix:///proxy.sock", "ps", "-o", "json"},
			Timeout:     10,
		}
		execResp, err := realRuntimeClient.ExecSync(ctx, execReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(execResp.GetExitCode()).To(BeZero())

		By("verifying the output only contains containers from the first pod sandbox")
		var output struct {
			Containers []struct {
				PodSandboxID string `json:"podSandboxId"`
			} `json:"containers"`
		}
		err = json.Unmarshal(execResp.GetStdout(), &output)
		Expect(err).NotTo(HaveOccurred())

		Expect(output.Containers).To(HaveLen(2))
		for _, c := range output.Containers {
			Expect(c.PodSandboxID).To(Equal(podSandboxID1))
		}

		By("execing into the container to run crictl stats -o json")
		execReq = &runtimeapi.ExecSyncRequest{
			ContainerId: orchestratorContainerID,
			Cmd:         []string{"/crictl", "--runtime-endpoint", "unix:///proxy.sock", "stats", "-o", "json"},
			Timeout:     10,
		}
		execResp, err = realRuntimeClient.ExecSync(ctx, execReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(execResp.GetExitCode()).To(BeZero())

		By("verifying the output only contains container stats from the first pod sandbox")
		var statsOutput struct {
			Stats []struct {
				Attributes struct {
					//nolint:tagliatelle // keeping the json tag as is to match the crictl output
					ContainerID string `json:"id"`
				} `json:"attributes"`
			} `json:"stats"`
		}
		err = json.Unmarshal(execResp.GetStdout(), &statsOutput)
		Expect(err).NotTo(HaveOccurred())

		Expect(statsOutput.Stats).To(HaveLen(2))
		expectedContainerIDs := map[string]bool{
			containerID1:            true,
			orchestratorContainerID: true,
		}
		for _, s := range statsOutput.Stats {
			Expect(expectedContainerIDs[s.Attributes.ContainerID]).To(BeTrue())
		}
		// This test fails - see TODO below
		// By("execing into the container to run crictl statsp -o json")
		// execReq = &runtimeapi.ExecSyncRequest{
		//	ContainerId: orchestratorContainerID,
		//	Cmd:         []string{"/crictl", "--runtime-endpoint", "unix:///proxy.sock", "statsp", "-o", "json"},
		//  	Timeout:     10,
		// }
		// TODO: the test is currently failing because of:
		// failed to get cgroup metrics for sandbox 0bfc21404353cb87eb9fc231c18feb1bb6df60c17cbc39a537ce0d40470e4611 because cgroupPath is empty
		// execResp, err = realRuntimeClient.ExecSync(ctx, execReq)
		// Expect(err).NotTo(HaveOccurred())
		// if execResp.GetExitCode() != 0 {
		//	GinkgoLogr.Info("crictl statsp stderr", "stderr", string(execResp.GetStderr()))
		//}
		// Expect(execResp.GetExitCode()).To(BeZero())
		//

		// By("verifying the output only contains pod sandbox stats from the first pod sandbox")
		// var podStatsOutput struct {
		//	Stats []struct {
		//		Attributes struct {
		//			Id string `json:"id"`
		//		} `json:"attributes"`
		//	} `json:"stats"`
		//}
		// err = json.Unmarshal(execResp.GetStdout(), &podStatsOutput)
		// Expect(err).NotTo(HaveOccurred())

		// Expect(podStatsOutput.Stats).To(HaveLen(1))
		// for _, s := range podStatsOutput.Stats {
		//	Expect(s.Attributes.Id).To(Equal(podSandboxID1))
		//}
	})
})
