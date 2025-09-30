# In-Place Container Restart with cri-lite Demo

This demo illustrates how to use `cri-lite` to enable in-place container restarts, a feature controlled by the `ContainerRestartRules` feature gate in Kubernetes.

## Overview

The demo consists of two main components:

1.  A **`cri-lite` DaemonSet**: This deploys `cri-lite` as a proxy to the underlying container runtime (e.g., containerd). It exposes a `PodScoped` endpoint, which allows containers within the same pod to manage each other.

    **Note**: The `ContainerRestartRules` feature gate is enabled by default in GKE alpha clusters of version 1.34+.

2.  A **Restart Policy Rules Demo Pod (`restart-policy-rules-demo`)**: This pod contains a single `main-app` container configured with `restartPolicyRules` to restart when it exits with a specific code (108).
    The goal is to show that the `main-app` will be restarting without restarting the whole Pod.

3.  A **Demo Pod (`inplace-restart-demo`)**: This pod contains two containers:
    *   `main-app`: A simple application container that is configured with a restart policy to restart on a specific exit code.
    *   `sidecar-orchestrator`: An init container that uses `crictl` to connect to the `cri-lite` `PodScoped` socket and stop the `main-app` container.
  The goal is to show that the `sidecar-orchestrator` can stop the `main-app` container, and Kubernetes will restart the `main-app` container in-place, without restarting the entire pod.

## Prerequisites

*   A running Kubernetes cluster. If you need to create one, you can use the following command to create an alpha GKE cluster with version 1.34:

    ```bash
    gcloud container clusters create inplace-restart-demo-cluster \
        --release-channel=rapid \
        --cluster-version=1.34 \
        --enable-kubernetes-alpha \
        --zone=us-central1-c \
        --num-nodes=1 \
        --quiet \
        --no-enable-autorepair \
        --no-enable-autoupgrade
    ```

*   `kubectl` configured to interact with your cluster. After creating the cluster, connect to it using:

    ```bash
    gcloud container clusters get-credentials inplace-restart-demo-cluster --zone us-central1-c
    ```

## Steps

### 1. Deploy the Restart Policy Rules Demo Pod

This demo can be run independently of `cri-lite`.

Deploy the pod that demonstrates `restartPolicyRules`.

```bash
kubectl apply -f restart-policy-rules-demo-pod.yaml
```

### 2. Observe and Verify the Restart Policy Rules Demo

Check the status of the `restart-policy-rules-demo` pod. The `main-app` container will start, sleep for 15 seconds, exit with code 108, and then be restarted by Kubernetes due to the `restartPolicyRules`.

Wait for about 20 seconds for the container to exit and restart, then check the pod status:

```bash
kubectl get pod restart-policy-rules-demo
```

Look at the `RESTARTS` column. It should have value of `1` or more after the container was restarted.

To see more details, you can inspect the pod's status conditions:

```bash
kubectl describe pod restart-policy-rules-demo
```

You will see that the `main-app` container has a `Restart Count` of `1` or more after the container was restarted.

To see the logs of the `main-app` container:

```bash
kubectl logs restart-policy-rules-demo -c main-app
```

### 3. Deploy the `cri-lite` DaemonSet

This DaemonSet will:
- Run `cri-lite` and expose a Pod-scoped CRI socket at `/var/run/cri-lite/dynamic-podscope/cri-lite.sock` on the host.

```bash
kubectl apply -f daemonset.yaml
```

Wait for the DaemonSet to be rolled out to the nodes in your cluster. You can check the status with:

```bash
kubectl rollout status daemonset/cri-lite-in-place-restart
```

### 4. Deploy the In-Place Restart Demo Pod

Once the DaemonSet is ready, deploy the pod that contains the main application and the sidecar orchestrator.

```bash
kubectl apply -f inplace-restart-demo-pod.yaml
```

### 5. Observe the In-Place Restart Demo

Check the logs of the `sidecar-orchestrator` init container to see it stopping the `main-app` container.

```bash
kubectl logs inplace-restart-demo -c sidecar-orchestrator
```

You should see output indicating that it found and stopped the `main-app` container successfully.

### 6. Verify the In-Place Restart Demo

Check the status of the `inplace-restart-demo` pod. You will see that the `main-app` container has been restarted.

```bash
kubectl get pod inplace-restart-demo
```

Look at the `RESTARTS` column. It should be `1`.

To see more details, you can inspect the pod's status conditions:

```bash
kubectl describe pod inplace-restart-demo
```

You will see that the `main-app` container has a `Restart Count` of 1.

## Cleanup

To remove the resources created in this demo, run the following commands:

```bash
kubectl delete pod inplace-restart-demo restart-policy-rules-demo
kubectl delete ds cri-lite-in-place-restart
```
