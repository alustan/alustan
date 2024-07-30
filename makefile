# Commands
GO := go

# Directories
INFRA_SRC_DIR := ./cmd/terraform-controller
APP_SRC_DIR := ./cmd/app-controller
INSTALL_SRC_DIR := ./cmd/install-argocd
TEST_DIR := ./test

# Targets
.PHONY: all generate-crds build test setup lint clean

all: generate-crds

generate-infra-crds:
	@echo "+ Generating crds"
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@$(shell go env GOPATH)/bin/controller-gen crd paths="./api/infrastructure/..." output:crd:stdout > ./api/infrastructure/v1alpha1/crd.yaml

generate-app-crds:
	@echo "+ Generating crds"
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@$(shell go env GOPATH)/bin/controller-gen crd paths="./api/app/..." output:crd:stdout > ./api/app/v1alpha1/crd.yaml


## Build the application
build-infra:
	$(GO) build -o bin/terraform-controller $(INFRA_SRC_DIR)

build-app:
	$(GO) build -o bin/app-controller $(APP_SRC_DIR)

build-install:
	$(GO) build -o bin/install-argocd $(INSTALL_SRC_DIR)

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
	@echo "  build-app          Builds the app controller binary"
	@echo "  build-install      Builds the install-argocd  binary"
	@echo "  test           Run tests"
	@echo "  lint           Run linting"
	@echo "  clean          Clean build artifacts"
	@echo "  setup          setup script before build"
	@echo "  generate-infra-crds  Generates infrastructure crds from struct"
	@echo "  generate-app-crds  Generates app crds from struct"
	@echo "  help           Display this help message"
