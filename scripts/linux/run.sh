#!/bin/bash
# Pension Forecast Simulator - Linux Run Script
# This script sets up and runs the application

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "Pension Forecast Simulator"
echo "=========================="
echo ""

# Find available binaries
WEB_BIN="$SCRIPT_DIR/goPensionForecast-linux-amd64-web"
CONSOLE_BIN="$SCRIPT_DIR/goPensionForecast-linux-amd64-console"
UI_BIN="$SCRIPT_DIR/goPensionForecast-linux-amd64-ui"

# Make binaries executable
chmod +x "$WEB_BIN" 2>/dev/null
chmod +x "$CONSOLE_BIN" 2>/dev/null
chmod +x "$UI_BIN" 2>/dev/null

echo "Available modes:"
echo ""
if [ -f "$WEB_BIN" ]; then
    echo "  1) Web Server (recommended) - Opens in browser"
fi
if [ -f "$UI_BIN" ]; then
    echo "  2) Desktop UI - Embedded browser window"
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
            echo "UI binary not found. Note: UI requires GTK3 and WebKit2GTK."
            echo "Install with: sudo apt-get install libgtk-3-0 libwebkit2gtk-4.1-0"
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
