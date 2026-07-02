package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"io"
	"os"
	"strings"

	"filippo.io/age"
	"golang.org/x/crypto/chacha20poly1305"
)

type Set struct {
	RecipientFiles []string `arg:"--recipient-file,-R,separate" help:"Encrypt to recipients listed at RECIPIENT-FILE. Can be repeated. Defaults to ./recipients.txt"`
	Recipients     []string `arg:"--recipient,-r,separate" help:"Encrypt to the specified RECIPIENT. Can be repeated."`
	EnvFile        string   `arg:"--env-file,-e" default:"./.env.ace" help:"Append the encrypted variables to this file"`
	EnvPairs       []string `arg:"positional" placeholder:"KEY=VALUE" help:"Variables to encrypt. When none are given they are read from stdin in .env format, which keeps values out of shell history"`
}

func parseRecipients(recs []string, files []string) ([]age.Recipient, error) {
	if len(files) == 0 {
		files = []string{"./recipients.txt"}
	}

	var recipients []age.Recipient
	for _, r := range recs {
		rec, err := age.ParseX25519Recipient(r)
		if err != nil {
			return nil, err
		}
		recipients = append(recipients, rec)
	}
	for _, r := range files {
		rec, err := func() ([]age.Recipient, error) {
			rcp, err := os.Open(r)
			if err != nil {
				return nil, err
			}
			defer rcp.Close()
			return age.ParseRecipients(rcp)
		}()
		if err != nil {
			return nil, err
		}
		recipients = append(recipients, rec...)
	}
	return recipients, nil
}

// encryptBlock assembles a complete v2 block: a fresh block key encrypted
// to the recipients, followed by the sealed pairs and a blank line
func encryptBlock(recipients []age.Recipient, kvs [][2]string) (string, error) {
	blockKey := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(blockKey); err != nil {
		return "", err
	}

	buf := bytes.NewBuffer(nil)

	// encrypt the key using age
	err := func() error {
		w, err := age.Encrypt(buf, recipients...)
		if err != nil {
			return err
		}
		defer w.Close()

		_, err = w.Write(blockKey)
		if err != nil {
			return err
		}
		return nil
	}()
	if err != nil {
		return "", err
	}

	aead, err := chacha20poly1305.NewX(blockKey)
	if err != nil {
		return "", err
	}

	var block strings.Builder
	block.WriteString(ACE_PREFIX_V2 + base32.StdEncoding.EncodeToString(buf.Bytes()) + "\n")
	for _, kv := range kvs {
		nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(kv[1])+aead.Overhead())
		if _, err := rand.Read(nonce); err != nil {
			return "", err
		}

		// bind the variable name as additional data so the ciphertext
		// cannot be reused under another name
		secret := base32.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(kv[1]), []byte(kv[0])))
		block.WriteString(kv[0] + "=" + secret + "\n")
	}
	block.WriteString("\n")

	return block.String(), nil
}

func (cmd *Set) Run() error {
	recipients, err := parseRecipients(cmd.Recipients, cmd.RecipientFiles)
	if err != nil {
		return err
	}

	pairs := cmd.EnvPairs
	if len(pairs) == 0 {
		data, err := io.ReadAll(input)
		if err != nil {
			return err
		}
		var cur strings.Builder
		var inQuote byte
		var escaped bool
		for _, c := range data {
			if inQuote == 0 && c == '\n' {
				line := cur.String()
				cur.Reset()
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || trimmed[0] == '#' || !strings.Contains(line, "=") {
					continue
				}
				pairs = append(pairs, line)
			} else {
				cur.WriteByte(c)
				if escaped {
					escaped = false
				} else if c == '\\' && inQuote == '"' {
					escaped = true
				} else if inQuote == 0 && (c == '"' || c == '\'') {
					inQuote = c
				} else if c == inQuote {
					inQuote = 0
				}
			}
		}
		if line := cur.String(); strings.TrimSpace(line) != "" && strings.Contains(line, "=") {
			pairs = append(pairs, line)
		}
	}

	// validate all pairs before writing anything, so a bad value
	// cannot leave a partial block in the file
	var kvs [][2]string
	for _, p := range pairs {
		pair := strings.SplitN(p, "=", 2)
		if len(pair) != 2 {
			continue
		}
		if _, err := UnescapeValue(pair[1]); err != nil {
			return err
		}
		kvs = append(kvs, [2]string{pair[0], pair[1]})
	}
	if len(kvs) == 0 {
		return nil
	}

	// assemble the whole block in memory and append it in a single write
	block, err := encryptBlock(recipients, kvs)
	if err != nil {
		return err
	}

	dst, err := os.OpenFile(cmd.EnvFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(dst, block); err != nil {
		dst.Close()
		return err
	}

	return dst.Close()
}
