package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/middle-management/ace/internal/proc"
)

type Env struct {
	OnMissing  string   `arg:"--on-missing" default:"error" help:"How to handle when env-file or identity is missing, can be 'ignore', 'warn' or 'error'"`
	EnvFile    string   `arg:"--env-file,-e" default:"./.env.ace" help:"Read encrypted variables from this file"`
	Identities []string `arg:"--identity,-i,separate" help:"Decrypt using the specified IDENTITY. Can be repeated. Defaults to $XDG_CONFIG_HOME/ace/identity"`
	Command    []string `arg:"positional,required" placeholder:"COMMAND" help:"Command to run with the decrypted variables added to its environment. Use -- before it to separate its flags from ace's"`
}

func (cmd *Env) Run() error {
	if len(cmd.Command) == 0 {
		return fmt.Errorf("missing command to run")
	}
	src, err := os.Open(cmd.EnvFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			switch cmd.OnMissing {
			case "ignore":
				// silence
			case "warn", "warning":
				slog.Warn("env-file not found")
			default:
				return err
			}
		} else {
			return err
		}
	} else {
		defer src.Close()
	}

	identities, err := readIdentities(cmd.Identities, cmd.OnMissing)
	if err != nil {
		return err
	}

	var vars []string
	if src != nil {
		if len(identities) == 0 {
			switch cmd.OnMissing {
			case "ignore":
				// silence
			case "warn", "warning":
				slog.Warn("no identities specified")
			default:
				return errors.New("no identities specified")
			}
		} else {
			vars, _, err = readEnvFile(src, identities, false)
			if err != nil {
				return err
			}
		}
	}

	c := exec.Command(cmd.Command[0], cmd.Command[1:]...)
	c.Env = append(os.Environ(), vars...)
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr
	c.Stdout = output

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer func() {
		signal.Stop(sigChan)
		close(sigChan)
	}()

	go func() {
		for sig := range sigChan {
			if c.Process != nil {
				proc.ForwardSignal(c, sig)
			}
		}
	}()

	return c.Run()
}
