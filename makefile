# Variables
INFRA_NAME := terraform-controller
APP_NAME := app-controller
GIT_CLONE_NAME := git-clone


# Commands
GO := go

# Directories
INFRA_SRC_DIR := ./cmd/terraform-controller
APP_SRC_DIR := ./cmd/app-controller
CLONE_DIR := ./cmd/git-clone
TEST_DIR := ./test

# Targets
.PHONY: all build build-infra build-app build-git test setup lint clean 


## Build the application
build-infra:
	$(GO) build -o bin/$(INFRA_NAME) $(INFRA_SRC_DIR)

build-app:
	$(GO) build -o bin/$(APP_NAME) $(APP_SRC_DIR)

build-git:
	$(GO) build -o bin/$(GIT_CLONE_NAME) $(CLONE_DIR)

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
	@echo "  build-infra    Builds the infrastructure binary"
	@echo "  build-app      Builds application binary"
	@echo "  build-git      Builds the git clone binary"
	@echo "  test           Run tests"
	@echo "  lint           Run linting"
	@echo "  clean          Clean build artifacts"
	@echo "  setup          setup script before build"
	@echo "  help           Display this help message"
