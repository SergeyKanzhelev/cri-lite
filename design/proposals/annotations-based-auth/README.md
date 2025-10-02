# Annotations-Based Authorization Proposal

**Status:** Draft

## Abstract

This proposal outlines a mechanism to authorize access to `cri-lite` policies using Kubernetes Pod annotations. The goal is to restrict which pods can use which policies, preventing a compromised container from accessing sensitive CRI operations simply by virtue of having access to the `cri-lite` socket.

## Problem Statement

Currently, `cri-lite` acts as a proxy, applying security policies to CRI requests. However, the authorization model is coarse-grained. Any process that can access the `cri-lite` socket (e.g., by mounting its host directory) can make requests against any and all policies that `cri-lite` is configured to serve.

This creates a security risk. If a pod is compromised, the attacker could potentially use `cri-lite` to perform privileged operations that the original application was never intended to, such as listing all containers on the node or pulling arbitrary images. We need a way to enforce that a specific pod is only allowed to use a specific, predetermined policy.

## Proposed Solution

The proposed solution is to leverage Kubernetes Pod annotations as a mechanism for authorization. When a request arrives, `cri-lite` will perform the following steps:

1.  **Identify the Caller:** `cri-lite` will determine the PID of the process making the gRPC request.
2.  **Map PID to Pod:** Using the container runtime, `cri-lite` will resolve the PID to the `PodSandboxId` of the pod it belongs to.
3.  **Fetch Pod Annotations:** `cri-lite` will query the container runtime for the full details of the pod sandbox. This mechanism relies on the pod's annotations, which are propagated by the kubelet to the container runtime as part of the standard `PodSandboxConfig`. This behavior is a core part of the [CRI specification](https://github.com/kubernetes/cri-api/blob/master/pkg/apis/runtime/v1/api.proto#L106), making the annotations available to `cri-lite` for authorization checks.
4.  **Authorize Based on Annotation:** `cri-lite` will check the pod's annotations for a specific key, for example, `cri-lite.io/policy`.
5.  **Policy Matching:** The value of the annotation will specify the name of the policy the pod is authorized to use (e.g., `podscoped`, `image-management`).
6.  **Enforce Decision:** If the annotation exists and its value matches the policy associated with the requested gRPC method, the request is allowed to proceed through the policy engine. Otherwise, the request is denied with a `PermissionDenied` error.

This model shifts the declaration of access from the node-level `cri-lite` configuration to the pod specification itself, making it a Kubernetes-native, auditable part of the pod's definition.

### Example Flow

A pod is deployed with the following annotation:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  annotations:
    cri-lite.io/policy: "podscoped"
spec:
  # ...
```

A process in this pod calls `ListContainers`. `cri-lite` identifies the caller is from the `my-app` pod, sees it is allowed to use the `podscoped` policy, and processes the request according to the rules in `podscoped.yaml`. If the same process tried to call `PullImage`, and that method was only available in the `image-management` policy, the request would be denied.

### Configuration

To enforce annotation-based authorization for a specific endpoint, a new boolean attribute `authorize-from-annotation` is added to its policy configuration. This maintains the existing model of one policy per endpoint while adding an extra layer of security.

When `authorize-from-annotation: true` is set for an endpoint, `cri-lite` will perform the following check before enforcing the policy:
1.  Identify the calling pod.
2.  Read the pod's `cri-lite.io/policy` annotation.
3.  Verify that the value of the annotation exactly matches the `name` of the policy configured for that endpoint.
4.  If they match, the request is allowed to proceed. Otherwise, it is denied.

Here is an example configuration in `/etc/cri-lite/config.yaml` demonstrating this:

```yaml
endpoints:
  # This endpoint is for the ReadOnly policy and does NOT require an annotation.
  # Any pod with access to the socket can use it.
  - endpoint: "/var/run/cri-lite/readonly.sock"
    policy:
      name: "ReadOnly"

  # This endpoint is for the PodScoped policy and REQUIRES an annotation.
  # Only pods with the annotation 'cri-lite.io/policy: "PodScoped"' can use it.
  - endpoint: "/var/run/cri-lite/pod-scoped.sock"
    policy:
      name: "PodScoped"
      authorize-from-annotation: true # This enables the check
      attributes:
        pod-sandbox-from-caller-pid: true
```

This approach allows administrators to decide on a per-endpoint basis whether to enforce annotation-based authorization, providing fine-grained control without changing the fundamental configuration structure. A pod wanting to use the second endpoint would need to be defined with the matching annotation:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  annotations:
    cri-lite.io/policy: "PodScoped"
spec:
  # ...
```

