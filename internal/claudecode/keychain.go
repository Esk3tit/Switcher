package claudecode

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"
)

type Keychain interface {
	Read(context.Context, string) ([]byte, bool, error)
	Write(context.Context, string, []byte) error
	Delete(context.Context, string) error
}

type SystemKeychain struct {
	// Only tests replace Runner; production pins Apple's system executable.
	Runner func(context.Context, []string, []byte) ([]byte, error)
}

func keychainUser() string {
	if value := os.Getenv("USER"); value != "" {
		return value
	}
	if current, err := user.Current(); err == nil {
		return current.Username
	}
	return "claude-code-user"
}

func (k SystemKeychain) run(ctx context.Context, args []string, input []byte, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if k.Runner != nil {
		return k.Runner(ctx, args, input)
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/security", args...)
	cmd.Stdin = bytes.NewReader(input)
	return cmd.Output()
}

func missingKey(err error) bool {
	var exit interface{ ExitCode() int }
	return errors.As(err, &exit) && exit.ExitCode() == 44
}

func (k SystemKeychain) Read(ctx context.Context, service string) ([]byte, bool, error) {
	value, err := k.run(ctx, []string{"find-generic-password", "-a", keychainUser(), "-s", service, "-w"}, nil, 5*time.Second)
	if missingKey(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, errors.New("Claude Code Keychain is locked, denied, or unavailable; no credential fallback was used")
	}
	return bytes.TrimSuffix(value, []byte{'\n'}), true, nil
}

func securityQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

// security -i reads each command into a fixed-size line buffer.
const securityLineLimit = 4032

// The hex value shares the 1 MiB exec argument space with the environment.
const keychainValueLimit = 256 << 10

var errKeychainValueTooLarge = errors.New("native credentials exceed the Keychain write limit; use Claude Code login for this profile")

// writeCommand writes through /usr/bin/security, the only program Claude
// Code's item trusts, so macOS never asks. A record too long for security -i
// goes in the command's arguments, where it is briefly visible to ps.
func writeCommand(service string, value []byte) (args []string, input []byte, err error) {
	account := keychainUser()
	if strings.ContainsAny(service+account, "\n\r\x00") {
		return nil, nil, errors.New("invalid native Keychain item identifier")
	}
	if len(value) > keychainValueLimit {
		return nil, nil, errKeychainValueTooLarge
	}
	encoded := hex.EncodeToString(value)
	line := fmt.Sprintf("add-generic-password -U -a %s -s %s -X %s\n", securityQuote(account), securityQuote(service), encoded)
	if len(line) <= securityLineLimit {
		return []string{"-i"}, []byte(line), nil
	}
	return []string{"add-generic-password", "-U", "-a", account, "-s", service, "-X", encoded}, nil, nil
}

// Validate says, before a refresh spends its grant, whether next can be written.
func (k SystemKeychain) Validate(service string, next []byte) error {
	_, _, err := writeCommand(service, next)
	return err
}

func (k SystemKeychain) Write(ctx context.Context, service string, value []byte) error {
	args, input, err := writeCommand(service, value)
	if err != nil {
		return err
	}
	if _, err := k.run(ctx, args, input, 5*time.Second); err != nil {
		return errors.New("could not write Claude Code Keychain credentials")
	}
	got, exists, err := k.Read(ctx, service)
	if err != nil || !exists || !bytes.Equal(got, value) {
		return errors.New("Claude Code Keychain write did not verify")
	}
	return nil
}

func (k SystemKeychain) Delete(ctx context.Context, service string) error {
	_, err := k.run(ctx, []string{"delete-generic-password", "-a", keychainUser(), "-s", service}, nil, 5*time.Second)
	if err != nil && !missingKey(err) {
		return errors.New("could not clear conflicting native Claude API key")
	}
	return nil
}
