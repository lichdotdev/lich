package extensions

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/extension"
)

const (
	burstRenders = 2000
	// burstDeadline is generous for a burst that is over in milliseconds: it
	// stands for "bounded", not for how fast a machine is.
	burstDeadline = 5 * time.Second
	toastEvery    = 100
)

func renderLine(t *testing.T, surface, text string) string {
	t.Helper()
	line := encodeLine(t, extension.Render.New(extension.RenderData{Surface: surface, Root: extension.BodyLine.New(extension.TextProps{Text: text})}))
	return string(line) + "\n"
}

// renderedText is the text of a delivered render, with the surface it named.
func renderedText(t *testing.T, raw json.RawMessage) (surface, text string, ok bool) {
	t.Helper()
	var msg extension.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("a delivered message does not decode: %v", err)
	}
	data, isRender := msg.Data.(extension.RenderData)
	if !isRender {
		return "", "", false
	}
	props, _ := data.Root.Props.(extension.TextProps)
	return data.Surface, props.Text, true
}

// renderArrivals are the texts of the renders delivered for surface, in order,
// with when each arrived.
func (r *recorder) renderArrivals(t *testing.T, surface string) ([]string, []time.Time) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var texts []string
	var times []time.Time
	for i, m := range r.messages {
		if got, text, ok := renderedText(t, m.Message); ok && got == surface {
			texts, times = append(texts, text), append(times, r.arrived[i])
		}
	}
	return texts, times
}

func TestARenderBurstArrivesAsItsNewestRender(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeScript, func(m *extension.Manifest) {
		m.Surfaces = append(m.Surfaces, extension.Surface{ID: "other", Title: "Other"})
	})
	var script strings.Builder
	var wantToasts []string
	for i := range burstRenders {
		fmt.Fprintf(&script, "%s%s", renderLine(t, "main", fmt.Sprintf("r%d", i)), renderLine(t, "other", fmt.Sprintf("r%d", i)))
		fmt.Fprint(&script, renderLine(t, fmt.Sprintf("undeclared-%d", i), fmt.Sprintf("r%d", i)))
		if i%toastEvery == 0 {
			title := fmt.Sprintf("toast %d", i)
			wantToasts = append(wantToasts, title)
			script.WriteString(toastLine(title) + "\n")
		}
	}
	writeFile(t, filepath.Join(root, scriptFile), script.String())
	begun := time.Now()
	_, r := boot(t, dir)

	newest := fmt.Sprintf("r%d", burstRenders-1)
	for _, surface := range []string{"main", "other"} {
		waitUntil(t, surface+"'s newest render", func() bool {
			texts, _ := r.renderArrivals(t, surface)
			return len(texts) > 0 && texts[len(texts)-1] == newest
		})
	}
	// The undeclared surfaces share one gate, so their newest is the one
	// that arrives, whatever surface it named.
	waitUntil(t, "the newest undeclared render", func() bool {
		texts, _ := r.renderArrivals(t, fmt.Sprintf("undeclared-%d", burstRenders-1))
		return slices.Contains(texts, newest)
	})
	if elapsed := time.Since(begun); elapsed > burstDeadline {
		t.Errorf("the newest render took %s", elapsed)
	}

	for _, surface := range []string{"main", "other"} {
		texts, times := r.renderArrivals(t, surface)
		if len(texts) < 2 {
			t.Errorf("%s: %d renders delivered, want the first and the newest", surface, len(texts))
		}
		// Sends are at least renderInterval apart, so n renders span
		// n-1 intervals.
		span := times[len(times)-1].Sub(times[0])
		if bound := int(span/renderInterval) + 1; len(texts) > bound {
			t.Errorf("%s: %d renders in %s, the bound is %d", surface, len(texts), span, bound)
		}
		if !slices.IsSorted(sortedByNumber(t, texts)) {
			t.Errorf("%s: renders arrived out of order: %v", surface, texts)
		}
	}
	if got := r.titles(t); !slices.Equal(got, wantToasts) {
		t.Errorf("toasts were coalesced or reordered: got %d, want %d", len(got), len(wantToasts))
	}
}

func sortedByNumber(t *testing.T, texts []string) []int {
	t.Helper()
	numbers := make([]int, 0, len(texts))
	for _, text := range texts {
		var n int
		if _, err := fmt.Sscanf(text, "r%d", &n); err != nil {
			t.Fatal(err)
		}
		numbers = append(numbers, n)
	}
	return numbers
}
