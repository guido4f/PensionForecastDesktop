//go:build console

package main

import "fmt"

// runEmbeddedUI is a stub for console-only builds
func runEmbeddedUI(configFile string) error {
	return fmt.Errorf("embedded UI not available in console build. Use -web flag for external browser mode")
}

// runGUI is a stub for console-only builds
func runGUI(configFile string) error {
	return fmt.Errorf("GUI not available in console build. Use -web flag for external browser mode")
}
