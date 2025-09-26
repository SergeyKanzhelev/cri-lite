BINARY_NAME=cri-lite
GOLANGCI_LINT_VERSION := v2.5.0
GOLANGCI_LINT := ./.bin/golangci-lint

.PHONY: all build run clean lint test clean-test

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o bin/$(BINARY_NAME) .
	@echo "$(BINARY_NAME) built successfully in bin/"

run:
	@echo "Running $(BINARY_NAME)..."
	@go run main.go --config config.yaml

clean: clean-test
	@echo "Cleaning up..."
	@rm -f bin/$(BINARY_NAME) /tmp/fake-cri.sock /tmp/cri-lite.sock
	@rm -rf crictl crictl-v*-linux-amd64.tar.gz
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
	@go test ./...

clean-test:
	@echo "Cleaning up test artifacts..."
	@rm -rf /tmp/cri-lite-test
	@echo "Test cleanup complete."
