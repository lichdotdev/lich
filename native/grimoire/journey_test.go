package grimoire

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// reference is what the approved mockup's welcome-journey.js drew from this
// package's sprites.json, run under node: a digest of every frame of every
// journey, and four key frames in full to read a failure by.
type reference struct {
	Keyframes map[string]string
	Digests   map[string]struct {
		Written, Bind, BindAwake, Unseal []string
	}
}

// writtenAt are the install fractions the reference drew a page at.
var writtenAt = []float64{0, 0.1, 0.25, 0.5, 0.6, 0.8, 1}

func readReference(t *testing.T) reference {
	t.Helper()
	data, err := os.ReadFile("testdata/journey.json")
	if err != nil {
		t.Fatal(err)
	}
	var r reference
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func text(g Grid) string {
	rows := make([]string, len(g))
	for y, row := range g {
		rows[y] = string(row)
	}
	return strings.Join(rows, "\n")
}

func digest(g Grid) string {
	sum := sha256.Sum256([]byte(text(g)))
	return hex.EncodeToString(sum[:])[:16]
}

func sameFrames(t *testing.T, name string, got []Grid, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d frames, want %d", name, len(got), len(want))
	}
	for i, g := range got {
		if digest(g) != want[i] {
			t.Fatalf("%s: frame %d differs from the mockup's:\n%s", name, i, text(g))
		}
	}
}

func TestJourneyIsTheMockupsFrameForFrame(t *testing.T) {
	ref := readReference(t)
	for k, want := range ref.Digests {
		var written []Grid
		for _, p := range writtenAt {
			written = append(written, Written(k, p))
		}
		sameFrames(t, k+" written", written, want.Written)
		sameFrames(t, k+" bind", Bind(k), want.Bind)
		sameFrames(t, k+" bind awake", BindAwake(k), want.BindAwake)
		sameFrames(t, k+" unseal", Unseal(k), want.Unseal)
	}
}

func TestKeyFramesReadAsTheMockupsStrip(t *testing.T) {
	ref := readReference(t)
	const id = "antigravity"
	cases := map[string]Grid{
		"written60": Written(id, 0.6),
		"bind6":     Bind(id)[6],
		"bind10":    Bind(id)[10],
		"unseal4":   Unseal(id)[4],
	}
	for name, g := range cases {
		if got := text(g); got != ref.Keyframes[name] {
			t.Errorf("%s:\n%s\nwant:\n%s", name, got, ref.Keyframes[name])
		}
	}
}

func TestEveryProviderIdFindsItsBook(t *testing.T) {
	for _, id := range []string{"claude", "codex", "antigravity", "opencode", "omp", "crush", "cursor", "kiro"} {
		for _, s := range []State{Ready, SignedOut, NotInstalled} {
			if g := Sprite(id, s); len(g) == 0 {
				t.Errorf("%s has no %s sprite", id, s)
			}
		}
	}
	if Has("nope") || Sprite("nope", Ready) != nil {
		t.Error("an unknown provider has a book")
	}
}

func TestEveryFrameFitsTheBox(t *testing.T) {
	for _, id := range []string{"claude", "kiro"} {
		for _, g := range append(append(Bind(id), Unseal(id)...), Box(Written(id, 0.5))) {
			if len(g) != Rows || len(g[0]) != Cols {
				t.Fatalf("%s: a %dx%d frame", id, len(g[0]), len(g))
			}
		}
	}
}
