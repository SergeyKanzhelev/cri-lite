# Declarative Policy Configuration

To provide a more flexible and user-friendly way to define security policies, `cri-lite` is moving from hardcoded Go implementations to a declarative YAML-based rule engine. This allows administrators to define fine-grained access controls without modifying and recompiling the source code.

## Core Concepts

The policy is defined as a list of rules in a YAML file. When `cri-lite` receives a gRPC request, it evaluates these rules in the order they appear. The first rule that matches the incoming method determines the action (`allow` or `deny`).

**If no rule matches a request, it is denied by default.** This ensures a secure-by-default posture.

## Rule Schema

Each rule is an object with the following fields:

```yaml
- method: <string>
  action: <string>
  conditions:
    - field: <string>
      operator: <string>
      source: <string>
      value: <any>
  filters:
    - field: <string>
      filterField: <string>
      operator: <string>
      source: <string>
```

---

### **`method`** (Required)
The full gRPC method name to match. It supports `*` for globbing, allowing you to match an entire service.
*Example:* `/runtime.v1.RuntimeService/ListContainers`
*Example (globbing):* `/runtime.v1.ImageService/*`

---

### **`action`** (Required)
The action to take if the method matches.
*   `allow`: Permit the request to proceed (subject to `conditions`).
*   `deny`: Immediately reject the request with a `PermissionDenied` error.

---

### **`conditions`** (Optional)
A list of conditions that must **all** be met for an `allow` rule to take effect. If any condition fails, the rule is skipped.

Each condition object has the following fields:

*   **`field`** (Required): The path to a field in the request message, using dot notation (e.g., `Filter.PodSandboxId`).
*   **`operator`** (Required): The comparison to perform.
    *   `equals`: The field value must exactly match the `value` or `source`.
    *   `belongsToPod`: A special operator that verifies a given `ContainerId` belongs to the pod sandbox identified by the `source`.
*   **`value`** (Conditional): A static value to compare against (e.g., a specific `PodSandboxId`).
*   **`source`** (Conditional): A dynamic value derived from the request's context. This is used for policies that depend on the caller's identity.
    *   `podSandboxIdFromPID`: `cri-lite` inspects the caller's PID to find the `PodSandboxId` of the pod it belongs to.

---



### **`filters`** (Optional)

A list of filters to apply to the *response* of an allowed request. This is a critical feature for scoping policies, as it ensures that list operations only return resources relevant to the caller. For example, it can filter the result of `ListContainers` to ensure a user only sees containers within their own pod.



Each filter object has the following fields:



*   `field` (Required): The path to a repeated field in the response message that should be filtered (e.g., `Containers` in a `ListContainersResponse`).

*   `filterField` (Required): The field *within* each element of the repeated field that the filter will check (e.g., `PodSandboxId` for a container, or `Attributes.Id` for a container stat).

*   `operator` (Required): The comparison to perform. This uses the same operators as `conditions`.

    *   `equals`: The `filterField` value must exactly match the `source`.

    *   `belongsToPod`: The `filterField` (which must be a `ContainerId`) is checked to ensure it belongs to the pod sandbox identified by the `source`.

*   `source` (Required): The dynamic value to compare against (e.g., `podSandboxIdFromPID`).



## Example: Filtering a List Response



This rule allows a call to `ListContainers` but ensures the response only contains containers that belong to the caller's pod.



```yaml
- method: /runtime.v1.RuntimeService/ListContainers
  action: allow
  filters:
    - field: Containers
      filterField: PodSandboxId
      operator: equals
      source: podSandboxIdFromPID
```
