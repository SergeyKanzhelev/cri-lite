# Gemini Agent Development Log

This document tracks the development process of the `cri-lite` project, with a focus on the interactions and learnings from the Gemini agent's perspective.

## Project Overview

`cri-lite` is a proxy for the Kubernetes Container Runtime Interface (CRI). Its primary purpose is to provide a secure, policy-based interface to the CRI, allowing unprivileged users and tools to interact with the container runtime in a controlled manner. This is a critical security feature for multi-tenant Kubernetes clusters, as direct access to the CRI is equivalent to having administrative privileges on a node.

## Linter Configuration

This project uses `golangci-lint` for linting. The configuration is in the `.golangci.yml` file.

The correct way to lint the code is to run `make lint`. To automatically fix formatting and some linting issues, run `make fmt`.

This project uses 2+ version of the linter, which may not be widely adopted yet. Never suggest to change how linting is enabled. Never try to downgrade linter version.

## TODO Rules

When I suggest placing a `TODO` to address something later, I will use the following conventions:

*   **`TODO:`**: For tasks that can be addressed in the future and are acceptable to have in the production code.
*   **`HACK:`**: For temporary workarounds or unfinished code that must be fixed before the current set of changes is complete. The linter will flag these to prevent them from being committed.

Also I will use `HACK:` convention when I implement the placeholder code that is intended to be replaced.