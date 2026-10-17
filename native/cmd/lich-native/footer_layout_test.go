package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/native/lichclient"
)

// fakeSettings serves store.GetSetting and store.SetSetting from an in-memory
// table keyed by "scope/key". failing makes every call answer HTTP 500.
type fakeSettings struct {
	mu      sync.Mutex
	values  map[string]string
	failing bool
}

func (f *fakeSettings) client(t *testing.T) *lichclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var args []string
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &args)
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.failing {
			http.Error(w, `{"error":"down"}`, http.StatusInternalServerError)
			return
		}
		switch r.URL.Path {
		case "/rpc/store.GetSetting":
			_ = json.NewEncoder(w).Encode(f.values[args[1]+"/"+args[0]])
		case "/rpc/store.SetSetting":
			f.values[args[1]+"/"+args[0]] = args[2]
			_ = json.NewEncoder(w).Encode(nil)
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
}

func (f *fakeSettings) get(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.values["/"+key]
}

var sampleLayout = footerLayout{
	Left:  []string{"attach", "files", "changes"},
	Right: []string{"model", "context", "plan"},
}

func TestParseFooterLayoutRejectsMalformed(t *testing.T) {
	for _, raw := range []string{"", "{", "null", "[]", `{"left":[]}`, `{"left":[],"right":false}`} {
		if l, ok := parseFooterLayout(raw); ok {
			t.Errorf("parseFooterLayout(%q) = %+v, want rejected", raw, l)
		}
	}
}

func TestParseFooterLayoutKeepsEmptySidesDropsUnknownAndDuplicates(t *testing.T) {
	cases := []struct {
		raw  string
		want footerLayout
	}{
		{`{"left":[],"right":[]}`, footerLayout{Left: []string{}, Right: []string{}}},
		{`{"left":["model","future",2,"model"],"right":["model","cost","files"]}`,
			footerLayout{Left: []string{"model"}, Right: []string{"cost", "files"}}},
		// The removed Settings item is discarded from either side.
		{`{"left":["settings","attach"],"right":["model","settings"]}`,
			footerLayout{Left: []string{"attach"}, Right: []string{"model"}}},
	}
	for _, c := range cases {
		got, ok := parseFooterLayout(c.raw)
		if !ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseFooterLayout(%s) = %+v, %v; want %+v", c.raw, got, ok, c.want)
		}
	}
}

