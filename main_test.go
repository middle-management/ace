package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode"

	"github.com/middle-management/ace/internal/test"
)

func TestAce(t *testing.T) {
	t.Run("set with missing default recipient file path", func(t *testing.T) {
		cmd := &Set{EnvFile: "testdata/.env.invalid.ace", RecipientFiles: []string{}, EnvPairs: []string{"A=1", "B=2"}}
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected an error due to missing recipients file, but none occurred")
		}
	})
	t.Run("get with invalid identity file path", func(t *testing.T) {
		cmd := &Get{EnvFile: "testdata/.env.invalid.ace", Identities: []string{"testdata/nonexistent_identity.txt"}}
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected an error due to missing identity file, but none occurred")
		}
	})
	t.Run("single recipient", func(t *testing.T) {
		os.Remove("testdata/.env1.ace")
		{
			cmd := &Set{EnvFile: "testdata/.env1.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{"A=1", "B=2", "C=1 2 3 ", "D ignored"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}

		{
			input = strings.NewReader("X=1\nY=2\nZ=3\n# comment\ninvalid line")
			cmd := &Set{EnvFile: "testdata/.env1.ace", RecipientFiles: []string{"testdata/recipients1.txt"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}

		{
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env1.ace", Identities: []string{"testdata/identity1"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		}
	})

	t.Run("multiple recipients", func(t *testing.T) {
		os.Remove("testdata/.env2.ace")
		{
			cmd := &Set{EnvFile: "testdata/.env2.ace", RecipientFiles: []string{"testdata/recipients12.txt"}, EnvPairs: []string{"A=1", "B=2", "C=1 2 3 "}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		{
			cmd := &Set{EnvFile: "testdata/.env2.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{"A=2", "D=3"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		{
			cmd := &Set{EnvFile: "testdata/.env2.ace", RecipientFiles: []string{"testdata/recipients13.txt"}, EnvPairs: []string{"E=5"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		t.Run("identity1", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env2.ace", Identities: []string{"testdata/identity1"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
		t.Run("identity2", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env2.ace", Identities: []string{"testdata/identity2"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
		t.Run("identity3", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env2.ace", Identities: []string{"testdata/identity3"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
	})

	t.Run("env", func(t *testing.T) {
		os.Remove("testdata/.env3.ace")
		{
			cmd := &Set{EnvFile: "testdata/.env3.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{"A=1", "B=2", "C=1 2 3 "}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}

		t.Run("identity1", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Env{EnvFile: "testdata/.env3.ace", Identities: []string{"testdata/identity1"}, Command: []string{"sh", "-c", "echo $A $B $C"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
		t.Run("identity2", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Env{EnvFile: "testdata/.env3.ace", Identities: []string{"testdata/identity2"}, Command: []string{"sh", "-c", "echo $A $B $C"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
		t.Run("env-file on-missing=error", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Env{EnvFile: "testdata/.env.not-found.ace", Identities: []string{"testdata/identity2"}, Command: []string{"sh", "-c", "echo $A $B $C"}}
			err := cmd.Run()
			if err == nil {
				t.Fatal("expected not such file or directory")
			}
			test.Snapshot(t, buf.Bytes())
		})

		t.Run("env-file on-missing=warn", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			t.Setenv("A", "woop")
			cmd := &Env{OnMissing: "warn", EnvFile: "testdata/.env.not-found.ace", Identities: []string{"testdata/identity2"}, Command: []string{"sh", "-c", "echo $A $B $C"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})

		t.Run("env-file on-missing=ignore", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			t.Setenv("A", "woop")
			cmd := &Env{OnMissing: "ignore", EnvFile: "testdata/.env.not-found.ace", Identities: []string{"testdata/identity2"}, Command: []string{"sh", "-c", "echo $A $B $C"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})

		t.Run("identity on-missing=error", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Env{EnvFile: "testdata/.env3.ace", Identities: []string{"testdata/identitynot-found"}, Command: []string{"sh", "-c", "echo $A $B $C"}}
			err := cmd.Run()
			if err == nil {
				t.Fatal("expected not such file or directory")
			}
			test.Snapshot(t, buf.Bytes())
		})

		t.Run("identity on-missing=warn", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			t.Setenv("A", "woop")
			cmd := &Env{OnMissing: "warn", EnvFile: "testdata/.env3.ace", Identities: []string{"testdata/identitynot-found"}, Command: []string{"sh", "-c", "echo $A $B $C"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})

		t.Run("identity on-missing=ignore", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			t.Setenv("A", "woop")
			cmd := &Env{OnMissing: "ignore", EnvFile: "testdata/.env3.ace", Identities: []string{"testdata/identitynot-found"}, Command: []string{"sh", "-c", "echo $A $B $C"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
	})

	t.Run("multiple recipients repeated flags", func(t *testing.T) {
		os.Remove("testdata/.env4.ace")
		{
			cmd := &Set{EnvFile: "testdata/.env4.ace", RecipientFiles: []string{"testdata/recipients1.txt", "testdata/recipients2.txt"}, EnvPairs: []string{"A=1", "B=2", "C=1 2 3 "}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		{
			cmd := &Set{EnvFile: "testdata/.env4.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{"A=2", "D=3"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		{
			cmd := &Set{EnvFile: "testdata/.env4.ace", RecipientFiles: []string{"testdata/recipients2.txt"}, EnvPairs: []string{"C=333 "}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		t.Run("identity1", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env4.ace", Identities: []string{"testdata/identity1"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
		t.Run("identity2", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env4.ace", Identities: []string{"testdata/identity2"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
		t.Run("identity1,identity2", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env4.ace", Identities: []string{"testdata/identity1", "testdata/identity2"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
		t.Run("identity2,identity1", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env4.ace", Identities: []string{"testdata/identity2", "testdata/identity1"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
	})
	t.Run("set with invalid pair writes nothing", func(t *testing.T) {
		os.Remove("testdata/.env_invalid_pair.ace")
		cmd := &Set{EnvFile: "testdata/.env_invalid_pair.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{"A=1", `BAD="unclosed`}}
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected an error due to unclosed quote, but none occurred")
		}
		if _, err := os.Stat("testdata/.env_invalid_pair.ace"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("expected no file to be written when a pair is invalid")
		}
	})

	t.Run("set with invalid key writes nothing", func(t *testing.T) {
		for name, pair := range map[string]string{
			"newline":    "A\nB=1",
			"space":      "A B=1",
			"tab":        "A\tB=1",
			"comment":    "#A=1",
			"empty name": "=1",
		} {
			t.Run(name, func(t *testing.T) {
				os.Remove("testdata/.env_invalid_key.ace")
				cmd := &Set{EnvFile: "testdata/.env_invalid_key.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{pair}}
				err := cmd.Run()
				if err == nil {
					t.Fatalf("expected an error for key of %q, but none occurred", pair)
				}
				if _, err := os.Stat("testdata/.env_invalid_key.ace"); !errors.Is(err, fs.ErrNotExist) {
					t.Fatal("expected no file to be written when a key is invalid")
				}
			})
		}
	})

	t.Run("set with no pairs writes nothing", func(t *testing.T) {
		os.Remove("testdata/.env_no_pairs.ace")
		input = strings.NewReader("# only a comment\nno equals sign\n")
		cmd := &Set{EnvFile: "testdata/.env_no_pairs.ace", RecipientFiles: []string{"testdata/recipients1.txt"}}
		err := cmd.Run()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat("testdata/.env_no_pairs.ace"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("expected no file to be written when there are no pairs")
		}
	})

	t.Run("large value roundtrip", func(t *testing.T) {
		os.Remove("testdata/.env_large.ace")
		large := strings.Repeat("0123456789", 10_000)
		{
			cmd := &Set{EnvFile: "testdata/.env_large.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{"LARGE=" + large}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		buf := &bytes.Buffer{}
		output = buf
		cmd := &Get{EnvFile: "testdata/.env_large.ace", Identities: []string{"testdata/identity1"}}
		err := cmd.Run()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := buf.String(), "LARGE="+large+"\n"; got != want {
			t.Fatalf("large value did not roundtrip, got %d bytes want %d bytes", len(got), len(want))
		}
	})

	t.Run("reads v1 format files", func(t *testing.T) {
		// testdata/env_v1 is a committed fixture written by an older
		// (v1-only) build: two blocks for recipient1, one for recipient2
		t.Run("identity1", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/env_v1", Identities: []string{"testdata/identity1"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := buf.String(), "A=1\nB=2\nC=\"hello world\"\n"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
		t.Run("identity2", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/env_v1", Identities: []string{"testdata/identity2"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := buf.String(), "D=only-for-identity2\n"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	})

	t.Run("v2 rejects ciphertexts swapped between names", func(t *testing.T) {
		os.Remove("testdata/.env_swap.ace")
		{
			cmd := &Set{EnvFile: "testdata/.env_swap.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{"A=1", "B=2"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}

		data, err := os.ReadFile("testdata/.env_swap.ace")
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(data), "\n")
		idxA, idxB := -1, -1
		for i, l := range lines {
			if strings.HasPrefix(l, "A=") {
				idxA = i
			}
			if strings.HasPrefix(l, "B=") {
				idxB = i
			}
		}
		if idxA < 0 || idxB < 0 {
			t.Fatal("expected A and B lines in the env file")
		}
		valA := strings.TrimPrefix(lines[idxA], "A=")
		valB := strings.TrimPrefix(lines[idxB], "B=")
		lines[idxA] = "A=" + valB
		lines[idxB] = "B=" + valA
		if err := os.WriteFile("testdata/.env_swap.ace", []byte(strings.Join(lines, "\n")), 0644); err != nil {
			t.Fatal(err)
		}

		buf := &bytes.Buffer{}
		output = buf
		cmd := &Get{EnvFile: "testdata/.env_swap.ace", Identities: []string{"testdata/identity1"}}
		err = cmd.Run()
		if err == nil {
			t.Fatalf("expected decryption to fail for swapped ciphertexts, got: %q", buf.String())
		}
	})

	t.Run("rotate", func(t *testing.T) {
		os.Remove("testdata/.env_rotate.ace")
		{
			cmd := &Set{EnvFile: "testdata/.env_rotate.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{"A=1", "B=2"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		{
			cmd := &Set{EnvFile: "testdata/.env_rotate.ace", RecipientFiles: []string{"testdata/recipients12.txt"}, EnvPairs: []string{"A=3", "C=\"multi\nline\""}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		{
			cmd := &Rotate{EnvFile: "testdata/.env_rotate.ace", RecipientFiles: []string{"testdata/recipients2.txt"}, Identities: []string{"testdata/identity1"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}

		data, err := os.ReadFile("testdata/.env_rotate.ace")
		if err != nil {
			t.Fatal(err)
		}
		if got, want := strings.Count(string(data), ACE_PREFIX_V2), 1; got != want {
			t.Fatalf("expected a single block after rotate, got %d", got)
		}

		t.Run("new recipient reads all vars", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env_rotate.ace", Identities: []string{"testdata/identity2"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := buf.String(), "A=3\nB=2\nC=\"multi\nline\"\n"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})

		t.Run("old recipient no longer matches", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env_rotate.ace", Identities: []string{"testdata/identity1"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			if buf.String() != "" {
				t.Fatalf("expected no output, got %q", buf.String())
			}
		})
	})

	t.Run("rotate refuses when a block cannot be decrypted", func(t *testing.T) {
		os.Remove("testdata/.env_rotate_refuse.ace")
		{
			cmd := &Set{EnvFile: "testdata/.env_rotate_refuse.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, EnvPairs: []string{"A=1"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		{
			cmd := &Set{EnvFile: "testdata/.env_rotate_refuse.ace", RecipientFiles: []string{"testdata/recipients2.txt"}, EnvPairs: []string{"B=2"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}
		before, err := os.ReadFile("testdata/.env_rotate_refuse.ace")
		if err != nil {
			t.Fatal(err)
		}

		cmd := &Rotate{EnvFile: "testdata/.env_rotate_refuse.ace", RecipientFiles: []string{"testdata/recipients1.txt"}, Identities: []string{"testdata/identity1"}}
		err = cmd.Run()
		if err == nil {
			t.Fatal("expected an error, identity1 cannot decrypt the recipients2 block")
		}

		after, err := os.ReadFile("testdata/.env_rotate_refuse.ace")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("expected env file to be unchanged after a refused rotate")
		}
	})

	t.Run("unsupported block version", func(t *testing.T) {
		os.Remove("testdata/.env_future.ace")
		if err := os.WriteFile("testdata/.env_future.ace", []byte("# ace/v9:XXXX\nA=YYYY\n"), 0644); err != nil {
			t.Fatal(err)
		}
		cmd := &Get{EnvFile: "testdata/.env_future.ace", Identities: []string{"testdata/identity1"}}
		err := cmd.Run()
		if err == nil || !strings.Contains(err.Error(), "unsupported block version") {
			t.Fatalf("expected unsupported block version error, got %v", err)
		}
	})

	t.Run("quoted and escaped values", func(t *testing.T) {
		os.Remove("testdata/.env_quotes.ace")
		{
			cmd := &Set{
				EnvFile:        "testdata/.env_quotes.ace",
				Recipients:     []string{},
				RecipientFiles: []string{"testdata/recipients1.txt"},
				EnvPairs: []string{
					"SIMPLE_QUOTE='single quoted value'",
					"DOUBLE_QUOTE=\"double quoted value\"",
					"ESCAPED_QUOTE=\"value with \\\"escaped\\\" quotes\"",
					"MIXED_QUOTES=\"'single' and \"double\" quotes\"",
					"MULTILINE=\"line1\nline2\nline3\"",
					"SPECIAL_CHARS=\"!@#$%^&*()_+-={}[]|\\:;<>,.?/~`\"",
					"ESCAPED_NEWLINE=\"line1\\nline2\\nline3\"",
					"SPACE_IN_VALUE=value with spaces",
					"EQUALS_IN_VALUE=\"key=value\"",
				},
			}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
		}

		t.Run("get", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Get{EnvFile: "testdata/.env_quotes.ace", Identities: []string{"testdata/identity1"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})

		t.Run("env", func(t *testing.T) {
			buf := &bytes.Buffer{}
			output = buf
			cmd := &Env{
				EnvFile:    "testdata/.env_quotes.ace",
				Identities: []string{"testdata/identity1"},
				Command:    []string{"sh", "-c", `echo SIMPLE_QUOTE="$SIMPLE_QUOTE"; echo DOUBLE_QUOTE="$DOUBLE_QUOTE"; echo ESCAPED_QUOTE="$ESCAPED_QUOTE"; echo MIXED_QUOTES="$MIXED_QUOTES"; echo MULTILINE="$MULTILINE"; echo SPECIAL_CHARS="$SPECIAL_CHARS"; echo ESCAPED_NEWLINE="$ESCAPED_NEWLINE"; echo SPACE_IN_VALUE="$SPACE_IN_VALUE"; echo EQUALS_IN_VALUE="$EQUALS_IN_VALUE"`},
			}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			test.Snapshot(t, buf.Bytes())
		})
	})
}

func TestMultilineStdin(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "double quoted multiline",
			input:    "KEY1=value1\nCERT=\"-----BEGIN CERTIFICATE-----\nMIIBkTCB+w==\n-----END CERTIFICATE-----\"\nKEY2=value2",
			expected: []string{"KEY1=value1", "-----BEGIN CERTIFICATE-----", "-----END CERTIFICATE-----", "KEY2=value2"},
		},
		{
			name:     "single quoted multiline",
			input:    "KEY='line1\nline2'\n",
			expected: []string{"KEY='line1\nline2'"},
		},
		{
			name:     "escaped quote in double quotes",
			input:    "KEY=\"value with \\\" escaped\"\n",
			expected: []string{"KEY=\"value with \\\" escaped\""},
		},
		{
			name:     "single quote inside double quotes",
			input:    "KEY=\"it's fine\"\n",
			expected: []string{"KEY=\"it's fine\""},
		},
		{
			name:     "empty line in multiline value",
			input:    "KEY=\"line1\n\nline3\"\n",
			expected: []string{"KEY=\"line1\n\nline3\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Remove("testdata/.env_multiline.ace")

			input = strings.NewReader(tt.input)
			cmd := &Set{EnvFile: "testdata/.env_multiline.ace", RecipientFiles: []string{"testdata/recipients1.txt"}}
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}

			buf := &bytes.Buffer{}
			output = buf
			getCmd := &Get{EnvFile: "testdata/.env_multiline.ace", Identities: []string{"testdata/identity1"}}
			err = getCmd.Run()
			if err != nil {
				t.Fatal(err)
			}

			got := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(got, exp) {
					t.Errorf("expected %q in output, got: %s", exp, got)
				}
			}
		})
	}
}

func TestIntegration(t *testing.T) {
	if os.Getenv("ACE_TESTBIN") == "" {
		t.Skip("Not running integration tests")
	}
	tests := []struct {
		ExpectedExitCode int
		Args             []string
		Stdin            io.Reader
	}{
		{0, []string{"ace"}, nil},
		{0, []string{"ace", "version"}, nil},
		{1, []string{"ace", "set", "-e=testdata/.env.invalid.ace", "A=1", "B=2"}, nil},
		{1, []string{"ace", "get", "-e=testdata/.env.invalid.ace", "-i=testdata/nonexistent_identity.txt"}, nil},
		{1, []string{"ace", "set", "-e=testdata/.env1.ace", "-r=invalid"}, nil},
		{1, []string{"ace", "set", "-e=testdata/.env1.ace", "-R=testdata/recipients1.txt", "BAD KEY=1"}, nil},
		{0, []string{"ace", "env", "-e=testdata/.env.invalid.ace", "-i=testdata/identity1", "--on-missing=warn", "--", "sh", "-c", "echo $A"}, nil},
		{0, []string{"ace", "env", "-e=testdata/.env.invalid.ace", "-i=testdata/identity1", "--on-missing=ignore", "--", "sh", "-c", "echo $A"}, nil},
		{1, []string{"ace", "env", "-e=testdata/.env.invalid.ace", "--", "sh", "-c", "echo $A"}, nil},

		{0, []string{"rm", "-f", "testdata/.envi1.ace"}, nil},
		{0, []string{"ace", "set", "-e=testdata/.envi1.ace", "-R=testdata/recipients1.txt"}, strings.NewReader("X=1\nY=2\nZ=3\n# comment\ninvalid line")},
		{1, []string{"ace", "set", "-e=testdata/.envi1.ace", "-r=age10sunh5mqv3jw7audxcylw3s9redgjfhqenkuhm4v4hetg84q835qamk6x6"}, strings.NewReader("X=1\nY=2\nZ=3\n# comment\ninvalid line")},
		{0, []string{"ace", "get", "-e=testdata/.envi1.ace", "-i=testdata/identity1"}, nil},
		{0, []string{"ace", "env", "-e=testdata/.envi1.ace", "-i=testdata/identity1", "--", "sh", "-c", "echo $X"}, nil},
		{0, []string{"ace", "env", "-e=testdata/.envi1.ace", "--on-missing=warn", "--", "sh", "-c", "echo $A"}, nil},

		{0, []string{"rm", "-f", "testdata/.envi5.ace"}, nil},
		{0, []string{"ace", "set", "-e=testdata/.envi5.ace", "-R=testdata/recipients1.txt"}, strings.NewReader("KEY1=value1\nCERT=\"-----BEGIN CERTIFICATE-----\nMIIBkTCB+w==\n-----END CERTIFICATE-----\"\nKEY2=value2")},
		{0, []string{"ace", "get", "-e=testdata/.envi5.ace", "-i=testdata/identity1"}, nil},

		{0, []string{"rm", "-f", "testdata/.envi3.ace"}, nil},
		{0, []string{"ace", "set", "-e=testdata/.envi3.ace", "-R=testdata/recipients1.txt", "A=1", "B=2", "C=1 2 3 "}, nil},
		{0, []string{"ace", "get", "-e=testdata/.envi3.ace", "-i=testdata/identity1", "A"}, nil},

		{0, []string{"rm", "-f", "testdata/.envi4.ace"}, nil},
		{0, []string{"ace", "set", "-e=testdata/.envi4.ace", "-R=testdata/recipients1.txt", "-R=testdata/recipients2.txt", "A=1", "B=2", "C=1 2 3 "}, nil},
		{0, []string{"ace", "set", "-e=testdata/.envi4.ace", "-R=testdata/recipients1.txt", "A=2", "D=3"}, nil},
		{0, []string{"ace", "set", "-e=testdata/.envi4.ace", "-R=testdata/recipients2.txt", "C=333 "}, nil},
		{0, []string{"ace", "get", "-e=testdata/.envi4.ace", "-i=testdata/identity1"}, nil},
		{0, []string{"ace", "get", "-e=testdata/.envi4.ace", "-i=testdata/identity2"}, nil},
		{0, []string{"ace", "get", "-e=testdata/.envi4.ace", "-i=testdata/identity1", "-i=testdata/identity2"}, nil},
		{0, []string{"ace", "get", "-e=testdata/.envi4.ace", "-i=testdata/identity2", "-i=testdata/identity1"}, nil},

		{0, []string{"ace", "get", "-e=testdata/env_v1", "-i=testdata/identity1"}, nil},
		{0, []string{"ace", "get", "-e=testdata/env_v1", "-i=testdata/identity2"}, nil},

		{0, []string{"rm", "-f", "testdata/.envi6.ace"}, nil},
		{0, []string{"ace", "set", "-e=testdata/.envi6.ace", "-R=testdata/recipients1.txt", "A=1", "B=2"}, nil},
		{0, []string{"ace", "set", "-e=testdata/.envi6.ace", "-R=testdata/recipients2.txt", "C=3"}, nil},
		{1, []string{"ace", "rotate", "-e=testdata/.envi6.ace", "-R=testdata/recipients1.txt", "-i=testdata/identity1"}, nil},
		{0, []string{"ace", "rotate", "-e=testdata/.envi6.ace", "-R=testdata/recipients2.txt", "-i=testdata/identity1", "-i=testdata/identity2"}, nil},
		{0, []string{"ace", "get", "-e=testdata/.envi6.ace", "-i=testdata/identity2"}, nil},

		{0, []string{"rm", "-f", "testdata/.env_quotes.ace"}, nil},
		{0, []string{"ace", "set", "-e=testdata/.env_quotes.ace", "-R=testdata/recipients1.txt",
			"SIMPLE_QUOTE='single quoted value'",
			"DOUBLE_QUOTE=\"double quoted value\"",
			"ESCAPED_QUOTE=\"value with \\\"escaped\\\" quotes\"",
			"MIXED_QUOTES=\"'single' and \"double\" quotes\"",
		}, nil},
		{0, []string{"ace", "set", "-e=testdata/.env_quotes.ace", "-R=testdata/recipients1.txt",
			"MULTILINE=\"line1\nline2\nline3\"",
			"SPECIAL_CHARS=\"!@#$%^&*()_+-={}[]|\\:;<>,.?/~`\"",
			"ESCAPED_NEWLINE=\"line1\\nline2\\nline3\"",
			"SPACE_IN_VALUE=value with spaces",
			"EQUALS_IN_VALUE=\"key=value\"",
		}, nil},
		{0, []string{"ace", "set", "-e=testdata/.env_quotes.ace", "-R=testdata/recipients1.txt",
			`PLAIN_JSON={"name":"John","age":30,"city":"New York"}`,
			`QUOTED_JSON='{"name":"John","age":30,"city":"New York"}'`,
		}, nil},
		{0, []string{"ace", "set", "-e=testdata/.env_quotes.ace", "-R=testdata/recipients1.txt",
			`DOUBLE_QUOTED_JSON="{\"name\":\"John\",\"age\":30,\"city\":\"New York\"}"`,
			`NESTED_JSON={"user":{"name":"John","details":{"age":30,"active":true}}}`,
		}, nil},
		{0, []string{"ace", "set", "-e=testdata/.env_quotes.ace", "-R=testdata/recipients1.txt",
			`JSON_ARRAY=["apple","banana","cherry"]`,
			`JSON_SPECIAL={"message":"Hello, world!","symbols":"!@#$%^&*()"}`,
			`JSON_WHITESPACE={"desc":"Line 1\\nLine 2\\tTabbed"}`,
		}, nil},
		{0, []string{"ace", "set", "-e=testdata/.env_quotes.ace", "-R=testdata/recipients1.txt",
			`COMPLEX_JSON={"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}],"metadata":{"version":"1.0","generated_at":"2023-01-01"}}`,
		}, nil},
		{0, []string{"ace", "get", "-e=testdata/.env_quotes.ace", "-i=testdata/identity1"}, nil},
		{0, []string{"ace", "env", "-e=testdata/.env_quotes.ace", "-i=testdata/identity1", "--", "sh", "-c", `echo "$SIMPLE_QUOTE"; echo "$DOUBLE_QUOTE"`}, nil},
		{0, []string{"ace", "env", "-e=testdata/.env_quotes.ace", "-i=testdata/identity1", "--", "sh", "-c", `echo "$ESCAPED_QUOTE"; echo "$MIXED_QUOTES"; echo "$MULTILINE"`}, nil},
		{0, []string{"ace", "env", "-e=testdata/.env_quotes.ace", "-i=testdata/identity1", "--", "sh", "-c", `echo "$SPECIAL_CHARS"; echo "$ESCAPED_NEWLINE"`}, nil},
		{0, []string{"ace", "env", "-e=testdata/.env_quotes.ace", "-i=testdata/identity1", "--", "sh", "-c", `echo "$SPACE_IN_VALUE"; echo "$EQUALS_IN_VALUE"`}, nil},
		{0, []string{"ace", "env", "-e=testdata/.env_quotes.ace", "-i=testdata/identity1", "--", "sh", "-c", `echo "$PLAIN_JSON";echo "$QUOTED_JSON";echo "$DOUBLE_QUOTED_JSON";echo "$NESTED_JSON";echo "$JSON_ARRAY";echo "$JSON_SPECIAL";echo "$JSON_WHITESPACE";echo "$COMPLEX_JSON";`}, nil},
		{0, []string{"ace", "env", "-e=testdata/.env1.ace", "-i=testdata/identity1", "--", "sh", "-c", "exit 0"}, nil},
		{1, []string{"ace", "env", "-e=testdata/.env1.ace", "-i=testdata/identity1", "--", "sh", "-c", "exit 1"}, nil},
		{42, []string{"ace", "env", "-e=testdata/.env1.ace", "-i=testdata/identity1", "--", "sh", "-c", "exit 42"}, nil},
		{143, []string{"ace", "env", "-e=testdata/.env1.ace", "-i=testdata/identity1", "--", "sh", "-c", "kill -TERM $$"}, nil},
	}
	coverDir := os.Getenv("GOCOVERDIR")
	if coverDir == "" {
		coverDir = ".coverdata/" + strconv.FormatInt(time.Now().Unix(), 10)
		_ = os.MkdirAll(coverDir, 0755)
	}
	for _, tt := range tests {
		t.Run(sanitizeTestName(strings.Join(tt.Args, " ")), func(t *testing.T) {
			if tt.Args[0] == "ace" {
				tt.Args[0] = os.Getenv("ACE_TESTBIN")
			}
			cmd := exec.Command(tt.Args[0], tt.Args[1:]...)
			cmd.Stdin = tt.Stdin
			cmd.Env = []string{
				"GOCOVERDIR=" + coverDir,
				"PATH=" + os.Getenv("PATH"),
				"TZ=UTC",
				"HOME=/tmp",
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Log(err)
			}
			if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() != tt.ExpectedExitCode {
				t.Errorf("Unexpected exit code %d, want %d", cmd.ProcessState.ExitCode(), tt.ExpectedExitCode)
			}
			test.Snapshot(t, out)
		})
	}
	t.Run("coverage", func(t *testing.T) {
		out, err := exec.Command("go", "tool", "covdata", "func", "-i="+coverDir).CombinedOutput()
		if err != nil {
			t.Log(err)
		}
		test.Snapshot(t, out)
	})
}

// TestSignalForwarding checks that a signal sent to ace itself (not the
// terminal foreground group, e.g. docker stop signalling PID 1) reaches
// the child, and that the child's exit code is propagated
func TestSignalForwarding(t *testing.T) {
	bin := os.Getenv("ACE_TESTBIN")
	if bin == "" {
		t.Skip("Not running integration tests")
	}
	if runtime.GOOS == "windows" {
		t.Skip("signal forwarding is POSIX-only")
	}

	ready := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command(bin, "env", "--on-missing=ignore", "-e=testdata/.env.absent.ace", "--",
		"sh", "-c", `trap 'exit 7' TERM; : > "$READY"; sleep 30 & wait $!`)
	cmd.Env = append(os.Environ(), "READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			t.Fatal("child shell never became ready")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
			t.Fatalf("expected exit code 7 from the child's TERM trap, got %v", err)
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("ace did not exit after SIGTERM, signal was not forwarded")
	}
}

func FuzzUnescapeValue(f *testing.F) {
	for _, seed := range []string{
		"", "plain value", `"double quoted"`, `'single quoted'`,
		`"escaped \" quote"`, "\"multi\nline\"", `'unclosed`, `"trailing \`,
		`  "leading space"`, `mixed 'quotes' "here"`, `back\slash`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		unescaped, err := UnescapeValue(value)
		if err != nil {
			return
		}
		// values that don't start with a quote must pass through unchanged
		trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
		if len(trimmed) > 0 && trimmed[0] != '\'' && trimmed[0] != '"' && unescaped != value {
			t.Fatalf("unquoted value %q was altered to %q", value, unescaped)
		}
	})
}

func sanitizeTestName(name string) string {
	// Replace problematic characters with underscores
	replacements := []struct {
		old string
		new string
	}{
		{`"`, "_quote_"},     // Double quotes
		{`'`, "_squote_"},    // Single quotes
		{`\`, "_slash_"},     // Backslashes
		{`$`, "_dollar_"},    // Dollar signs
		{`;`, "_semicolon_"}, // Semicolons
		{`|`, "_pipe_"},      // Pipes
		{`<`, "_lt_"},        // Less than
		{`>`, "_gt_"},        // Greater than
		{`*`, "_star_"},      // Asterisks
		{`?`, "_qmark_"},     // Question marks
		{`:`, "_colon_"},     // Colons
		{`/`, "_"},
	}

	result := name
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.old, r.new)
	}

	// Limit length to avoid potential path length issues
	if len(result) > 100 {
		result = result[:100]
	}

	return result
}
