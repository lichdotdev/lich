package terminal

import (
	"encoding/base64"
	"slices"
	"testing"
	"unicode/utf16"
)

func TestServiceArgsRunTheLineWithoutAnRc(t *testing.T) {
	if got, want := serviceArgs("docker compose up", "linux"), []string{"-c", "docker compose up"}; !slices.Equal(got, want) {
		t.Errorf("serviceArgs(linux) = %q, want %q", got, want)
	}
	got := serviceArgs(`pnpm run "dev server"`, "windows")
	if len(got) != 3 || got[0] != "-NoProfile" || got[1] != "-EncodedCommand" {
		t.Fatalf("serviceArgs(windows) = %q", got)
	}
	raw, err := base64.StdEncoding.DecodeString(got[2])
	if err != nil {
		t.Fatal(err)
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
	}
	if decoded := string(utf16.Decode(units)); decoded != `pnpm run "dev server"` {
		t.Errorf("encoded command decodes to %q", decoded)
	}
}
