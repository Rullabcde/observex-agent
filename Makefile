.PHONY: build build-all clean test release tag

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

BINARY_NAME := observex-agent
BUILD_DIR := dist

# Build for current platform
build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) main.go

# Build for all platforms
build-all: clean
	@mkdir -p $(BUILD_DIR)
	@echo "Building for Linux amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	@echo "Building for Linux arm64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 main.go
	@echo "Building for Linux arm..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm main.go
	@echo "Building for macOS amd64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	@echo "Building for macOS arm64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 main.go
	@echo "Building for Windows amd64..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go
	@echo "Build complete!"
	@echo ""
	@echo "Creating checksums..."
	@cd $(BUILD_DIR) && sha256sum * > checksums.txt
	@echo "Done! Binaries are in $(BUILD_DIR)/"

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)

# Run tests
test:
	go test -v ./...

# Run locally
run:
	go run main.go

# Docker build
docker:
	docker build -t observex-agent:$(VERSION) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE) \
		.

# Create a new tag
tag:
	@read -p "Enter new version (e.g., v1.0.0): " version; \
	git tag -a $$version -m "Release $$version"; \
	echo "Tag $$version created. Push with: git push origin $$version"

# Create release (requires gh CLI)
release: build-all
	@if ! command -v gh &> /dev/null; then \
		echo "Error: GitHub CLI (gh) is not installed"; \
		echo "Install from: https://cli.github.com/"; \
		exit 1; \
	fi
	@echo "Creating GitHub release..."
	@read -p "Enter release version (e.g., v1.0.0): " version; \
	gh release create $$version $(BUILD_DIR)/* \
		--title "Release $$version" \
		--generate-notes

# Quick release workflow
quick-release:
	@echo "Quick Release Workflow"
	@echo "======================"
	@read -p "Enter version (e.g., v1.0.0): " version; \
	echo ""; \
	echo "Creating tag $$version..."; \
	git tag -a $$version -m "Release $$version"; \
	echo "Pushing tag..."; \
	git push origin $$version; \
	echo ""; \
	echo "✓ Tag pushed! GitHub Actions will build and create the release automatically."; \
	echo "  Check progress at: https://github.com/$(shell git config --get remote.origin.url | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/actions"