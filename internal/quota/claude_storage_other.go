//go:build !darwin

package quota

func readClaudeCredentials(a Account) (claudeCredentials, string) {
	return readClaudeFile(a)
}

// ForgetClaudeLogin is the macOS Keychain's alone: elsewhere Claude Code keeps
// the login in the config directory, which goes with it.
func ForgetClaudeLogin(string) error { return nil }
