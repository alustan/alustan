# Commands
GO := go

# Directories
INFRA_SRC_DIR := ./cmd/controller
TEST_DIR := ./test

# Targets
.PHONY: all generate-crds build test setup lint clean

all: generate-crds

generate-crds:
	@echo "+ Generating crds"
	@$(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@$(shell $(GO) env GOPATH)/bin/controller-gen +crd +paths="./api/..." +output:crd:stdout > api/v1alpha1/crd.yaml



## Build the application
build:
	$(GO) build -o bin/terraform-controller $(INFRA_SRC_DIR)

## Run tests
test:
	$(GO) test -v $(TEST_DIR)/...

setup:
	./hack/setup.sh

## Run linting
lint:
	golangci-lint run ./...

## Clean build artifacts
clean:
	rm -rf bin/

## Display help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Builds the controller binary"
	@echo "  test           Run tests"
	@echo "  lint           Run linting"
	@echo "  clean          Clean build artifacts"
	@echo "  setup          setup script before build"
	@echo "  generate-crds  Generates crds from struct"
	@echo "  help           Display this help message"
