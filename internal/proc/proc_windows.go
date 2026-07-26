//go:build windows
// +build windows

package proc

import (
	"os"
	"os/exec"
)

func setupSysProcAttr(cmd *exec.Cmd) {
	// Windows-specific setup if needed
}

func forwardSignal(cmd *exec.Cmd, sig os.Signal) {
	cmd.Process.Signal(sig)
}

func exitCode(ps *os.ProcessState) int {
	return ps.ExitCode()
}
