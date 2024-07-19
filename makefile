# Commands
GO := go

# Directories
INFRA_SRC_DIR := ./cmd/terraform-controller
SERVICE_SRC_DIR := ./cmd/service-controller
TEST_DIR := ./test

# Targets
.PHONY: all generate-crds build test setup lint clean

all: generate-crds

generate-infra-crds:
	@echo "+ Generating crds"
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@$(shell go env GOPATH)/bin/controller-gen crd paths="./api/infrastructure/..." output:crd:stdout > ./api/infrastructure/v1alpha1/crd.yaml

generate-service-crds:
	@echo "+ Generating crds"
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@$(shell go env GOPATH)/bin/controller-gen crd paths="./api/service/..." output:crd:stdout > ./api/service/v1alpha1/crd.yaml


## Build the application
build-infra:
	$(GO) build -o bin/terraform-controller $(INFRA_SRC_DIR)

build-service:
	$(GO) build -o bin/service-controller $(SERVICE_SRC_DIR)

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
	@echo "  build-infra          Builds the infra controller binary"
	@echo "  build-service          Builds the service controller binary"
	@echo "  test           Run tests"
	@echo "  lint           Run linting"
	@echo "  clean          Clean build artifacts"
	@echo "  setup          setup script before build"
	@echo "  generate-infra-crds  Generates infrastructure crds from struct"
	@echo "  generate-service-crds  Generates service crds from struct"
	@echo "  help           Display this help message"
