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
		rcp, err := os.Open(r)
		if err != nil {
			return err
		}
		defer rcp.Close()

		rec, err := age.ParseRecipients(rcp)
		if err != nil {
			return err
		}
		recipients = append(recipients, rec...)
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

	dst, err := os.OpenFile(cmd.EnvFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.WriteString(dst, ACE_PREFIX+base32.StdEncoding.EncodeToString(buf.Bytes())+"\n")
	if err != nil {
		return err
	}

	aead, err := chacha20poly1305.NewX(blockKey)
	if err != nil {
		return err
	}

	pairs := cmd.EnvPairs
	if len(pairs) == 0 {
		data, _ := io.ReadAll(input)
		var cur strings.Builder
		var inQuote byte
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
				if inQuote == 0 && (c == '"' || c == '\'') {
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

	for _, p := range pairs {
		pair := strings.SplitN(p, "=", 2)
		if len(pair) != 2 {
			continue
		}

		_, err := UnescapeValue(pair[1])
		if err != nil {
			return err
		}

		nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(pair[1])+aead.Overhead())
		if _, err := rand.Read(nonce); err != nil {
			return err
		}

		secret := base32.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(pair[1]), nil))
		_, err = io.WriteString(dst, pair[0]+"="+secret+"\n")
		if err != nil {
			return err
		}
	}
	_, err = io.WriteString(dst, "\n")
	if err != nil {
		return err
	}

	return nil
}
