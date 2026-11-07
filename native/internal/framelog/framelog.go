// Package framelog writes one JSON line per frame the window draws, with the
// causes that asked for it, so a frame rate can be explained rather than only
// counted. A nil *Log is a disabled log: every method is a no-op that costs
// nothing, so callers never test for it.
package framelog

import (
	"encoding/json"
	"expvar"
	"image"
	"log"
	"os"
	"runtime"
	"runtime/metrics"
	"slices"
	"strings"
	"sync"
	"time"
)

// Causes counts the frames each cause took part in, served at /debug/vars
// beside the frame count. A frame with two causes counts in both.
var Causes = expvar.NewMap("frame_causes")

// The causes the log names itself. CauseInput is a frame no recorded request
// explains: input events, and timed requests (tooltip delays, clocks) that
// nothing here tags.
const (
	CauseInput  = "input"
	CauseBeat   = "beat"
	CauseResize = "resize"
	CauseConfig = "config"
	CauseDrop   = "drop"
	causeOther  = "other:"
)

// beatSlack is how long after a looping frame fell due a frame still counts
// as drawn for it: the timer fires on the due instant, the frame starts a
// scheduling delay later.
const beatSlack = 8 * time.Millisecond

const (
	metricAllocObjects = "/gc/heap/allocs:objects"
	metricAllocBytes   = "/gc/heap/allocs:bytes"
)

// Record is one line of the log.
type Record struct {
	T time.Time `json:"t"`
	// SinceMs is the time since the previous frame began.
	SinceMs float64  `json:"since_ms"`
	Causes  []string `json:"causes"`
	// LayoutUs is the frame event to the hand-off to Gio: input handling,
	// layout and building the ops. RenderUs is that hand-off: the GPU draw,
	// up to the point the window thread lets the frame loop go on.
	// Presenting happens after that on the window thread and is not seen.
	LayoutUs int64 `json:"layout_us"`
	RenderUs int64 `json:"render_us"`
	// Allocs and AllocBytes are the process's heap allocations between the
	// frame event and its end, background goroutines included.
	Allocs     uint64 `json:"allocs"`
	AllocBytes uint64 `json:"alloc_bytes"`
}

// Log is an open frame log.
type Log struct {
	out         *os.File
	beatPending func(time.Time) bool
	aliases     map[string]string

	mu      sync.Mutex
	pending map[string]struct{}

	// The frame in flight is owned by the frame loop, which runs one at a
	// time; reusing it keeps the log's own allocations out of the count.
	cur     Frame
	lastAt  time.Time
	size    image.Point
	samples [2]metrics.Sample
}

// Frame is the frame being drawn between Begin and End.
type Frame struct {
	log      *Log
	rec      Record
	begin    time.Time
	handOff  time.Time
	allocsAt [2]uint64
}

// Open starts a log at path, truncating what a previous run left there.
// beatPending is the window's looping-animation beat (ui.LoopBeat.Pending),
// from which a frame drawn for it is told apart. aliases maps the short name
// of a function that invalidates ("terminals.onOutput") to the cause its
// frames carry; the rest are tagged "other:" with the function's name.
func Open(path string, beatPending func(time.Time) bool, aliases map[string]string) (*Log, error) {
	out, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	l := &Log{out: out, beatPending: beatPending, aliases: aliases, pending: map[string]struct{}{}}
	l.samples[0].Name, l.samples[1].Name = metricAllocObjects, metricAllocBytes
	l.cur.log = l
	return l, nil
}

// Invalidator returns wake tagged: each call records the function that made
// it as a cause of the next frame, so the window's one Invalidate stays the
// only call every goroutine makes.
func (l *Log) Invalidator(wake func()) func() {
	if l == nil {
		return wake
	}
	return func() {
		l.Note(l.name(callerOf(2)))
		wake()
	}
}

// Tagged returns wake recording cause for the call, for a source that needs a
// name of its own where the calling function says too little.
func (l *Log) Tagged(wake func(), cause string) func() {
	if l == nil {
		return wake
	}
	return func() {
		l.Note(cause)
		wake()
	}
}

