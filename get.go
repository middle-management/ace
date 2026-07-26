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

	vars, _, err := readEnvFile(src, identities, true)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd.EnvFile, err)
	}

	found := make(map[string]bool, len(cmd.Keys))
	for _, kv := range vars {
		if len(cmd.Keys) > 0 {
			var match bool
			for _, k := range cmd.Keys {
				if strings.HasPrefix(kv, k+"=") {
					match = true
					found[k] = true
				}
			}
			if !match {
				continue
			}
		}
		fmt.Fprintln(output, kv)
	}

	// fail when a requested key has no readable value, so scripts can
	// distinguish an unset variable from an empty one
	var missing []string
	for _, k := range cmd.Keys {
		if !found[k] {
			found[k] = true // dedupe repeated keys
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("no readable value for: %s", strings.Join(missing, ", "))
	}

	return nil
}
