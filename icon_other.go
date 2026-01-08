//go:build !linux || console

package main

import "unsafe"

// SetWindowIcon is a no-op on non-Linux platforms
func SetWindowIcon(windowPtr unsafe.Pointer) {
	// Icon setting only implemented for Linux/GTK
}
