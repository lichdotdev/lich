package xdgprofile

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// statusCommand is the CLI call that says who is logged in to a profile, and
// how to read what it printed. read gets the output whatever the exit status:
// Kiro CLI exits 1 for a logged-out profile and says so on stdout.
type statusCommand struct {
	args []string
	read func(out string) (login, error)
}

// login is who a status call found.
type login struct {
	loggedIn bool
	email    string
}

// opencodeCredentials is the count `opencode auth list` ends on, "└  2
// credentials" (measured on 1.18.35, logged out: "0 credentials"). It names no
// account, only the providers.
var opencodeCredentials = regexp.MustCompile(`(\d+) credentials?\b`)

var opencodeStatus = statusCommand{
	args: []string{"auth", "list"},
	read: func(out string) (login, error) {
		m := opencodeCredentials.FindStringSubmatch(out)
		if m == nil {
			return login{}, fmt.Errorf("no credential count in %q", strings.TrimSpace(out))
		}
		n, err := strconv.Atoi(m[1])
		return login{loggedIn: n > 0}, err
	},
}

// cursorStatus reads `cursor-agent status --format json`: logged out it is
// {"status":"unauthenticated","isAuthenticated":false,…}, indented, with exit 0
// (measured on 2026.10.01), logged in it carries userInfo.email (read in the bundle).
var cursorStatus = statusCommand{
	args: []string{"status", "--format", "json"},
	read: func(out string) (login, error) {
		var status struct {
			IsAuthenticated bool `json:"isAuthenticated"`
			UserInfo        *struct {
				Email string `json:"email"`
			} `json:"userInfo"`
		}
		if err := json.Unmarshal([]byte(jsonObject(out)), &status); err != nil {
			return login{}, err
		}
		found := login{loggedIn: status.IsAuthenticated}
		if status.UserInfo != nil {
			found.email = status.UserInfo.Email
		}
		return found, nil
	},
}

// kiroStatus reads `kiro-cli whoami --format json`: {"account":null} with exit
// 1 logged out (measured on 2.21.0). The logged-in shape was not measured, so
// an account is read for an email and logged in whatever else it holds.
var kiroStatus = statusCommand{
	args: []string{"whoami", "--format", "json"},
	read: func(out string) (login, error) {
		var status struct {
			Account *struct {
				Email string `json:"email"`
			} `json:"account"`
		}
		if err := json.Unmarshal([]byte(jsonObject(out)), &status); err != nil {
			return login{}, err
		}
		if status.Account == nil {
			return login{}, nil
		}
		return login{loggedIn: true, email: status.Account.Email}, nil
	},
}

// jsonObject is the JSON object in out, from its first line that opens one to
// the last line that closes one: a CLI may print a notice (an update, a
// migration) around it, on either stream, and Cursor CLI indents its object over
// several lines.
func jsonObject(out string) string {
	lines := strings.Split(out, "\n")
	first := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "{") })
	if first < 0 {
		return ""
	}
	last := first
	for i := first; i < len(lines); i++ {
		if strings.HasSuffix(strings.TrimSpace(lines[i]), "}") {
			last = i
		}
	}
	return strings.Join(lines[first:last+1], "\n")
}
