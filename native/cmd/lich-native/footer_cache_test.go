package main

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestFooterUsageCache(t *testing.T) {
	base := `"id":"s1","percent":10,"tokens":1,"window":2,"model":"m","effort":""`
	cases := []struct {
		extra string
		want  *api.SessionCache
	}{
		{`,"cache":{"read":90,"write":6,"uncached":4}`, &api.SessionCache{Read: 90, Write: 6, Uncached: 4}},
		{``, nil},
		{`,"cache":null`, nil},
		{`,"cache":"90"`, nil}, // unusable reads as absent
		{`,"cache":{"read":-1,"write":0,"uncached":4}`, nil},
	}
	for _, c := range cases {
		f := newFooterUsage()
		if _, err := f.apply(footerEvent("session-usage", "{"+base+c.extra+"}")); err != nil {
			t.Fatal(err)
		}
		u, ok := f.get("s1")
		if !ok {
			t.Fatalf("%s: the report was dropped", c.extra)
		}
		if (u.Cache == nil) != (c.want == nil) || (u.Cache != nil && *u.Cache != *c.want) {
			t.Errorf("%s: cache %+v, want %+v", c.extra, u.Cache, c.want)
		}
	}
}

func TestCacheHit(t *testing.T) {
	cases := []struct {
		cache  *api.SessionCache
		want   int
		wantOK bool
	}{
		{&api.SessionCache{Read: 118_020, Write: 2_310, Uncached: 412}, 98, true},
		{&api.SessionCache{Write: 120_739, Uncached: 3}, 0, true},
		{&api.SessionCache{Read: 1, Uncached: 1}, 50, true},
		{&api.SessionCache{}, 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		got, ok := cacheHit(c.cache)
		if got != c.want || ok != c.wantOK {
			t.Errorf("cacheHit(%+v) = %d, %v; want %d, %v", c.cache, got, ok, c.want, c.wantOK)
		}
	}
}