func TestResolveFooterLayoutMigratesVisibilityWithoutRestoringHidden(t *testing.T) {
	got := resolveFooterLayout(nil, footerVisibility{Plan: true, Clock: true}, false, true)
	want := footerLayout{Left: defaultFooterLayout().Left, Right: []string{"checkout", "plan", "cost", "clock"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	stored := sampleLayout
	if got := resolveFooterLayout(&stored, footerVisibility{Clock: true}, false, false); !reflect.DeepEqual(got, sampleLayout) {
		t.Errorf("a saved layout was rewritten: %+v", got)
	}
}

func TestResolveFooterLayoutDefaultIsTheWebDefault(t *testing.T) {
	if got := resolveFooterLayout(nil, defaultFooterVisibility, true, false); !reflect.DeepEqual(got, defaultFooterLayout()) {
		t.Errorf("got %+v, want %+v", got, defaultFooterLayout())
	}
}

func TestBackendCostChoiceWinsOverASavedLayout(t *testing.T) {
	off := footerLayout{Left: []string{"cost", "model"}, Right: []string{}}
	if got := resolveFooterLayout(&off, defaultFooterVisibility, true, false); !reflect.DeepEqual(got, footerLayout{Left: []string{"model"}, Right: []string{}}) {
		t.Errorf("cost off: got %+v", got)
	}
	on := footerLayout{Left: []string{"model"}, Right: []string{}}
	if got := resolveFooterLayout(&on, defaultFooterVisibility, true, true); !reflect.DeepEqual(got, footerLayout{Left: []string{"model"}, Right: []string{"cost"}}) {
		t.Errorf("cost on: got %+v", got)
	}
}

func TestMoveFooterItemLeavesTheSourceAlone(t *testing.T) {
	cases := []struct {
		id, over string
		want     footerLayout
	}{
		{"attach", "changes", footerLayout{Left: []string{"files", "changes", "attach"}, Right: sampleLayout.Right}},
		{"context", "model", footerLayout{Left: sampleLayout.Left, Right: []string{"context", "model", "plan"}}},
		{"model", "files", footerLayout{Left: []string{"attach", "model", "files", "changes"}, Right: []string{"context", "plan"}}},
		{"model", "left", footerLayout{Left: []string{"attach", "files", "changes", "model"}, Right: []string{"context", "plan"}}},
		{"clock", "right", footerLayout{Left: sampleLayout.Left, Right: []string{"model", "context", "plan", "clock"}}},
		{"model", "available", footerLayout{Left: sampleLayout.Left, Right: []string{"context", "plan"}}},
		{"model", "clock", footerLayout{Left: sampleLayout.Left, Right: []string{"context", "plan"}}},
	}
	for _, c := range cases {
		before, _ := json.Marshal(sampleLayout)
		got, moved := moveFooterItem(sampleLayout, c.id, c.over)
		if !moved || !reflect.DeepEqual(got, c.want) {
			t.Errorf("move %s onto %s = %+v, %v; want %+v", c.id, c.over, got, moved, c.want)
		}
		if after, _ := json.Marshal(sampleLayout); string(after) != string(before) {
			t.Errorf("move %s onto %s changed the source to %s", c.id, c.over, after)
		}
	}
}

func TestMoveFooterItemInvalidOrCancelledIsNoMove(t *testing.T) {
	got, moved := moveFooterItem(footerLayout{Left: []string{}, Right: []string{}}, "model", "left")
	if !moved || !reflect.DeepEqual(got, footerLayout{Left: []string{"model"}, Right: []string{}}) {
		t.Errorf("first item onto an empty side: %+v, %v", got, moved)
	}
	for _, c := range [][2]string{{"unknown", "left"}, {"model", "outside"}, {"model", "model"}, {"clock", "available"}, {"settings", "left"}} {
		if got, moved := moveFooterItem(sampleLayout, c[0], c[1]); moved || !reflect.DeepEqual(got, sampleLayout) {
			t.Errorf("move %s onto %s = %+v, %v; want no move", c[0], c[1], got, moved)
		}
	}
}

func TestEveryItemMovesBetweenSidesWithoutDuplication(t *testing.T) {
	count := func(l footerLayout, id string) (n int) {
		for _, item := range slices.Concat(l.Left, l.Right) {
			if item == id {
				n++
			}
		}
		return n
	}
	for _, item := range footerItems {
		next, _ := moveFooterItem(sampleLayout, item.ID, footerLeft)
		if footerZone(next, item.ID) != footerLeft {
			t.Errorf("%s did not reach the left", item.ID)
		}
		next, _ = moveFooterItem(next, item.ID, footerRight)
		if footerZone(next, item.ID) != footerRight || count(next, item.ID) != 1 {
			t.Errorf("%s: %+v", item.ID, next)
		}
		next, _ = moveFooterItem(next, item.ID, footerAvailable)
		if next.has(item.ID) {
			t.Errorf("%s still placed after moving to available", item.ID)
		}
	}
}

func TestFooterLayoutPersistsBothSidesInOrder(t *testing.T) {
	f := &fakeSettings{values: map[string]string{}}
	c := f.client(t)
	ctx := context.Background()
	if got, err := loadFooterLayout(ctx, c, false); err != nil || !reflect.DeepEqual(got, defaultFooterLayout()) {
		t.Fatalf("unsaved layout = %+v, %v; want the default", got, err)
	}
	if err := saveFooterLayout(ctx, c, sampleLayout); err != nil {
		t.Fatal(err)
	}
	if got, err := loadFooterLayout(ctx, c, false); err != nil || !reflect.DeepEqual(got, sampleLayout) {
		t.Fatalf("reloaded %+v, %v; want %+v", got, err, sampleLayout)
	}
}

func TestFooterLayoutSavesAnEmptiedSideAsAnArray(t *testing.T) {
	f := &fakeSettings{values: map[string]string{}}
	c := f.client(t)
	if err := saveFooterLayout(context.Background(), c, footerLayout{Left: nil, Right: []string{"clock"}}); err != nil {
		t.Fatal(err)
	}
	if got := f.get(footerLayoutKey); got != `{"left":[],"right":["clock"]}` {
		t.Errorf("saved %s", got)
	}
}
