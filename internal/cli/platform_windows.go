//go:build windows

package cli

import "os/exec"

func shellCmd(body string) *exec.Cmd {
	return exec.Command("cmd", "/c", body)
}

func defaultEditor() string { return "notepad" }
