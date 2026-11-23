package claudecode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"switcher/internal/store"
	"testing"
)

func TestProfilePathResolutionMatchesClaudeSwap(t *testing.T) {
	home := t.TempDir()
	lookup := func(string) (string, bool) { return "", false }
	p, err := pathsFor(home, "", lookup, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if p.ConfigFile != filepath.Join(home, ".claude.json") || p.Service != "Claude Code-credentials" || !p.Keychain {
		t.Fatal("default profile paths diverged")
	}
	custom := filepath.Join(home, "profile") + "/./"
	p, err = pathsFor(home, custom, lookup, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(custom))
	if p.Service != "Claude Code-credentials-"+hex.EncodeToString(digest[:4]) {
		t.Fatal("service hash normalized the raw path")
	}
	if p.ConfigFile != filepath.Join(filepath.Clean(custom), ".claude.json") {
		t.Fatal("custom config path wrong")
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(home, ".claude", ".config.json")
	if err := os.WriteFile(legacy, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err = pathsFor(home, "", lookup, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if p.ConfigFile != legacy || p.Keychain {
		t.Fatal("legacy config/file backend not selected")
	}
	if _, err := pathsFor(home, "relative", lookup, "darwin"); err == nil {
		t.Fatal("relative config profile accepted")
	}
	secure := func(string) (string, bool) { return "other-profile", true }
	if _, err := pathsFor(home, custom, secure, "darwin"); err == nil {
		t.Fatal("divergent secure profile accepted")
	}
	decomposed := filepath.Join(home, "e\u0301")
	composed := filepath.Join(home, "\u00e9")
	a, _ := pathsFor(home, decomposed, lookup, "darwin")
	b, _ := pathsFor(home, composed, lookup, "darwin")
	if a.Service != b.Service {
		t.Fatal("Keychain service derivation omitted NFC normalization")
	}
}

type exitCode int

func (e exitCode) Error() string { return "fixture command exit" }
func (e exitCode) ExitCode() int { return int(e) }

func TestKeychainWritesUseStdinAndVerify(t *testing.T) {
	t.Setenv("USER", "fixture-user")
	secret := []byte(`{"claudeAiOauth":{"accessToken":"fixture-sensitive","refreshToken":"fixture-grant"}}`)
	var stored []byte
	k := SystemKeychain{Runner: func(ctx context.Context, args []string, input []byte) ([]byte, error) {
		for _, arg := range args {
			if strings.Contains(arg, "fixture-sensitive") || strings.Contains(arg, hex.EncodeToString(secret)) {
				t.Fatal("credential leaked into process arguments")
			}
		}
		if len(args) == 1 && args[0] == "-i" {
			line := string(input)
			encodedValue := strings.Fields(line)[len(strings.Fields(line))-1]
			var err error
			stored, err = hex.DecodeString(encodedValue)
			if err != nil {
				t.Fatal(err)
			}
			return nil, nil
		}
		return append(append([]byte(nil), stored...), '\n'), nil
	}}
	if err := k.Write(context.Background(), "Claude Code-credentials", secret); err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(secret) {
		t.Fatal("round-trip corrupted credential")
	}
}

func TestKeychainAbsenceIsNotAReadFailure(t *testing.T) {
	for _, tc := range []struct {
		err     error
		wantErr bool
	}{{exitCode(44), false}, {exitCode(1), true}, {errors.New("fixture timeout"), true}} {
		k := SystemKeychain{Runner: func(context.Context, []string, []byte) ([]byte, error) { return nil, tc.err }}
		_, exists, err := k.Read(context.Background(), "Claude Code-credentials")
		if exists || (err != nil) != tc.wantErr {
			t.Fatalf("Keychain verdict incorrect: exists=%v err=%v", exists, err)
		}
	}
}

// fakeSecurity answers the /usr/bin/security commands SystemKeychain runs,
// keeping the items in m and recording each command's arguments.
func fakeSecurity(t *testing.T, m *memoryKeychain, calls *[][]string) func(context.Context, []string, []byte) ([]byte, error) {
	stdinWrite := regexp.MustCompile(`^add-generic-password -U -a "(?:[^"\\]|\\.)*" -s "((?:[^"\\]|\\.)*)" -X ([0-9a-f]*)\n$`)
	return func(ctx context.Context, args []string, input []byte) ([]byte, error) {
		*calls = append(*calls, append([]string(nil), args...))
		service, encoded := "", ""
		switch {
		case len(args) == 6 && args[0] == "find-generic-password" && args[5] == "-w":
			value, exists, err := m.Read(ctx, args[4])
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, exitCode(44)
			}
			return append(value, '\n'), nil
		case len(args) == 5 && args[0] == "delete-generic-password":
			return nil, m.Delete(ctx, args[4])
		case len(args) == 1 && args[0] == "-i":
			match := stdinWrite.FindSubmatch(input)
			if match == nil {
				t.Fatalf("unexpected security -i input of %d bytes", len(input))
			}
			service = strings.NewReplacer(`\\`, `\`, `\"`, `"`).Replace(string(match[1]))
			encoded = string(match[2])
		case len(args) == 8 && args[0] == "add-generic-password" && args[1] == "-U" && args[6] == "-X":
			if len(input) != 0 {
				t.Fatal("an argument write also sent stdin")
			}
			service, encoded = args[5], args[7]
		default:
			t.Fatalf("unexpected security command %q", args[0])
		}
		value, err := hex.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		return nil, m.Write(ctx, service, value)
	}
}

// Claude Code's item trusts only /usr/bin/security, so every write goes
// through it: a record too long for security -i goes in its arguments.
func TestLongKeychainWriteGoesThroughSecurityArguments(t *testing.T) {
	t.Setenv("USER", "fixture-user")
	service := "Claude Code-credentials"
	secret := []byte(`{"claudeAiOauth":{"accessToken":"fixture-sensitive"},"mcpOAuth":"` + strings.Repeat("x", 11700) + `"}`)
	m := &memoryKeychain{values: map[string][]byte{}}
	var calls [][]string
	k := SystemKeychain{Runner: fakeSecurity(t, m, &calls)}
	if err := k.Validate(service, secret); err != nil {
		t.Fatal(err)
	}
	if err := k.Write(context.Background(), service, secret); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"add-generic-password", "-U", "-a", "fixture-user", "-s", service, "-X", hex.EncodeToString(secret)},
		{"find-generic-password", "-a", "fixture-user", "-s", service, "-w"},
	}
	if !reflect.DeepEqual(calls, want) || !bytes.Equal(m.values[service], secret) {
		t.Fatalf("long write ran %d commands, stored %v", len(calls), bytes.Equal(m.values[service], secret))
	}

	// A write that does not read back is not a write.
	lossy := SystemKeychain{Runner: func(ctx context.Context, args []string, input []byte) ([]byte, error) {
		if args[0] == "find-generic-password" {
			return []byte("something else\n"), nil
		}
		return nil, nil
	}}
	if err := lossy.Write(context.Background(), service, secret); err == nil {
		t.Fatal("an unverified write was accepted")
	}

	calls = nil
	over := bytes.Repeat([]byte("a"), keychainValueLimit+1)
	if err := k.Validate(service, over); !errors.Is(err, errKeychainValueTooLarge) {
		t.Fatalf("preflight accepted a record over the limit: %v", err)
	}
	if err := k.Write(context.Background(), service, over); !errors.Is(err, errKeychainValueTooLarge) || len(calls) != 0 {
		t.Fatalf("a record over the limit reached security: %v", err)
	}
}

func TestKeychainWritesSwitchToArgumentsWhereSecurityStdinEnds(t *testing.T) {
	t.Setenv("USER", "fixture-user")
	sawStdin, sawArgs := false, false
	for size := 1950; size < 2050; size++ {
		args, input, err := writeCommand("Claude Code-credentials", bytes.Repeat([]byte("a"), size))
		if err != nil {
			t.Fatal(err)
		}
		if args[0] == "-i" {
			if sawArgs || len(input) > securityLineLimit {
				t.Fatalf("a %d-byte record overflowed security -i", size)
			}
			sawStdin = true
		} else {
			sawArgs = true
		}
	}
	if !sawStdin || !sawArgs {
		t.Fatal("the sizes did not cross the security -i line limit")
	}
}

// Switcher's own process is never the one writing Claude Code's item:
// macOS would ask the person each time, as the item does not trust it.
func TestKeychainCodeHasNoNativeWriter(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("import "+`"C"`)) || bytes.Contains(raw, []byte("SecKeychainItem"+"ModifyContent")) {
			t.Fatalf("%s writes the Keychain from Switcher's own process", file)
		}
	}
}

func TestSameUserOrganizationsAreDistinctNativeIdentities(t *testing.T) {
	a := nativeAccount("a")
	b := a
	other := identityFor("a")
	other.OrganizationUUID = "different-org"
	b.ID = "claude-other-org"
	b.ClaudeCode = &store.ClaudeCodeLogin{Credentials: rawCredential("b"), OAuthAccount: encoded(other)}
	if owns(other, a) || !owns(other, b) {
		t.Fatal("organization identity collapsed to email/UUID alone")
	}
}
