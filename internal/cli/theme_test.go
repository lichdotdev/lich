package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/themes"
)

const installedThemes = `[{"id":"light","name":"Light","scheme":"light","pair":"dark"},` +
	`{"id":"everforest-dark","name":"Everforest Dark","scheme":"dark","pair":"everforest-light"}]`

func themeLich(t *testing.T, selected string) *fakeLich {
	t.Helper()
	f := newFakeLich(t, "null")
	f.answers = map[string]answer{
		"themes.List":      {http.StatusOK, installedThemes},
		"themes.Selection": {http.StatusOK, `{"theme":"` + selected + `","followSystem":false}`},
	}
	return f
}

func TestThemeListMarksTheThemeInUse(t *testing.T) {
	code, stdout, stderr := run(t, themeLich(t, "everforest-dark"), "theme", "list")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	want := "theme\tscheme\tname\n" +
		"system\t-\tSystem, the bundled theme the OS asks for\n" +
		"light\tlight\tLight\n" +
		"everforest-dark\tdark\tEverforest Dark\t(in use)\n"
	if stdout != want {
		t.Errorf("output =\n%s\nwant\n%s", stdout, want)
	}
}

func TestThemeListJSONNamesPairsAndTheOneInUse(t *testing.T) {
	code, stdout, _ := run(t, themeLich(t, "system"), "theme", "--json", "list")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	for _, part := range []string{`"id":"system"`, `"inUse":true`, `"pair":"everforest-light"`} {
		if !strings.Contains(stdout, part) {
			t.Errorf("output %s lacks %s", stdout, part)
		}
	}
}

func TestThemeSwitchSelectsTheTheme(t *testing.T) {
	f := newFakeLich(t, "null")
	code, stdout, stderr := run(t, f, "theme", "switch", "everforest-light")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	call := f.only(t)
	want := themes.SelectOptions{ID: "everforest-light"}
	if call.method != "themes.Select" || optionsOf[themes.SelectOptions](t, call) != want {
		t.Errorf("called %s %v, want themes.Select %+v", call.method, call.args, want)
	}
	if stdout != "Switched to \"everforest-light\".\n" {
		t.Errorf("output = %q", stdout)
	}
}

func TestThemeSwitchReportsAThemeNotInstalled(t *testing.T) {
	f := newFakeLich(t, `{"error":"no theme \"nord\"; lich theme list shows the installed ones"}`)
	f.status = http.StatusInternalServerError
	code, _, stderr := run(t, f, "theme", "switch", "nord")
	if code != 1 || !strings.Contains(stderr, `no theme "nord"`) {
		t.Errorf("exit = %d, stderr = %q, want the backend's refusal", code, stderr)
	}
}

func TestThemeTakesListOrSwitchWithOneTheme(t *testing.T) {
	f := newFakeLich(t, "null")
	for _, args := range [][]string{{"theme"}, {"theme", "use", "x"}, {"theme", "switch"}, {"theme", "list", "x"}} {
		code, _, stderr := run(t, f, args...)
		if code != 1 || !strings.Contains(stderr, "usage: lich theme") {
			t.Errorf("%v: exit = %d, stderr = %q, want the usage", args, code, stderr)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("a malformed call reached the server: %v", f.calls)
	}
}
