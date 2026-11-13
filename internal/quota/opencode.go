package quota

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/providers"
	_ "modernc.org/sqlite"
)

const (
	opencodeUsageURL = "https://opencode.ai/zen/go/v1/usage"
	// opencodeGoEntry is the auth.json entry `opencode auth login` writes for
	// the Go subscription, beside the Zen key and any other provider's.
	opencodeGoEntry = "opencode-go"
	// monthlyWindow is Go's third window. Go reports it by name, not length.
	monthlyWindow = 30 * dailyWindow
)

// opencodeCredential is one entry of opencode's auth.json; only an "api" entry
// carries a key.
type opencodeCredential struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

// opencodeUsage is the Go usage route's answer. Its shape is the opencode
// console's (routes/zen/go/v1/usage.ts) as the Orca project parses it: no Go
// subscription was at hand to measure a 200 here. Its errors were measured:
// 403 EntitlementError for a key with no Go subscription, 401 AuthError for a
// key the console does not know.
type opencodeUsage struct {
	Usage *struct {
		Rolling *opencodeMeter `json:"rolling"`
		Weekly  *opencodeMeter `json:"weekly"`
		Monthly *opencodeMeter `json:"monthly"`
	} `json:"usage"`
}

type opencodeMeter struct {
	Percent  *float64 `json:"percent"`
	ResetsAt string   `json:"resetsAt"`
}

// opencodePlan reads the Go subscription with the key opencode stored for it.
// opencode otherwise runs on the user's own provider keys, so no Go key is no
// plan, and opencode is left out.
func (s *Service) opencodePlan(a Account) Plan {
	p := plan(providers.OpenCode)
	if a.hidden() {
		return unknown(p)
	}
	path, ok := harnessFile(a, dataHomeVar, filepath.Join(".local", "share"), "opencode", "auth.json")
	if !ok {
		return failed(p)
	}
	key := opencodeGoKey(path)
	if key == "" {
		return noPlan
	}
	var usage opencodeUsage
	authOK, err := s.readJSON(s.opencodeURL, key, map[string]string{"Accept": "application/json"}, &usage)
	if !authOK {
		return signedOut(p)
	}
	if err != nil {
		return failed(p)
	}
	p.Plan = "Go"
	p.Windows = usage.windows()
	if len(p.Windows) == 0 {
		return failed(p)
	}
	return p
}

// windows is the five-hour and weekly windows Go always meters, then the
// monthly one when the plan has it. A reply missing either of the first two is
// not a usage reply, and draws nothing.
func (u opencodeUsage) windows() []Window {
	if u.Usage == nil {
		return nil
	}
	meters := []struct {
		meter   *opencodeMeter
		seconds int
	}{
		{u.Usage.Rolling, sessionWindow},
		{u.Usage.Weekly, weeklyWindow},
		{u.Usage.Monthly, monthlyWindow},
	}
	var out []Window
	for i, m := range meters {
		if m.meter == nil || m.meter.Percent == nil {
			if i < 2 {
				return nil
			}
			continue
		}
		out = append(out, Window{
			Kind:     codexKind(m.seconds),
			Seconds:  m.seconds,
			Percent:  percent(*m.meter.Percent),
			ResetsAt: m.meter.ResetsAt,
		})
	}
	return out
}

// opencodeDBBusyMS caps the wait on a database opencode is writing to; a read
// that gives up costs one gauge refresh, not the plan.
const opencodeDBBusyMS = 500

// opencode2GoKeyQuery reads the Go key opencode 2 keeps in opencode.db rather
// than auth.json: an active `credential` row for the opencode-go integration
// whose value is {"type":"key","key":…} (measured on 2.2.5, whose login left a
// 1.x auth.json as it found it).
const opencode2GoKeyQuery = `SELECT json_extract(value, '$.key') FROM credential
	WHERE integration_id = 'opencode-go' AND active = 1 AND json_extract(value, '$.type') = 'key'
	ORDER BY time_updated DESC LIMIT 1`

// opencodeGoKey is the Go key beside authPath: opencode 2's first, since 2.x
// stops writing auth.json and a 1.x key left there may be one the user has
// since replaced, then the one 1.x wrote. "" when neither holds one.
func opencodeGoKey(authPath string) string {
	if key := opencode2GoKey(filepath.Join(filepath.Dir(authPath), "opencode.db")); key != "" {
		return key
	}
	var creds map[string]opencodeCredential
	readCredentials(authPath, &creds)
	if login := creds[opencodeGoEntry]; login.Type == "api" {
		return login.Key
	}
	return ""
}

// opencode2GoKey reads the database read-only; a 1.x database, whose
// credential table has another shape, answers nothing.
func opencode2GoKey(dbPath string) string {
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(%d)", dbPath, opencodeDBBusyMS))
	if err != nil {
		return ""
	}
	defer func() { _ = db.Close() }()
	var key sql.NullString
	if db.QueryRow(opencode2GoKeyQuery).Scan(&key) != nil {
		return ""
	}
	return key.String
}
