package main

import (
	"bufio"
	"bytes"
	"crypto/cipher"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"filippo.io/age"
	arg "github.com/alexflint/go-arg"
	"golang.org/x/crypto/chacha20poly1305"
)

type Main struct {
	Env     *Env     `arg:"subcommand:env" help:"Run a command with the decrypted env vars added to its environment"`
	Get     *Get     `arg:"subcommand:get" help:"Decrypt and print env vars"`
	Set     *Set     `arg:"subcommand:set" help:"Encrypt env vars and append them to the env file"`
	Rotate  *Rotate  `arg:"subcommand:rotate" help:"Re-encrypt all env vars into a single block for the given recipients, replacing the env file"`
	Version *Version `arg:"subcommand:version" help:"Print version"`
}

func (Main) Description() string {
	return `ace manages append-only encrypted environment variables.

Variables are encrypted with age (https://age-encryption.org) to a set of
recipients (public keys) and appended to a plain-text env file (./.env.ace
by default) that is safe to commit to version control. Anyone with the
recipients can append new values, but only holders of a matching identity
(private key) can decrypt them. Values are never modified in place: setting
an existing key appends a new value, and the latest value wins when reading.
`
}

func (Main) Epilogue() string {
	return `Getting started:
  age-keygen -o "$XDG_CONFIG_HOME/ace/identity"    create an identity (once per user/machine)
  age-keygen -y "$XDG_CONFIG_HOME/ace/identity" >> recipients.txt
                                                   register its public key as a recipient

Examples:
  ace set API_KEY=abc123    encrypt a variable and append it to ./.env.ace
  ace set < .env            encrypt variables read from stdin in .env format
                            (preferred for secrets: values stay out of shell history)
  ace get                   decrypt and print all variables readable by your identity
  ace get API_KEY           decrypt and print selected variables
  ace env -- npm start      run a command with the decrypted variables in its environment
  ace rotate                re-encrypt all variables into a single fresh block for the
                            current recipients, replacing the env file

Note that set encrypts to recipients (public keys) while get and env decrypt
with identities (private keys): to read back a value you set, the public key
of one of your identities must be listed among the recipients.

Documentation: https://github.com/middle-management/ace`
}

const (
	// v1 blocks encrypt each value on its own, so ciphertexts can be
	// swapped between variable names without failing authentication
	ACE_PREFIX_V1 = "# ace/v1:"
	// v2 blocks bind the variable name into the value's AEAD as
	// additional data, so a ciphertext only decrypts under its own name
	ACE_PREFIX_V2 = "# ace/v2:"
)

type envFileStats struct {
	blocks  int
	matched int
}

func readEnvFile(src io.Reader, identities []age.Identity, keepQuotes bool) ([]string, envFileStats, error) {
	var keys []string
	vals := map[string]string{}
	var stats envFileStats

	s := bufio.NewScanner(src)
	// large values (certificates, keystores) can exceed the default 64KB line limit
	s.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var aead cipher.AEAD
	var bindKey bool
	var lineNo int
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())

		// split on block prefix
		if strings.HasPrefix(line, "# ace/") {
			var prefix string
			switch {
			case strings.HasPrefix(line, ACE_PREFIX_V1):
				prefix = ACE_PREFIX_V1
				bindKey = false
			case strings.HasPrefix(line, ACE_PREFIX_V2):
				prefix = ACE_PREFIX_V2
				bindKey = true
			default:
				return nil, stats, fmt.Errorf("line %d: unsupported block version %q, upgrade ace to read this file", lineNo, strings.SplitN(line, ":", 2)[0])
			}
			stats.blocks++

			// base32decode and armor decode age header
			header, err := base32.StdEncoding.DecodeString(strings.TrimPrefix(line, prefix))
			if err != nil {
				return nil, stats, fmt.Errorf("line %d: block header: %w", lineNo, err)
			}

			var r io.Reader
			r = bytes.NewReader(header)

			// decrypt the block key using identities
			r, err = age.Decrypt(r, identities...)
			var noMatch *age.NoIdentityMatchError
			if errors.As(err, &noMatch) {
				// try next env block
				aead = nil
				continue
			} else if err != nil {
				return nil, stats, fmt.Errorf("line %d: block header: %w", lineNo, err)
			}
			blockKey, err := io.ReadAll(r)
			if err != nil {
				return nil, stats, fmt.Errorf("line %d: block header: %w", lineNo, err)
			}
			aead, err = chacha20poly1305.NewX(blockKey)
			if err != nil {
				return nil, stats, fmt.Errorf("line %d: block header: %w", lineNo, err)
			}
			stats.matched++
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		// decrypt each secret using block key
		pair := strings.SplitN(line, "=", 2)
		if len(pair) != 2 {
			continue
		}

		if aead == nil {
			continue
		}

		secret, err := base32.StdEncoding.DecodeString(pair[1])
		if err != nil {
			return nil, stats, fmt.Errorf("line %d: %s: %w", lineNo, pair[0], err)
		}

		if len(secret) < aead.NonceSize() {
			return nil, stats, fmt.Errorf("line %d: %s: ciphertext too short", lineNo, pair[0])
		}
		nonce, ciphertext := secret[:aead.NonceSize()], secret[aead.NonceSize():]

		var aad []byte
		if bindKey {
			aad = []byte(pair[0])
		}

		// Decrypt the message and check it wasn't tampered with.
		plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
		if err != nil {
			return nil, stats, fmt.Errorf("line %d: %s: %w", lineNo, pair[0], err)
		}

		if _, exists := vals[pair[0]]; !exists {
			keys = append(keys, pair[0])
		}
		vals[pair[0]] = string(plaintext)
	}
	if err := s.Err(); err != nil {
		return nil, stats, err
	}
	if stats.blocks > 0 && stats.matched == 0 {
		slog.Warn("no identity matched any encrypted block", "blocks", stats.blocks)
	}

	var newVars []string
	for _, k := range keys {
		if keepQuotes {
			newVars = append(newVars, k+"="+vals[k])
		} else {
			v, err := UnescapeValue(vals[k])
			if err != nil {
				return nil, stats, err
			}
			newVars = append(newVars, k+"="+v)
		}
	}

	return newVars, stats, nil
}

