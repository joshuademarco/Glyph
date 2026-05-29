//go:build !windows

package clipboard

func ttyName() string { return "/dev/tty" }
