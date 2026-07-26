//go:build !windows
// +build !windows

package proc

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func setupSysProcAttr(cmd *exec.Cmd) {
	// run the child in its own process group so forwardSignal can signal
	// the whole tree it spawns (e.g. when ace is a docker entrypoint and
	// only PID 1 receives SIGTERM)
	attr := &syscall.SysProcAttr{
		Setpgid: true,
	}
	// when stdin is a terminal and ace is its foreground process group,
	// hand the foreground over to the child's new group so terminal
	// keystrokes (Ctrl-C) and reads keep reaching it
	if f, ok := cmd.Stdin.(*os.File); ok {
		if fg, err := unix.IoctlGetInt(int(f.Fd()), unix.TIOCGPGRP); err == nil && fg == unix.Getpgrp() {
			attr.Foreground = true
			attr.Ctty = int(f.Fd())
		}
	}
	cmd.SysProcAttr = attr
}

func forwardSignal(cmd *exec.Cmd, sig os.Signal) {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, s); err != nil {
		_ = cmd.Process.Signal(sig)
	}
}

func exitCode(ps *os.ProcessState) int {
	// follow the shell convention of 128+signal for a signal-terminated
	// child instead of ProcessState.ExitCode's -1
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ps.ExitCode()
}
