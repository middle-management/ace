package proc

import (
	"os"
	"os/exec"
)

func SetupSysProcAttr(cmd *exec.Cmd) {
	setupSysProcAttr(cmd)
}

func ForwardSignal(cmd *exec.Cmd, sig os.Signal) {
	forwardSignal(cmd, sig)
}
