//go:build windows

package clipboard

// CONOUT$ is the Windows console output device; opening it lets the OSC 52
func ttyName() string { return "CONOUT$" }
