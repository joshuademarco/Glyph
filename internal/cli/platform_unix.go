//go:build !windows

package cli

import (
	"os"
	"os/exec"
)

func shellCmd(body string) *exec.Cmd {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	return exec.Command(sh, "-c", body)
}

func defaultEditor() string { return "vi" }
