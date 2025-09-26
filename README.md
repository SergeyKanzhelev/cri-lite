# cri-lite (The CRI Proxy) - Providing a Limited Interface Through Enforcement

## Overview

cri-lite is a proxy for the Kubernetes Container Runtime Interface (CRI) that allows for lower privilege access to a subset of the CRI API. In a standard Kubernetes environment, granting access to the CRI API is equivalent to granting administrative privileges on the node, as it allows for unrestricted control over containers and sandboxes. This level of access is often too permissive and poses a significant security risk.

cri-lite addresses this problem by providing a policy-based enforcement layer in front of the CRI API. This allows administrators to define fine-grained policies that restrict what a user or tool can do, effectively creating a limited, secure interface to the container runtime.

## Key Features

*   **Subset of CRI API:** Expose only a limited, safer subset of the CRI API to unprivileged users and tools.
*   **Policy-Based Enforcement:** Implement policies to control access to specific CRI API calls based on user identity or other attributes.
*   **Pod-Scoped Permissions:** Limit API access based on a specific Pod's Sandbox ID. This powerful feature enables Pods to manage their own lifecycle and resources without gaining access to other Pods on the same node.
*   **Enhanced Security:** Reduce the attack surface by preventing direct, unrestricted access to the node's container runtime.

## Motivation & Use Cases

Many scenarios in Kubernetes require some level of interaction with the container runtime, but do not require full administrative access.

*   **CI/CD Pipelines:** A CI/CD runner pod might need to create or inspect its own containers for a job, but it should not be able to interfere with other pods on the node.
*   **In-Pod Tooling:** A debugging or monitoring tool running within a pod might need to collect information about its own container environment without having node-level permissions.
*   **Self-Managing Pods:** Applications that can manage their own state, perform self-healing, or dynamically adjust their resources can use cri-lite to safely interact with their own sandbox.

By using cri-lite, platform administrators can empower developers and tools with the access they need while maintaining a strong security posture.

## Design

The initial implementation of cri-lite will be a proxy that is configured via a static configuration file. This file will define a set of endpoints, each with its own UNIX socket and a set of policies that enforce access controls.

### Configuration

The configuration file for cri-lite is a YAML file that defines global settings and a list of endpoints to create.

Here is a sample configuration file:

```yaml
# /etc/cri-lite/config.yaml
runtime-endpoint: "unix:///run/containerd/containerd.sock"
image-endpoint: "unix:///run/containerd/containerd.sock"
timeout: 10
debug: true

endpoints:
  - endpoint: "/var/run/cri-lite/readonly.sock"
    policies:
      - "ReadOnly"

  - endpoint: "/var/run/cri-lite/image-manager.sock"
    policies:
      - "ImageManagement"

  - endpoint: "/var/run/cri-lite/pod-app-static.sock"
    policies:
      - "PodScoped"
    pod_sandbox_id: "some-hardcoded-sandbox-id"

  - endpoint: "/var/run/cri-lite/pod-app-dynamic.sock"
    policies:
      - "PodScoped"
    # For PodScoped, the sandbox ID can be dynamically determined
    # from the caller's PID.
    pod_sandbox_from_caller_pid: true
```

**Global Settings:**

*   `runtime-endpoint`: The upstream CRI socket for the container runtime.
*   `image-endpoint`: The upstream CRI socket for the image service. If not specified, `runtime-endpoint` is used.
*   `timeout`: Timeout in seconds for CRI calls.
*   `debug`: Enable debug logging.

### Policies

Policies are composable rules that determine which CRI API calls are allowed. The initial set of policies will be:

*   **ReadOnly:** This policy grants access to read-only operations from both the `RuntimeService` and `ImageService`. It allows calls like `ListContainers`, `ContainerStatus`, and `ListImages`, but denies any calls that would modify the state of the system, such as `CreateContainer`, `RemoveContainer`, `PullImage`, or `RemoveImage`.

*   **ImageManagement:** This policy grants full access to the `ImageService` API, allowing users to pull, list, and remove images. However, it denies all access to the `RuntimeService` API, preventing any interaction with running containers or pods.

*   **PodScoped:** This policy is a filter that restricts `RuntimeService` operations to a single, specific `PodSandbox`. When this policy is active, cri-lite will inspect each incoming CRI call. If the call contains a `pod_sandbox_id`, it must match the one associated with the endpoint. For calls that reference a `container_id`, cri-lite will first verify that the container belongs to the allowed `pod_sandbox_id` before proxying the request. This enables a pod to safely manage its own containers.

    The `pod_sandbox_id` can be provided in two ways:
    1.  **Static:** A specific `pod_sandbox_id` is hardcoded in the configuration file. This is useful for dedicated services that manage a known pod.
    2.  **Dynamic:** The `pod_sandbox_id` is determined at runtime by inspecting the PID of the process calling the cri-lite socket. This allows for a more general setup where any pod can be granted access to manage itself.

