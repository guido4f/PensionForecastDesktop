# Pension Forecast Build Makefile
# Cross-compile for macOS (Apple Silicon), Windows, Linux
# Supports embedded browser (-ui), web server (-web), and console-only builds

BINARY_NAME=pensionForecast
VERSION?=1.0.0
BUILD_DIR=build

# Build flags
LDFLAGS=-ldflags "-s -w"

.PHONY: all clean test build help \
       ui ui-linux ui-macos ui-macos-intel ui-windows \
       console-all console-linux console-macos console-macos-intel console-windows \
       web-all web-linux web-macos web-macos-intel web-windows \
       web web-port run-ui

# Build all cross-platform (console-only, no CGO required)
all: clean console-all
	@echo "Build complete. Console binaries in $(BUILD_DIR)/"
	@echo "Note: Embedded UI builds require native compilation on each platform."

# =============================================================================
# Embedded Browser builds (requires CGO for webview)
# =============================================================================
# These builds include embedded browser functionality. Run with: ./binary -ui

# Build embedded UI for current platform
ui:
	@echo "Building embedded UI for current platform..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-ui .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-ui"

# Native embedded browser builds (run these ON the target platform)
# These require CGO and cannot be cross-compiled
ui-linux:
	@echo "Building embedded UI for Linux (x64)..."
	@echo "Requires: sudo apt-get install libgtk-3-dev libwebkit2gtk-4.0-dev"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64-ui .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64-ui"

ui-macos:
	@echo "Building embedded UI for macOS (Apple Silicon, must run on macOS)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64-ui .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64-ui"

ui-macos-intel:
	@echo "Building embedded UI for macOS Intel (must run on macOS)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-macos-amd64-ui .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-macos-amd64-ui"

ui-windows:
	@echo "Building embedded UI for Windows (x64)..."
	@echo "Requires: WebView2 runtime (pre-installed on Windows 10 20H2+, Windows 11)"
	@echo "Cross-compiling from Linux requires: apt-get install gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64"
	@mkdir -p $(BUILD_DIR)
ifeq ($(OS),Windows_NT)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64-ui.exe .
else
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64-ui.exe .
endif
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64-ui.exe"

# Run embedded UI (builds first if needed)
run-ui: ui
	./$(BUILD_DIR)/$(BINARY_NAME)-ui -ui

# =============================================================================
# Console-only builds (no GUI, smaller binaries, for automation/scripting)
# =============================================================================
# These builds can use -web flag for external browser mode

console-all: console-linux console-macos console-macos-intel console-windows
	@echo "Console-only builds complete."

console-linux:
	@echo "Building console-only for Linux (x64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -tags console $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64-console .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64-console"

console-macos:
	@echo "Building console-only for macOS (Apple Silicon)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -tags console $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64-console .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64-console"

console-macos-intel:
	@echo "Building console-only for macOS (Intel)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -tags console $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-macos-amd64-console .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-macos-amd64-console"

console-windows:
	@echo "Building console-only for Windows (x64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -tags console $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64-console.exe .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64-console.exe"

# =============================================================================
# Web Server builds (cross-platform, includes web UI, no CGO required)
# =============================================================================
# These builds include web server functionality. Run with: ./binary -web

web-all: web-linux web-macos web-macos-intel web-windows
	@echo "Web server builds complete."

web-linux:
	@echo "Building web server for Linux (x64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -tags console $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64-web .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64-web"

web-macos:
	@echo "Building web server for macOS (Apple Silicon)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -tags console $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64-web .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64-web"

web-macos-intel:
	@echo "Building web server for macOS (Intel)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -tags console $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-macos-amd64-web .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-macos-amd64-web"

web-windows:
	@echo "Building web server for Windows (x64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -tags console $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64-web.exe .
	@echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64-web.exe"

# Build for current platform only
build:
	@echo "Building for current platform..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .

# Run web server (builds first if needed, auto port, opens browser)
web: build
	./$(BINARY_NAME) -web

# Run web server on specific port (usage: make web-port PORT=8080)
PORT?=8080
web-port: build
	./$(BINARY_NAME) -web -addr :$(PORT)

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)

# Show help
help:
	@echo "Pension Forecast Build Targets:"
	@echo ""
	@echo "  Embedded Browser (requires CGO, run ON target platform):"
	@echo "  make ui               - Build embedded UI for current platform"
	@echo "  make ui-linux         - Build embedded UI for Linux (x64)"
	@echo "  make ui-macos         - Build embedded UI for macOS (ARM)"
	@echo "  make ui-macos-intel   - Build embedded UI for macOS (Intel)"
	@echo "  make ui-windows       - Build embedded UI for Windows (x64)"
	@echo "  make run-ui           - Build and run embedded UI"
	@echo ""
	@echo "  Web Server (opens external browser):"
	@echo "  make web              - Build and run web server (auto port)"
	@echo "  make web-port PORT=8080 - Run web server on specific port"
	@echo ""
	@echo "  Cross-platform web server builds (can build from any OS):"
	@echo "  make web-all              - Build web server for all platforms"
	@echo "  make web-linux            - Build web server for Linux (x64)"
	@echo "  make web-macos            - Build web server for macOS (ARM)"
	@echo "  make web-macos-intel      - Build web server for macOS (Intel)"
	@echo "  make web-windows          - Build web server for Windows (x64)"
	@echo ""
	@echo "  Console-only builds (can build from any OS):"
	@echo "  make all                  - Build console-only for all platforms"
	@echo "  make console-all          - Build console-only for all platforms"
	@echo "  make console-linux        - Build console-only for Linux (x64)"
	@echo "  make console-macos        - Build console-only for macOS (ARM)"
	@echo "  make console-macos-intel  - Build console-only for macOS (Intel)"
	@echo "  make console-windows      - Build console-only for Windows (x64)"
	@echo ""
	@echo "  Other:"
	@echo "  make build         - Build for current platform"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Remove build artifacts"
	@echo ""
	@echo "  Prerequisites for embedded UI builds:"
	@echo "  - Linux: sudo apt-get install libgtk-3-dev libwebkit2gtk-4.0-dev"
	@echo "  - macOS: Xcode command line tools (WebKit built-in)"
	@echo "  - Windows: WebView2 runtime (pre-installed on Win10 20H2+, Win11)"