// Note records cause for the next frame.
func (l *Log) Note(cause string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.pending[cause] = struct{}{}
	l.mu.Unlock()
}

// Begin starts the frame for a window of size, taking every cause noted since
// the last one.
func (l *Log) Begin(size image.Point) *Frame {
	if l == nil {
		return nil
	}
	now := time.Now()
	l.mu.Lock()
	causes := make([]string, 0, len(l.pending)+2)
	for c := range l.pending {
		causes = append(causes, c)
		delete(l.pending, c)
	}
	l.mu.Unlock()
	if l.beatPending != nil && l.beatPending(now.Add(-beatSlack)) && !l.beatPending(now) {
		causes = append(causes, CauseBeat)
	}
	if size != l.size && !l.size.Eq(image.Point{}) {
		causes = append(causes, CauseResize)
	}
	l.size = size
	if len(causes) == 0 {
		causes = append(causes, CauseInput)
	}
	slices.Sort(causes)
	f := &l.cur
	f.rec = Record{T: now, Causes: causes}
	if !l.lastAt.IsZero() {
		f.rec.SinceMs = ms(now.Sub(l.lastAt))
	}
	l.lastAt = now
	f.begin = now
	l.readAllocs(&f.allocsAt)
	return f
}

// HandOff marks the end of layout, just before the ops go to Gio.
func (f *Frame) HandOff() {
	if f == nil {
		return
	}
	f.handOff = time.Now()
}

// End closes the frame, once Gio has drawn it, and writes its line.
func (f *Frame) End() {
	if f == nil {
		return
	}
	end := time.Now()
	var after [2]uint64
	f.log.readAllocs(&after)
	f.rec.LayoutUs = f.handOff.Sub(f.begin).Microseconds()
	f.rec.RenderUs = end.Sub(f.handOff).Microseconds()
	f.rec.Allocs, f.rec.AllocBytes = after[0]-f.allocsAt[0], after[1]-f.allocsAt[1]
	for _, c := range f.rec.Causes {
		Causes.Add(c, 1)
	}
	f.log.write(&f.rec)
}

func (l *Log) write(r *Record) {
	if l.out == nil {
		return
	}
	line, err := json.Marshal(r)
	if err == nil {
		_, err = l.out.Write(append(line, '\n'))
	}
	if err != nil {
		log.Printf("frame log: %v; it stops here", err)
		l.out.Close()
		l.out = nil
	}
}

func (l *Log) readAllocs(into *[2]uint64) {
	metrics.Read(l.samples[:])
	for i, s := range l.samples {
		into[i] = s.Value.Uint64()
	}
}

func (l *Log) name(fn string) string {
	if cause, ok := l.aliases[fn]; ok {
		return cause
	}
	return causeOther + fn
}

// callerOf is the short name of the function skip frames above its caller.
func callerOf(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	return short(runtime.FuncForPC(pc).Name())
}

// short turns "github.com/x/y/main.(*terminals).onOutput.func1" into
// "terminals.onOutput": no package path, no pointer receiver syntax, no
// closure or goroutine wrapper suffix.
func short(full string) string {
	if i := strings.LastIndex(full, "/"); i >= 0 {
		full = full[i+1:]
	}
	full = strings.TrimPrefix(full, "main.")
	full = strings.NewReplacer("(*", "", ")", "").Replace(full)
	for {
		i := strings.LastIndex(full, ".")
		if i < 0 || !isWrapperSuffix(full[i+1:]) {
			return full
		}
		full = full[:i]
	}
}

// isWrapperSuffix reports the compiler's names for a closure (func1, then 1
// inside it) and for a go statement's wrapper (gowrap1).
func isWrapperSuffix(s string) bool {
	for _, p := range []string{"func", "gowrap", ""} {
		if rest, ok := strings.CutPrefix(s, p); ok && rest != "" && strings.Trim(rest, "0123456789") == "" {
			return true
		}
	}
	return false
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
