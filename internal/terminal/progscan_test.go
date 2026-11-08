package terminal

import (
	"reflect"
	"strings"
	"testing"
)

func scanAll(chunks ...string) []progSignal {
	var s progScanner
	var out []progSignal
	for _, c := range chunks {
		out = append(out, s.scan([]byte(c))...)
	}
	return out
}

func TestProgScannerReadsASequenceSplitAcrossReads(t *testing.T) {
	t.Parallel()
	whole := "text\x1b[1mbold\x1b]7501;state=done\x1b\\more"
	want := []progSignal{{payload: "7501;state=done"}}
	for cut := 1; cut < len(whole); cut++ {
		if got := scanAll(whole[:cut], whole[cut:]); !reflect.DeepEqual(got, want) {
			t.Fatalf("cut at %d: %+v, want %+v", cut, got, want)
		}
	}
}

func TestProgScannerSignals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in string
		want     []progSignal
	}{
		{"BEL ends it", "\x1b]7501;?\a", []progSignal{{payload: "7501;?", bel: true}}},
		{"C0 inside is ignored", "\x1b]7501;sta\nte=done\a", []progSignal{{payload: "7501;state=done", bel: true}}},
		{"CAN cancels", "\x1b]7501;state=done\x18\a", nil},
		{"SUB cancels", "\x1b]7501;state=done\x1a\a", nil},
		{"ESC not ST ends it", "\x1b]7501;state=done\x1b[0m", []progSignal{{payload: "7501;state=done"}}},
		{"prompt start", "\x1b]133;A\a\x1b]133;A;cl=m\a\x1b]133;B\a\x1b]133;AB\a", []progSignal{{prompt: true}, {prompt: true}}},
		{"RIS", "\x1bc", []progSignal{{reset: true}}},
		{"other OSC dropped", "\x1b]0;title\a\x1b]52;c;Zm9v\a", nil},
		{"over the limit dropped", "\x1b]7501;state=done:" + strings.Repeat("a", progMaxSequence) + "\a", nil},
	}
	for _, c := range cases {
		if got := scanAll(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func BenchmarkProgScannerTUIOutput(b *testing.B) {
	chunk := []byte(strings.Repeat("\x1b[38;5;12mhello\x1b[0m world ", 1000))
	var s progScanner
	b.SetBytes(int64(len(chunk)))
	for b.Loop() {
		s.scan(chunk)
	}
}
