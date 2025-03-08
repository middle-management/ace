package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

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
		{0, []string{"ace", "env", "-e=testdata/.env.invalid.ace", "-i=testdata/identity1", "--on-missing=warn", "--", "sh", "-c", "echo $A"}, nil},
		{0, []string{"ace", "env", "-e=testdata/.env.invalid.ace", "-i=testdata/identity1", "--on-missing=ignore", "--", "sh", "-c", "echo $A"}, nil},
		{1, []string{"ace", "env", "-e=testdata/.env.invalid.ace", "--", "sh", "-c", "echo $A"}, nil},

		{0, []string{"rm", "-f", "testdata/.envi1.ace"}, nil},
		{0, []string{"ace", "set", "-e=testdata/.envi1.ace", "-R=testdata/recipients1.txt"}, strings.NewReader("X=1\nY=2\nZ=3\n# comment\ninvalid line")},
		{1, []string{"ace", "set", "-e=testdata/.envi1.ace", "-r=age10sunh5mqv3jw7audxcylw3s9redgjfhqenkuhm4v4hetg84q835qamk6x6"}, strings.NewReader("X=1\nY=2\nZ=3\n# comment\ninvalid line")},
		{0, []string{"ace", "get", "-e=testdata/.envi1.ace", "-i=testdata/identity1"}, nil},
		{0, []string{"ace", "env", "-e=testdata/.envi1.ace", "-i=testdata/identity1", "--", "sh", "-c", "echo $X"}, nil},
		{0, []string{"ace", "env", "-e=testdata/.envi1.ace", "--on-missing=warn", "--", "sh", "-c", "echo $A"}, nil},

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
