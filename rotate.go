package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

type Rotate struct {
	RecipientFiles []string `arg:"--recipient-file,-R,separate" help:"Encrypt to recipients listed at RECIPIENT-FILE. Can be repeated. Defaults to ./recipients.txt"`
	Recipients     []string `arg:"--recipient,-r,separate" help:"Encrypt to the specified RECIPIENT. Can be repeated."`
	EnvFile        string   `arg:"--env-file,-e" default:"./.env.ace" help:"Rewrite this file with the re-encrypted variables"`
	Identities     []string `arg:"--identity,-i,separate" help:"Decrypt using the specified IDENTITY. Can be repeated. Defaults to $XDG_CONFIG_HOME/ace/identity"`
}

// Run rewrites the env file as a single fresh block encrypted to the given
// recipients. Unlike `ace get | ace set` this replaces the file instead of
// appending, so previous recipients lose access to the new file contents
// and the file stops growing.
func (cmd *Rotate) Run() error {
	recipients, err := parseRecipients(cmd.Recipients, cmd.RecipientFiles)
	if err != nil {
		return err
	}

	identities, err := readIdentities(cmd.Identities, "error")
	if err != nil {
		return err
	}

	vars, stats, err := func() ([]string, envFileStats, error) {
		src, err := os.Open(cmd.EnvFile)
		if err != nil {
			return nil, envFileStats{}, err
		}
		defer src.Close()
		vars, stats, err := readEnvFile(src, identities, true)
		if err != nil {
			err = fmt.Errorf("%s: %w", cmd.EnvFile, err)
		}
		return vars, stats, err
	}()
	if err != nil {
		return err
	}
	if stats.matched < stats.blocks {
		return fmt.Errorf("refusing to rotate: %d of %d blocks could not be decrypted with the available identities, their vars would be lost", stats.blocks-stats.matched, stats.blocks)
	}

	warnOnSelfLockout(identities, recipients)

	var kvs [][2]string
	for _, kv := range vars {
		pair := strings.SplitN(kv, "=", 2)
		if len(pair) != 2 {
			continue
		}
		kvs = append(kvs, [2]string{pair[0], pair[1]})
	}

	var block string
	if len(kvs) > 0 {
		block, err = encryptBlock(recipients, kvs)
		if err != nil {
			return err
		}
	}

	// keep the permissions of the file being replaced
	mode := os.FileMode(0644)
	if fi, err := os.Stat(cmd.EnvFile); err == nil {
		mode = fi.Mode().Perm()
	}

	// write to a temp file in the same directory and rename it over the
	// original, so a failed rotation never leaves a truncated env file
	tmp, err := os.CreateTemp(filepath.Dir(cmd.EnvFile), filepath.Base(cmd.EnvFile)+".rotate*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := io.WriteString(tmp, block); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), cmd.EnvFile)
}

// warnOnSelfLockout warns when none of the identities that just decrypted
// the file are among the new recipients, since the rotated file would be
// unreadable to the caller. Only X25519 keys can be compared, so the check
// is skipped when any other recipient type is present.
func warnOnSelfLockout(identities []age.Identity, recipients []age.Recipient) {
	recSet := map[string]bool{}
	for _, r := range recipients {
		x, ok := r.(*age.X25519Recipient)
		if !ok {
			return
		}
		recSet[x.String()] = true
	}
	for _, id := range identities {
		if x, ok := id.(*age.X25519Identity); ok && recSet[x.Recipient().String()] {
			return
		}
	}
	slog.Warn("none of the identities are among the new recipients, the rotated file will not be readable with them")
}