func readIdentities(idents []string, onMissing string) ([]age.Identity, error) {
	// resolves to $XDG_CONFIG_HOME when set, otherwise the platform default
	configDir, configDirErr := os.UserConfigDir()

	if len(idents) == 0 {
		if configDirErr != nil {
			return nil, fmt.Errorf("unable to read user config dir: %w", configDirErr)
		}
		idents = []string{filepath.Join(configDir, "ace", "identity")}
	}

	// expand like os.ExpandEnv, but resolve $XDG_CONFIG_HOME even when unset
	// without mutating the process environment (it would leak into `ace env` children)
	expand := func(path string) string {
		return os.Expand(path, func(key string) string {
			if v, ok := os.LookupEnv(key); ok {
				return v
			}
			if key == "XDG_CONFIG_HOME" && configDirErr == nil {
				return configDir
			}
			return ""
		})
	}

	var identities []age.Identity
	for _, id := range idents {
		err := func() error {
			i, err := os.Open(expand(id))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					switch onMissing {
					case "ignore":
						return nil
					case "warn", "warning":
						slog.Warn("identity not found")
						return nil
					default:
						return err
					}
				} else {
					return err
				}
			}
			defer i.Close()

			idents, err := age.ParseIdentities(i)
			if err != nil {
				return err
			}
			identities = append(identities, idents...)
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}
	return identities, nil
}

func UnescapeValue(value string) (string, error) {
	if len(value) == 0 {
		return "", nil
	}

	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if len(trimmed) == 0 {
		return value, nil
	}
	if trimmed[0] != '\'' && trimmed[0] != '"' {
		return value, nil
	}

	var unescaped strings.Builder
	var i int
	state := "unquoted"

	for i < len(value) {
		c := value[i]

		switch state {
		case "unquoted":
			if c == '\'' {
				state = "singleQuoted"
				i++
			} else if c == '"' {
				state = "doubleQuoted"
				i++
			} else if c == '\\' {
				i++
				if i >= len(value) {
					return "", fmt.Errorf("unexpected end of string")
				}
				unescaped.WriteByte(value[i])
				i++
			} else {
				unescaped.WriteByte(c)
				i++
			}
		case "singleQuoted":
			if c == '\'' {
				state = "unquoted"
				i++
			} else {
				unescaped.WriteByte(c)
				i++
			}
		case "doubleQuoted":
			if c == '"' {
				state = "unquoted"
				i++
			} else if c == '\\' {
				i++
				if i >= len(value) {
					return "", fmt.Errorf("unexpected end of string")
				}
				c2 := value[i]
				switch c2 {
				case '$', '`', '"', '\\', '\n':
					unescaped.WriteByte(c2)
				case 'n':
					unescaped.WriteByte('\n')
				case 't':
					unescaped.WriteByte('\t')
				default:
					unescaped.WriteByte('\\')
					unescaped.WriteByte(c2)
				}
				i++
			} else {
				unescaped.WriteByte(c)
				i++
			}
		}
	}

	if state != "unquoted" {
		return "", fmt.Errorf("unclosed quote in value")
	}

	return unescaped.String(), nil
}

// configurable for tests
var input io.Reader = os.Stdin
var output io.Writer = os.Stdout

// this is set using `-ldflags "-X main.version=1.2.3"`
var version string

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && version == "test" {
				a.Value = slog.TimeValue(time.Unix(0, 0))
			}
			return a
		},
	})).With("version", version))

	var args Main
	p := arg.MustParse(&args)

	err := func() error {
		switch {
		case args.Env != nil:
			return args.Env.Run()
		case args.Get != nil:
			return args.Get.Run()
		case args.Set != nil:
			return args.Set.Run()
		case args.Rotate != nil:
			return args.Rotate.Run()
		case args.Version != nil:
			args.Version.version = version
			return args.Version.Run()
		default:
			p.WriteHelp(os.Stderr)
			return nil
		}
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: "+err.Error())

		var exitErr interface {
			ExitCode() int
		}
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		} else {
			os.Exit(1)
		}
	}
}