The `RunPodSandbox` call is forbidden across all policies, as creating new pods is a highly privileged operation that is outside the scope of cri-lite's intended use cases.

## Usage

`cri-lite` is started with a single command-line argument that points to the configuration file:

```bash
cri-lite --config /etc/cri-lite/config.yaml
```

The tool will then create the specified UNIX sockets and start listening for connections.

### `crictl` Compatibility

A key design principle is to maintain compatibility with standard tools like `crictl`. Because cri-lite exposes standard CRI-compatible sockets, `crictl` can be used to interact with the limited API endpoints just by specifying the appropriate socket path.

For example, a user with access to the `readonly` socket could inspect containers with the following command:

```bash
crictl --runtime-endpoint unix:///var/run/cri-lite/readonly.sock ps
```

## Security Posture

The security of `cri-lite` relies on the principle of least privilege, providing a limited interface to the container runtime. However, it is essential to understand the security context in which it operates.

### Privilege Escalation

`cri-lite` is designed to prevent privilege escalation beyond the scope of its configured policies. The proxy does not grant any additional permissions to the caller; it only allows a subset of the CRI API calls to be forwarded to the container runtime. The policies are enforced on the server-side, and the client has no ability to bypass them.

### PID and Cgroup Spoofing

The dynamic `PodScoped` policy relies on the caller's PID to determine its pod sandbox ID by inspecting the `/proc` filesystem and cgroups. This introduces a dependency on the integrity of the underlying node's process and cgroup management.

A malicious actor with sufficient privileges on the node could potentially manipulate PIDs or cgroups to impersonate another pod and gain unauthorized access to its containers. However, it is important to note that the level of privilege required to perform such an attack is extremely high. An attacker would need to have root or near-root access to the node to modify the `/proc` filesystem or manipulate cgroups.

To spoof a PID when calling the `cri-lite` socket, an attacker would need to be able to control the process that is making the call. This would require the ability to either inject code into a running process or to create a new process with a specific PID. Both of these actions require the `CAP_SYS_ADMIN` capability, which is not granted to containers by default.

To modify cgroups, an attacker would need to have write access to the cgroup filesystem. This is a privileged operation that is typically only available to the root user on the host.

If an attacker has already gained this level of access, they would almost certainly have the ability to bypass `cri-lite` and interact with the container runtime directly. Therefore, while PID and cgroup spoofing is a theoretical attack vector, it is not considered a practical vulnerability in the context of `cri-lite`'s intended use case.

In summary, `cri-lite` provides a significant security enhancement by limiting the CRI API surface, but it is not a substitute for a secure and trusted node environment. The `cri-lite` DaemonSet is configured to run with `hostPID: true` to ensure it can correctly identify the caller's PID, but this also underscores the importance of securing the node itself.

## Demo

This project includes Kubernetes manifests to demonstrate the functionality of `cri-lite`.

### DaemonSet Deployment

The `cri-lite` proxy can be deployed as a DaemonSet on your Kubernetes cluster. This will run a `cri-lite` pod on each node, providing a secure interface to the container runtime on that node.

To deploy the DaemonSet, apply the `k8s/cri-lite.yaml` manifest:

```bash
kubectl apply -f k8s/cri-lite.yaml
```

The DaemonSet runs in a privileged security context and with `hostPID: true` to allow it to access the host's container runtime and see the PIDs of processes running on the node.

### Example Pods

The following example pods demonstrate how to use the `cri-lite` proxy.

#### Image Management

The `image-client-pod.yaml` manifest demonstrates the image management policy. This pod will:

1.  List all images on the node.
2.  Pull a new image.
3.  Attempt to list containers (which will fail, as the policy does not allow it).

To deploy the image client pod, apply the `k8s/image-client-pod.yaml` manifest:

```bash
kubectl apply -f k8s/image-client-pod.yaml
```

#### Pod-Scoped Policy

The `podscoped-client-pod.yaml` manifest demonstrates the pod-scoped policy. This pod has two containers: a "victim" and an "orchestrator". The orchestrator container will:

1.  List containers within the same pod.
2.  Find the ID of the "victim" container.
3.  Stop the "victim" container.

To deploy the pod-scoped client pod, apply the `k8s/podscoped-client-pod.yaml` manifest:

```bash
kubectl apply -f k8s/podscoped-client-pod.yaml
```