.PHONY: help build build-windows build-linux build-darwin clean test

# Version from main.go
VERSION := 2.9.1

# Build flags
LDFLAGS := -s -w

# Default target
help:
	@echo "Tactical RMM Agent - Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  make build          - Build executable for current OS/architecture"
	@echo "  make build-windows  - Build Windows executable (amd64)"
	@echo "  make build-linux    - Build Linux executable (amd64)"
	@echo "  make build-darwin   - Build macOS executable (amd64)"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make clean          - Remove built executables"
	@echo "  make test           - Run tests"
	@echo ""
	@echo "For custom builds, use:"
	@echo "  env CGO_ENABLED=0 GOOS=<os> GOARCH=<arch> go build -ldflags \"-s -w\""

# Build for current platform
build:
	@echo "Building for current platform..."
	env CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o tacticalrmm

# Build for Windows (amd64)
build-windows: generate-resources-amd64
	@echo "Building for Windows (amd64)..."
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o tacticalrmm.exe

# Build for Windows (386)
build-windows-386: generate-resources-386
	@echo "Building for Windows (386)..."
	env CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "$(LDFLAGS)" -o tacticalrmm-386.exe

# Generate Windows resource files for amd64
generate-resources-amd64:
	@echo "Generating Windows resource files (amd64)..."
	@command -v goversioninfo >/dev/null 2>&1 || { echo "Installing goversioninfo..."; go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest; }
	@if command -v goversioninfo >/dev/null 2>&1; then \
		goversioninfo -64 versioninfo.json; \
	else \
		$$HOME/go/bin/goversioninfo -64 versioninfo.json; \
	fi

# Generate Windows resource files for 386
generate-resources-386:
	@echo "Generating Windows resource files (386)..."
	@command -v goversioninfo >/dev/null 2>&1 || { echo "Installing goversioninfo..."; go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest; }
	@if command -v goversioninfo >/dev/null 2>&1; then \
		goversioninfo -o resource_386.syso versioninfo.json; \
	else \
		$$HOME/go/bin/goversioninfo -o resource_386.syso versioninfo.json; \
	fi

# Build for Linux (amd64)
build-linux:
	@echo "Building for Linux (amd64)..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o tacticalrmm-linux-amd64

# Build for Linux (arm64)
build-linux-arm64:
	@echo "Building for Linux (arm64)..."
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o tacticalrmm-linux-arm64

# Build for macOS (amd64)
build-darwin:
	@echo "Building for macOS (amd64)..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o tacticalrmm-darwin-amd64

# Build for macOS (arm64/Apple Silicon)
build-darwin-arm64:
	@echo "Building for macOS (arm64)..."
	env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o tacticalrmm-darwin-arm64

# Build for all major platforms
build-all: build-windows build-windows-386 build-linux build-linux-arm64 build-darwin build-darwin-arm64
	@echo "All builds completed!"

# Clean built executables
clean:
	@echo "Cleaning built executables..."
	rm -f tacticalrmm tacticalrmm.exe tacticalrmm-*.exe tacticalrmm-*-*

# Run tests
test:
	go test ./...
