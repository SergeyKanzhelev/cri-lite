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

## Testing

Some tests require `sudo` to run. never attempt to run tests under the `test/e2e` directly or using `make test-e2e` commands as they will not succeed in AI sandbox. Ask user to run them and post logs back.

Tests located in the `test/` directory are exclusively for end-to-end scenarios that interact with a "real" container runtime. Unit tests that utilize fake runtimes or mock sockets should be placed within their respective packages (e.g., `pkg/policy/readonly_test.go`, `pkg/proxy/proxy_test.go`).

## Vendored Files

Never modify files within the `vendor/` directory. Changes requiring modifications to vendored files indicate a wrong direction and should be re-evaluated. If a change requires a modification to a vendored file, it should be reverted, and an alternative approach should be sought.

## Continuous Integration

This repository is hosted on GitHub and uses GitHub Actions for continuous integration. The workflow definitions are located in the `.github/workflows` directory. Any changes to the build, test, or linting process should be reflected in these workflow files.
