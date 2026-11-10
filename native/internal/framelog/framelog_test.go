package framelog

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func openTemp(t *testing.T, beat func(time.Time) bool, aliases map[string]string) (*Log, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "frames.jsonl")
	l, err := Open(path, beat, aliases)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l, path
}

func records(t *testing.T, path string) []Record {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []Record
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		out = append(out, r)
	}
	return out
}

func drawFrame(l *Log, size image.Point) {
	f := l.Begin(size)
	f.HandOff()
	f.End()
}

func TestNilLogIsDisabledAndDoesNothing(t *testing.T) {
	var l *Log
	woke := 0
	l.Invalidator(func() { woke++ })()
	l.Tagged(func() { woke++ }, "x")()
	l.Note("x")
	f := l.Begin(image.Pt(1, 1))
	f.HandOff()
	f.End()
	if woke != 2 || f != nil {
		t.Fatalf("woke %d times, frame %v", woke, f)
	}
}

func TestFrameNamesWhatAskedForIt(t *testing.T) {
	l, path := openTemp(t, nil, map[string]string{"framelog.TestFrameNamesWhatAskedForIt": "pty"})
	woke := 0
	l.Invalidator(func() { woke++ })()
	l.Tagged(func() { woke++ }, "echo")()
	drawFrame(l, image.Pt(10, 10))
	drawFrame(l, image.Pt(10, 10))
	got := records(t, path)
	if woke != 2 || len(got) != 2 {
		t.Fatalf("woke %d, %d lines", woke, len(got))
	}
	if want := []string{"echo", "pty"}; !slices.Equal(got[0].Causes, want) {
		t.Errorf("first frame causes = %v, want %v", got[0].Causes, want)
	}
	if want := []string{CauseInput}; !slices.Equal(got[1].Causes, want) {
		t.Errorf("a frame nothing asked for = %v, want %v", got[1].Causes, want)
	}
}

func TestUnaliasedInvalidatorIsNamedAfterItsCaller(t *testing.T) {
	l, path := openTemp(t, nil, nil)
	l.Invalidator(func() {})()
	drawFrame(l, image.Pt(10, 10))
	if got := records(t, path)[0].Causes; !slices.Equal(got, []string{"other:framelog.TestUnaliasedInvalidatorIsNamedAfterItsCaller"}) {
		t.Fatalf("causes = %v", got)
	}
}

func TestBeatCountsOnlyWhenItFellDueJustBeforeTheFrame(t *testing.T) {
	var due time.Time
	l, path := openTemp(t, func(at time.Time) bool { return due.After(at) }, nil)
	due = time.Now().Add(-time.Millisecond)
	drawFrame(l, image.Pt(10, 10))
	due = time.Now().Add(time.Hour)
	drawFrame(l, image.Pt(10, 10))
	due = time.Now().Add(-time.Hour)
	drawFrame(l, image.Pt(10, 10))
	got := records(t, path)
	if !slices.Equal(got[0].Causes, []string{CauseBeat}) {
		t.Errorf("frame at the beat = %v", got[0].Causes)
	}
	for i, r := range got[1:] {
		if !slices.Equal(r.Causes, []string{CauseInput}) {
			t.Errorf("frame %d off the beat = %v", i+1, r.Causes)
		}
	}
}

func TestResizeIsACauseFromTheSecondFrameOn(t *testing.T) {
	l, path := openTemp(t, nil, nil)
	drawFrame(l, image.Pt(10, 10))
	drawFrame(l, image.Pt(20, 10))
	got := records(t, path)
	if slices.Contains(got[0].Causes, CauseResize) || !slices.Contains(got[1].Causes, CauseResize) {
		t.Fatalf("causes = %v, then %v", got[0].Causes, got[1].Causes)
	}
}

func TestFrameRecordsPhasesAndAllocations(t *testing.T) {
	l, path := openTemp(t, nil, nil)
	f := l.Begin(image.Pt(10, 10))
	// Above the small-object limit, so the runtime counts each at once; small
	// objects are counted when a span is refilled and lag the frame.
	const large, largeSize = 4, 64 << 10
	sink := make([][]byte, 0, large)
	for range large {
		sink = append(sink, make([]byte, largeSize))
	}
	time.Sleep(2 * time.Millisecond)
	f.HandOff()
	time.Sleep(2 * time.Millisecond)
	f.End()
	_ = sink
	r := records(t, path)[0]
	if r.LayoutUs < 2000 || r.RenderUs < 2000 {
		t.Errorf("layout %dus, render %dus, each slept 2ms", r.LayoutUs, r.RenderUs)
	}
	if r.Allocs < large || r.AllocBytes < large*largeSize {
		t.Errorf("allocs %d (%d bytes), want the %d large objects made", r.Allocs, r.AllocBytes, large)
	}
}

func TestFramesCountPerCause(t *testing.T) {
	l, _ := openTemp(t, nil, nil)
	before := Causes.Get("tested")
	var n int64
	if before != nil {
		n = before.(interface{ Value() int64 }).Value()
	}
	l.Note("tested")
	drawFrame(l, image.Pt(10, 10))
	if got := Causes.Get("tested").(interface{ Value() int64 }).Value(); got != n+1 {
		t.Fatalf("counter = %d, want %d", got, n+1)
	}
}

func TestCloseIsNilSafeAndStopsWriting(t *testing.T) {
	var nilLog *Log
	if err := nilLog.Close(); err != nil {
		t.Fatal(err)
	}
	l, path := openTemp(t, nil, nil)
	drawFrame(l, image.Pt(10, 10))
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	drawFrame(l, image.Pt(10, 10))
	if got := len(records(t, path)); got != 1 {
		t.Fatalf("%d lines, want the one drawn before Close", got)
	}
}

func TestShortNames(t *testing.T) {
	for in, want := range map[string]string{
		"main.(*terminals).onOutput":                "terminals.onOutput",
		"main.(*model).decodeFrames.func1.1":        "model.decodeFrames",
		"github.com/x/lich/native/ui.(*Theme).Loop": "ui.Theme.Loop",
		"main.(*window).connect.gowrap1":            "window.connect",
		"main.startup":                              "startup",
	} {
		if got := short(in); got != want {
			t.Errorf("short(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSummaryRatesPerCauseAndPhases(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	enc := json.NewEncoder(&buf)
	for i := range 101 {
		causes := []string{"beat"}
		if i%2 == 0 {
			causes = []string{"beat", "echo"}
		}
		enc.Encode(Record{T: start.Add(time.Duration(i) * 10 * time.Millisecond), SinceMs: 10, Causes: causes, LayoutUs: int64(1000 + i*10), RenderUs: 500})
	}
	s, err := Read(&buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	s.Write(&out)
	for _, want := range []string{"101 frames in 1.0s: 100.0 fps", "beat  100.0", "echo   50.0"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary lacks %q:\n%s", want, out.String())
		}
	}
}

func TestReadKeepsTheLastWindow(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	enc := json.NewEncoder(&buf)
	for i := range 10 {
		enc.Encode(Record{T: start.Add(time.Duration(i) * time.Second)})
	}
	s, err := Read(&buf, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Frames) != 4 {
		t.Fatalf("kept %d frames, want the last 4 (3s back from the final one, inclusive)", len(s.Frames))
	}
}

func TestReadRefusesAMalformedLine(t *testing.T) {
	if _, err := Read(strings.NewReader("{}\nnot json\n"), 0); err == nil {
		t.Fatal("a malformed line was accepted")
	}
}
