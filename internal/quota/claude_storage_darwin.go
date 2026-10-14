//go:build darwin

package quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"regexp"
	"time"
)

const (
	// keychainTimeout bounds the /usr/bin/security call, which runs while Plans
	// holds the cache lock: every other session's reading waits behind it, so a
	// Keychain prompting for an unlock must cost a gauge, never the panel.
	keychainTimeout      = 2 * time.Second
	keychainItemNotFound = 44
)

var claudeKeychainUser = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func readClaudeCredentials(a Account) (claudeCredentials, string) {
	return readClaudeKeychain(a, func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
		defer cancel()
		return exec.CommandContext(ctx, "/usr/bin/security", args...).Output()
	})
}

// keychainAccount is the Keychain account Claude Code files its login under:
// UZ in the cited CLI selects USER, then os.userInfo().username, sanitizing
// unsupported names to claude-code-user.
func keychainAccount(a Account) (string, error) {
	name := a.lookup(accountUserVar)
	if name == "" {
		current, err := user.Current()
		if err != nil {
			return "", err
		}
		name = current.Username
	}
	if !claudeKeychainUser.MatchString(name) {
		name = "claude-code-user"
	}
	return name, nil
}

func readClaudeKeychain(a Account, run func(...string) ([]byte, error)) (claudeCredentials, string) {
	var creds claudeCredentials
	// Match the account as well as the service.
	name, err := keychainAccount(a)
	if err != nil {
		return creds, StatusUnknown
	}
	data, err := run("find-generic-password", "-a", name, "-w", "-s", claudeKeychainService(a))
	if err != nil {
		// security truncates errSecItemNotFound (-25300) to exit status 44.
		// Only an absent item allows Claude's plaintext fallback; a locked or
		// refused Keychain must never send a different file login's quota.
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == keychainItemNotFound {
			return readClaudeFile(a)
		}
		return creds, StatusUnknown
	}
	if json.Unmarshal(data, &creds) != nil || creds.OAuth.AccessToken == "" {
		return creds, StatusUnknown
	}
	return creds, StatusOK
}

// ForgetClaudeLogin deletes the Keychain item Claude Code keeps the login of
// the config directory dir in, which outlives the directory: deleting a
// profile's directory alone leaves its login behind. An item already gone is
// not an error.
func ForgetClaudeLogin(dir string) error {
	return forgetClaudeKeychain(dir, func(args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
		defer cancel()
		return exec.CommandContext(ctx, "/usr/bin/security", args...).Run()
	})
}

func forgetClaudeKeychain(dir string, run func(...string) error) error {
	a := Account{Env: map[string]string{claudeDirVar: dir}}
	name, err := keychainAccount(Account{})
	if err != nil {
		return fmt.Errorf("forget Claude login: %w", err)
	}
	err = run("delete-generic-password", "-a", name, "-s", claudeKeychainService(a))
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == keychainItemNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("forget Claude login: %w", err)
	}
	return nil
}
