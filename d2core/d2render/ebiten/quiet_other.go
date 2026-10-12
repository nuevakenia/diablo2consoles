//go:build !darwin
// +build !darwin

package ebiten

// quietWindow: only macOS has a native quiet mode; elsewhere the window keeps its defaults.
func quietWindow() {}
