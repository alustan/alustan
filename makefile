# Variables
INFRA_NAME := terraform-controller




# Commands
GO := go

# Directories
INFRA_SRC_DIR := ./cmd/controller


TEST_DIR := ./test

# Targets
.PHONY: all build  test setup lint clean 


## Build the application
build:
	$(GO) build -o bin/$(INFRA_NAME) $(INFRA_SRC_DIR)



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
	@echo "  help           Display this help message"
