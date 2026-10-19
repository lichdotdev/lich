package main

import "testing"

func TestWindowClass(t *testing.T) {
	cases := []struct {
		name string
		dev  string
		want string
	}{
		{"installed window matches lich.desktop", "", "lich"},
		{"dev window keeps its own", "1", "lichdev"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			getenv := func(key string) string {
				if key == "LICH_DEV" {
					return c.dev
				}
				return ""
			}
			if got := windowClass(getenv); got != c.want {
				t.Errorf("windowClass = %q, want %q", got, c.want)
			}
		})
	}
}
