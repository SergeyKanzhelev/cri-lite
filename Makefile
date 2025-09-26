BINARY_NAME=cri-lite
GOLANGCI_LINT_VERSION := v2.5.0
GOLANGCI_LINT := ./.bin/golangci-lint
VERSION := $(shell git describe --tags --dirty --always | sed 's/^v//')

.PHONY: all build run clean lint test clean-test crictl

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -ldflags "-X cri-lite/pkg/version.Version=$(VERSION)" -o bin/$(BINARY_NAME) .
	@echo "$(BINARY_NAME) built successfully in bin/"

run:
	@echo "Running $(BINARY_NAME)..."
	@go run main.go --config config.yaml

clean: clean-test
	@echo "Cleaning up..."
	@rm -f bin/$(BINARY_NAME) /tmp/fake-cri.sock /tmp/cri-lite.sock
	@rm -rf crictl
	@echo "Cleanup complete."
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --config .golangci.yml ./...


$(GOLANGCI_LINT):
	@echo "golangci-lint not found, downloading..."
	@mkdir -p ./.bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./.bin $(GOLANGCI_LINT_VERSION)

fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --config .golangci.yml --fix ./...

test:
	@echo "Running tests..."
	@go test -v ./...

test-e2e:
	@echo "Running E2E tests..."
	sudo -E go test -v ./...


clean-test:
	@echo "Cleaning up test artifacts..."
	@rm -rf /tmp/cri-lite-test
	@echo "Test cleanup complete."

crictl:
	@if [ ! -f ./crictl ]; then \
		echo "crictl not found, downloading..."; \
		CRICTL_VERSION="v1.28.0"; \
		curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/$$CRICTL_VERSION/crictl-$$CRICTL_VERSION-linux-amd64.tar.gz --output crictl-$$CRICTL_VERSION-linux-amd64.tar.gz; \
		tar zxvf crictl-$$CRICTL_VERSION-linux-amd64.tar.gz; \
		rm crictl-$$CRICTL_VERSION-linux-amd64.tar.gz; \
	fi
