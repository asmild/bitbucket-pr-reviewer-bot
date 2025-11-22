.PHONY: build build-local build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 run test clean docker-build docker-run docker-stop fmt lint tidy

# Version can be passed as environment variable (empty means no version suffix)
APP_VERSION ?=

# Helper function to build for a specific platform
define build_binary
	@mkdir -p build
	$(eval GOOS := $(1))
	$(eval GOARCH := $(2))
	$(eval VERSION_SUFFIX := $(if $(APP_VERSION),-$(APP_VERSION),))
	$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))
	$(eval LDFLAGS := $(if $(filter windows,$(GOOS)),,-ldflags="-w -s"))
	$(eval ARCH_SUFFIX := $(if $(filter windows,$(GOOS)),,-$(GOARCH)))
	$(eval OUTPUT := ./bin/bb-pr-reviewer-$(GOOS)$(ARCH_SUFFIX)$(VERSION_SUFFIX)$(EXT))
	@echo "Building $(OUTPUT)..."
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(OUTPUT) ./cmd/server/main.go
endef

# Individual platform build targets
build-linux-amd64:
	$(call build_binary,linux,amd64)

build-linux-arm64:
	$(call build_binary,linux,arm64)

build-darwin-amd64:
	$(call build_binary,darwin,amd64)

build-darwin-arm64:
	$(call build_binary,darwin,arm64)

build-windows-amd64:
	$(call build_binary,windows,amd64)

# Build optimized cross-platform binaries
build: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64
	@echo "Build complete! Optimized binaries in build/ directory"

# Build for local development
build-local:
	@echo "Building bb-pr-reviewer for local platform..."
	@CGO_ENABLED=1 go build -o ./bin/bb-pr-reviewer ./cmd/server/main.go
	@echo "Build complete: bb-pr-reviewer"

# Run the application
run:
	@echo "Running pr-reviewer..."
	@go run cmd/server/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f bb-pr-reviewer
	@rm -rf build/
	@rm -f coverage.out coverage.html
	@rm -rf projects/ logs/ metrics-storage/
	@echo "Clean complete"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

# Run linter
lint:
	@echo "Running linter..."
	@go vet ./...
	@echo "Lint complete"

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy
	@echo "Tidy complete"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@echo "Dependencies installed"

# Help
help:
	@echo "Available targets:"
	@echo "  build              - Build optimized cross-platform binaries (Linux, Darwin, Windows)"
	@echo "  build-linux-amd64  - Build for Linux AMD64 (APP_VERSION=x.x.x make build-linux-amd64)"
	@echo "  build-linux-arm64  - Build for Linux ARM64"
	@echo "  build-darwin-amd64 - Build for macOS AMD64"
	@echo "  build-darwin-arm64 - Build for macOS ARM64"
	@echo "  build-windows-amd64- Build for Windows AMD64"
	@echo "  build-local        - Build for local platform (development)"
	@echo "  run                - Run the application"
	@echo "  test               - Run tests"
	@echo "  test-coverage      - Run tests with coverage"
	@echo "  clean              - Clean build artifacts"
	@echo "  fmt                - Format code"
	@echo "  lint               - Run linter"
	@echo "  tidy               - Tidy dependencies"
	@echo "  deps               - Install dependencies"
	@echo "  help               - Show this help"