## Security Analysis & Open Questions

This section explores the potential for spoofing or bypassing this authorization mechanism. The core of this design relies on the integrity of pod annotations.

### Q1: How can annotations be spoofed?

There are two primary environments where annotations could be spoofed: at the Kubernetes API level and at the node/runtime level.

#### Kubernetes API-Level Spoofing
- **Threat:** A non-privileged user creates a Pod or modifies an existing one's annotations to grant themselves access to a `cri-lite` policy they should not have.
- **Analysis:** In many Kubernetes environments, less-privileged users are granted RBAC permissions to create or patch Pods within their own namespaces. Relying solely on RBAC to prevent users from setting the `policy.cri-lite.io/policy` annotation is therefore insufficient. A user could easily add this annotation to their pod specification, bypassing the intended security control.
  - **Mitigation:** The most robust mitigation for this threat is a **Kubernetes Validating Admission Webhook**. This webhook must be designed to specifically control the usage of the `cri-lite.io/policy` annotation, rather than relying on indirect checks like volume mounts.
  A naive approach of simply using a webhook to block pods that mount the host path containing the `cri-lite` socket is insufficient for two key reasons:
    1.  **Brittleness of Path Checking:** It's difficult to reliably predict and block all host paths that might expose the socket. A pod could mount a parent directory (e.g., `/var/run/`) for a legitimate reason, inadvertently gaining access to a `cri-lite` sub-path. This makes a path-based blocklist both hard to maintain and prone to error.
    2.  **Decoupling of Concerns:** A pod may have a legitimate need to mount a host path for other purposes (e.g., log collection). Coupling the permission to mount host paths with the permission to use `cri-lite` is too coarse. Cluster administrators need the ability to grant these permissions separately.

  Therefore, the webhook's primary responsibility should be to enforce an allowlist for the annotation itself. It would intercept all `CREATE` and `UPDATE` requests for Pods and apply a set of rules *only when the `cri-lite.io/policy` annotation is present*:
    - Verify that the ServiceAccount, namespace, or user creating the pod is on an explicit allowlist for `cri-lite` usage.
    - Ensure the requested policy (`podscoped`, `image-management`, etc.) is appropriate for the pod's identity and function.

  If the request does not meet these criteria, the webhook would reject the API request, preventing the pod from being created or updated with the annotation.

  For many common use cases, such as restricting the annotation to specific namespaces, this logic can be implemented using a lightweight, in-process `ValidatingAdmissionPolicy` based on the Common Expression Language (CEL), which avoids the need to deploy and maintain a separate webhook server. An example of this can be found in [`cel-policy-example.yaml`](./cel-policy-example.yaml).
- **Conclusion:** This annotation-centric webhook provides granular, explicit, and decoupled control over which workloads can access `cri-lite`. It moves the security decision from a potentially permissive RBAC configuration or a flawed host-path check to a mandatory, auditable, and explicit grant of privilege at admission time, providing a much stronger security guarantee.

#### Node/Runtime-Level Spoofing
- **Threat:** A process running on the node (either in a container or as a host process) directly calls the container runtime to create a pod sandbox with forged annotations, then uses that sandbox to make malicious calls through `cri-lite`.
- **Analysis:** To perform this attack, the process would need direct access to the underlying container runtime socket (e.g., `/run/containerd/containerd.sock` or `/var/run/crio/crio.sock`). Access to this socket is equivalent to having root-level administrative privileges on the node.
- **Conclusion:** If an attacker has administrative control over the node and can access the container runtime directly, they have already bypassed all container isolation and security boundaries. At this point, they can do far more damage than just making unauthorized `cri-lite` calls. Therefore, this threat vector is outside the scope of what this proposal aims to protect against. The threat model for this design assumes the attacker is a non-privileged process running inside a container without access to the host's runtime socket.

### Q2: Is the only way to spoof them to be an admin on the VM?

Yes, for node-level spoofing. To create a pod sandbox with arbitrary annotations via CRI, you need to communicate with the CRI endpoint of the container runtime. Access to this endpoint is, and must be, restricted to privileged users (like `kubelet` and the system administrator).

## Conclusion

The proposed annotation-based authorization model provides a significant security enhancement by linking policy access directly to a pod's identity. Its security relies on established Kubernetes primitives (RBAC) and the existing security boundary between host and container. The risk of spoofing is low, provided that standard Kubernetes and node security practices are followed.