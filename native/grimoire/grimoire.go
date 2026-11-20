// Package grimoire is the Welcome's pixel art: each provider's book, as grids
// of symbols the window paints with the theme's tokens, and the frames of the
// journey from a loose page to an awake book. It is a port of the approved
// mockup's welcome-journey.js, frame for frame.
//
// A symbol is one of 1 (Accent), 5 (between Accent and MutedForeground), 3
// (MutedForeground), 2 (between MutedForeground and Foreground), 4
// (Foreground), or . (nothing): the chat flame's, plus 5.
package grimoire

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"math"
	"slices"
)

// The book's box: every frame is this size, so a page turning into a book
// never moves its cell.
const (
	Cols = 34
	Rows = 46
)

// State is the sprite of a provider at rest.
type State string

const (
	Ready        State = "ok"
	SignedOut    State = "out"
	NotInstalled State = "inst"
)

// Grid is a frame, row 0 on top, one symbol per byte.
type Grid [][]byte

func (g Grid) clone() Grid {
	c := make(Grid, len(g))
	for y, row := range g {
		c[y] = slices.Clone(row)
	}
	return c
}

func blank() Grid {
	g := make(Grid, Rows)
	for y := range g {
		g[y] = bytes.Repeat([]byte{'.'}, Cols)
	}
	return g
}

//go:embed sprites.json
var spritesJSON []byte

// sprites is each sprite key's three states.
var sprites = func() map[string]map[State][]string {
	var m map[string]map[State][]string
	if err := json.Unmarshal(spritesJSON, &m); err != nil {
		panic(err)
	}
	return m
}()

// spriteKeys are the keys of the providers whose sprite is not filed under
// their id.
var spriteKeys = map[string][]string{
	"omp":   {"pi"},
	"kiro":  {"kiro", "K"},
	"crush": {"crush", "H"},
}

// key is the sprite key of provider id, false for a provider with no sprite.
func key(id string) (string, bool) {
	for _, k := range append([]string{id}, spriteKeys[id]...) {
		if _, ok := sprites[k]; ok {
			return k, true
		}
	}
	return "", false
}

// Has reports whether provider id has a book.
func Has(id string) bool {
	_, ok := key(id)
	return ok
}

// Sprite is provider id's book in state s, nil for a provider with none.
func Sprite(id string, s State) Grid {
	k, ok := key(id)
	if !ok {
		return nil
	}
	rows := sprites[k][s]
	g := make(Grid, len(rows))
	for y, row := range rows {
		g[y] = []byte(row)
	}
	return g
}

// The medallion's center on the book, and the seal's on the page.
const (
	medalX, medalY = 17, 21
	sealX, sealY   = 12, 13
)

// down is a symbol one tone dimmer, up one tone lighter.
var (
	down = map[byte]byte{'4': '2', '2': '3', '3': '5', '5': '1', '1': '1'}
	up   = map[byte]byte{'1': '5', '5': '3', '3': '2', '2': '4', '4': '4'}
)

func shift(m map[byte]byte, c byte) byte {
	if v, ok := m[c]; ok {
		return v
	}
	return c
}

// clockwise is the angle of (x, y) around (cx, cy), 0 on top.
func clockwise(x, y, cx, cy int) float64 {
	return math.Mod(math.Atan2(float64(x-cx), -float64(y-cy))+2*math.Pi, 2*math.Pi)
}

func dist(x, y, cx, cy int) float64 { return math.Hypot(float64(x-cx), float64(y-cy)) }

func clamp01(v float64) float64 { return max(0, min(1, v)) }
