package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
)

// sidebarBackend is a backend holding settings by key@scope and answering
// providers.Detect and themes.List; every other call is recorded as
// "method(args)".
type sidebarBackend struct {
	mu       sync.Mutex
	settings map[string]string
	detected []lichclient.DetectedProvider
	themes   []lichclient.Theme
	calls    []string
}

func newSidebarBackend(t *testing.T) (*sidebarBackend, *lichclient.Client) {
	t.Helper()
	f := &sidebarBackend{settings: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var args []json.RawMessage
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &args)
		_ = json.NewEncoder(w).Encode(f.answer(strings.TrimPrefix(r.URL.Path, "/rpc/"), args))
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return f, lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
}

func (f *sidebarBackend) answer(method string, args []json.RawMessage) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	str := func(i int) string {
		var s string
		_ = json.Unmarshal(args[i], &s)
		return s
	}
	switch method {
	case "store.GetSetting":
		return f.settings[str(0)+"@"+str(1)]
	case "store.SetSetting":
		f.settings[str(0)+"@"+str(1)] = str(2)
		return nil
	case "providers.Detect":
		return f.detected
	case "themes.List":
		return f.themes
	}
	raw := make([]string, len(args))
	for i, a := range args {
		raw[i] = string(a)
	}
	f.calls = append(f.calls, method+"("+strings.Join(raw, ",")+")")
	return nil
}

func (f *sidebarBackend) get(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.settings[key]
}

func TestParseSidebarWidth(t *testing.T) {
	cases := map[string]float64{
		"":     sidebarDefaultRem,
		"wide": sidebarDefaultRem,
		"-3":   sidebarDefaultRem,
		"0":    sidebarDefaultRem,
		"NaN":  sidebarDefaultRem,
		"Inf":  sidebarDefaultRem,
		"20.5": 20.5,
		"2":    sidebarMinRem,
		"99":   sidebarMaxRem,
	}
	for raw, want := range cases {
		if got := parseSidebarWidth(raw); got != want {
			t.Errorf("parseSidebarWidth(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestParseBoolSettingTakesOnlyWhatAWriteEmits(t *testing.T) {
	for raw, want := range map[string]bool{"true": true, "false": false, "": true, "1": true, "yes": true} {
		if got := parseBoolSetting(raw, true); got != want {
			t.Errorf("parseBoolSetting(%q, true) = %v", raw, got)
		}
	}
	if parseBoolSetting("1", false) {
		t.Error(`"1" pinned a toggle on`)
	}
}

func TestSidebarLayoutRoundTripsUnderTheWebKeys(t *testing.T) {
	ctx := context.Background()
	f, c := newSidebarBackend(t)
	got, err := loadSidebarLayout(ctx, c)
	if err != nil || got != (sidebarLayout{widthRem: sidebarDefaultRem, open: true}) {
		t.Fatalf("unsaved layout = %+v, %v", got, err)
	}
	if err := saveSidebarWidth(ctx, c, 21.25); err != nil {
		t.Fatal(err)
	}
	if err := saveSidebarOpen(ctx, c, false); err != nil {
		t.Fatal(err)
	}
	if f.get("lich.sidebar.width@") != "21.25" || f.get("lich.sidebar.open@") != "false" {
		t.Fatalf("stored %v", f.settings)
	}
	if got, _ := loadSidebarLayout(ctx, c); got != (sidebarLayout{widthRem: 21.25, open: false}) {
		t.Fatalf("reloaded layout = %+v", got)
	}
}

func TestCollapsedIsScopedToTheProject(t *testing.T) {
	ctx := context.Background()
	f, c := newSidebarBackend(t)
	for _, key := range []string{rootBlockKey, pinnedBlockKey} {
		if err := saveCollapsed(ctx, c, "p1", key, true); err != nil {
			t.Fatal(err)
		}
		if got, _ := loadCollapsed(ctx, c, "p1", key); !got {
			t.Errorf("p1 %s did not stay folded", key)
		}
		if got, _ := loadCollapsed(ctx, c, "p2", key); got {
			t.Errorf("folding p1's %s folded p2's", key)
		}
	}
	if f.get("lich.sidebar.collapsed."+rootBlockKey+"@p1") != "true" {
		t.Fatalf("stored %v", f.settings)
	}
	_ = saveCollapsed(ctx, c, "p1", rootBlockKey, false)
	if got, _ := loadCollapsed(ctx, c, "p1", rootBlockKey); got || f.get("lich.sidebar.collapsed."+rootBlockKey+"@p1") != "" {
		t.Fatalf("unfold left %v", f.settings)
	}
}

func TestMoveCollapsedCarriesOrDropsTheFold(t *testing.T) {
	ctx := context.Background()
	_, c := newSidebarBackend(t)
	_ = saveCollapsed(ctx, c, "p", folderKey("Apps"), true)
	if moved, err := moveCollapsed(ctx, c, "p", folderKey("Apps"), folderKey("Ops")); err != nil || !moved {
		t.Fatalf("move = %v, %v", moved, err)
	}
	if was, _ := loadCollapsed(ctx, c, "p", folderKey("Apps")); was {
		t.Fatal("the old key kept its fold")
	}
	if now, _ := loadCollapsed(ctx, c, "p", folderKey("Ops")); !now {
		t.Fatal("the fold did not follow the rename")
	}
	_, _ = moveCollapsed(ctx, c, "p", folderKey("Ops"), "")
	if now, _ := loadCollapsed(ctx, c, "p", folderKey("Ops")); now {
		t.Fatal("ungrouping kept the fold")
	}
}
