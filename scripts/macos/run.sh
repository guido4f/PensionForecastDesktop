#!/bin/bash
# Pension Forecast Simulator - macOS Run Script
# This script removes quarantine attributes and runs the application

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    ARCH_SUFFIX="arm64"
else
    ARCH_SUFFIX="amd64"
fi

echo "Pension Forecast Simulator"
echo "=========================="
echo "Detected: macOS $ARCH"
echo ""

# Find available binaries for this architecture
WEB_BIN="$SCRIPT_DIR/goPensionForecast-macos-${ARCH_SUFFIX}-web"
CONSOLE_BIN="$SCRIPT_DIR/goPensionForecast-macos-${ARCH_SUFFIX}-console"
UI_BIN="$SCRIPT_DIR/goPensionForecast-macos-${ARCH_SUFFIX}-ui"

# Remove quarantine and set permissions
echo "Preparing binaries..."
for file in "$SCRIPT_DIR"/goPensionForecast-macos-*; do
    if [ -f "$file" ]; then
        xattr -d com.apple.quarantine "$file" 2>/dev/null
        chmod +x "$file" 2>/dev/null
    fi
done
echo ""

echo "Available modes:"
echo ""
if [ -f "$WEB_BIN" ]; then
    echo "  1) Web Server (recommended) - Opens in browser"
fi
if [ -f "$UI_BIN" ]; then
    echo "  2) Desktop UI - Native macOS window"
fi
if [ -f "$CONSOLE_BIN" ]; then
    echo "  3) Console - Command line interface"
fi
echo "  q) Quit"
echo ""
read -p "Select mode [1]: " choice

case "${choice:-1}" in
    1)
        if [ -f "$WEB_BIN" ]; then
            echo "Starting web server..."
            exec "$WEB_BIN" -web
        else
            echo "Web binary not found"
            exit 1
        fi
        ;;
    2)
        if [ -f "$UI_BIN" ]; then
            echo "Starting desktop UI..."
            exec "$UI_BIN" -ui
        else
            echo "UI binary not found"
            exit 1
        fi
        ;;
    3)
        if [ -f "$CONSOLE_BIN" ]; then
            echo "Starting console mode..."
            exec "$CONSOLE_BIN" -console
        else
            echo "Console binary not found"
            exit 1
        fi
        ;;
    q|Q)
        echo "Exiting."
        exit 0
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac
