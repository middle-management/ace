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
	"strings"
	"time"
	"unicode"

	"filippo.io/age"
	arg "github.com/alexflint/go-arg"
	"golang.org/x/crypto/chacha20poly1305"
)

type Main struct {
	Env     *Env     `arg:"subcommand:env" help:"Expand to env and pass to command"`
	Get     *Get     `arg:"subcommand:get" help:"Decrypt env with available identities"`
	Set     *Set     `arg:"subcommand:set" help:"Append encrypted env vars to file"`
	Version *Version `arg:"subcommand:version"`
}

const ACE_PREFIX = "# ace/v1:"

func readEnvFile(src io.Reader, identities []age.Identity, keepQuotes bool) ([]string, error) {
	var keys []string
	vals := map[string]string{}

	s := bufio.NewScanner(src)
	var aead cipher.AEAD
	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		// split on ACE_PREFIX
		if strings.HasPrefix(line, ACE_PREFIX) {
			// base32decode and armor decode age header
			header, err := base32.StdEncoding.DecodeString(strings.TrimPrefix(line, ACE_PREFIX))
			if err != nil {
				return nil, err
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
				return nil, err
			}
			blockKey, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			aead, err = chacha20poly1305.NewX(blockKey)
			if err != nil {
				return nil, err
			}
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
			return nil, err
		}

		if len(secret) < aead.NonceSize() {
			return nil, fmt.Errorf("ciphertext too short")
		}
		nonce, ciphertext := secret[:aead.NonceSize()], secret[aead.NonceSize():]

		// Decrypt the message and check it wasn't tampered with.
		plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil, err
		}

		if _, exists := vals[pair[0]]; !exists {
			keys = append(keys, pair[0])
		}
		vals[pair[0]] = string(plaintext)
	}

	var newVars []string
	for _, k := range keys {
		if keepQuotes {
			newVars = append(newVars, k+"="+vals[k])
		} else {
			v, err := UnescapeValue(vals[k])
			if err != nil {
				return nil, err
			}
			newVars = append(newVars, k+"="+v)
		}
	}

	return newVars, nil
}

func readIdentities(idents []string, onMissing string) ([]age.Identity, error) {
	if _, exists := os.LookupEnv("XDG_CONFIG_HOME"); !exists {
		dir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("unable to read user config dir: %w", err)
		}
		os.Setenv("XDG_CONFIG_HOME", dir)
	}

	if len(idents) == 0 {
		idents = []string{"$XDG_CONFIG_HOME/ace/identity"}
	}

	var identities []age.Identity
	for _, id := range idents {
		err := func() error {
			i, err := os.Open(os.ExpandEnv(id))
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
