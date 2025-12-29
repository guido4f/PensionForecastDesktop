//go:build !console

package main

import (
	"fmt"
	"os"

	webview "github.com/webview/webview_go"
)

// runEmbeddedUI starts the web server and opens an embedded browser window
func runEmbeddedUI(configFile string) error {
	// Load configuration (ignore error if file doesn't exist)
	config, err := LoadConfig(configFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Create web server
	ws := NewWebServer(config, "localhost:0")

	// Start server and get URL
	url, cleanup, err := ws.StartForEmbedded()
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	defer cleanup()

	// Create webview window (false = no debug mode)
	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle("Pension Forecast Simulator")
	w.SetSize(1200, 800, webview.HintNone)
	w.Navigate(url)

	// Run blocks until window is closed
	w.Run()

	return nil
}

// runGUI starts the graphical user interface (uses embedded browser)
func runGUI(configFile string) error {
	return runEmbeddedUI(configFile)
}
