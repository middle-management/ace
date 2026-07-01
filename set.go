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
	EnvFile        string   `arg:"--env-file,-e" default:"./.env.ace"`
	EnvPairs       []string `arg:"positional"`
}

func (cmd *Set) Run() error {
	recs := cmd.Recipients
	files := cmd.RecipientFiles
	if len(files) == 0 {
		files = []string{"./recipients.txt"}
	}

	var recipients []age.Recipient
	for _, r := range recs {
		rec, err := age.ParseX25519Recipient(r)
		if err != nil {
			return err
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
			return err
		}
		recipients = append(recipients, rec...)
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

	blockKey := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(blockKey); err != nil {
		return err
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
		return err
	}

	aead, err := chacha20poly1305.NewX(blockKey)
	if err != nil {
		return err
	}

	// assemble the whole block in memory and append it in a single write
	var block strings.Builder
	block.WriteString(ACE_PREFIX + base32.StdEncoding.EncodeToString(buf.Bytes()) + "\n")
	for _, kv := range kvs {
		nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(kv[1])+aead.Overhead())
		if _, err := rand.Read(nonce); err != nil {
			return err
		}

		secret := base32.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(kv[1]), nil))
		block.WriteString(kv[0] + "=" + secret + "\n")
	}
	block.WriteString("\n")

	dst, err := os.OpenFile(cmd.EnvFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(dst, block.String()); err != nil {
		dst.Close()
		return err
	}

	return dst.Close()
}
