package main

import (
	"fmt"
	"os"
	"strings"
)

type Get struct {
	EnvFile    string   `arg:"--env-file,-e" default:"./.env.ace" help:"Read encrypted variables from this file"`
	Identities []string `arg:"--identity,-i,separate" help:"Decrypt using the specified IDENTITY. Can be repeated. Defaults to $XDG_CONFIG_HOME/ace/identity"`
	Keys       []string `arg:"positional" placeholder:"KEY" help:"Print only these variables. Prints all variables readable by the identities when omitted"`
}

func (cmd *Get) Run() error {
	src, err := os.Open(cmd.EnvFile)
	if err != nil {
		return err
	}
	defer src.Close()

	identities, err := readIdentities(cmd.Identities, "error")
	if err != nil {
		return err
	}

	vars, err := readEnvFile(src, identities, true)
	if err != nil {
		return err
	}

	for _, kv := range vars {
		if len(cmd.Keys) > 0 {
			var match bool
			for _, k := range cmd.Keys {
				if strings.HasPrefix(kv, k+"=") {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		fmt.Fprintln(output, kv)
	}

	return nil
}
