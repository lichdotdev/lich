package framelog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// Summary is a log read back: the frames of a window of time.
type Summary struct {
	Frames []Record
}

// Read parses a log, keeping the frames in the last window before its final
// frame; a window of zero keeps all of them.
func Read(r io.Reader, window time.Duration) (Summary, error) {
	var all []Record
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var rec Record
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			return Summary{}, fmt.Errorf("line %d: %w", len(all)+1, err)
		}
		all = append(all, rec)
	}
	if err := sc.Err(); err != nil {
		return Summary{}, err
	}
	if window > 0 && len(all) > 0 {
		from := all[len(all)-1].T.Add(-window)
		all = all[sort.Search(len(all), func(i int) bool { return !all[i].T.Before(from) }):]
	}
	return Summary{Frames: all}, nil
}

// Write prints the frame rate per cause and each phase's p50 and p95.
func (s Summary) Write(w io.Writer) {
	if len(s.Frames) < 2 {
		fmt.Fprintf(w, "%d frames: too few to measure\n", len(s.Frames))
		return
	}
	span := s.Frames[len(s.Frames)-1].T.Sub(s.Frames[0].T).Seconds()
	// The first frame opens the span, so n frames are n-1 intervals long.
	rate := func(n int) float64 { return float64(n) / span }
	fmt.Fprintf(w, "%d frames in %.1fs: %.1f fps\n\n", len(s.Frames), span, rate(len(s.Frames)-1))

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "cause\tfps\tonly this\t")
	with, only := s.byCause()
	for _, c := range slices.SortedFunc(maps.Keys(with), func(a, b string) int { return with[b] - with[a] }) {
		fmt.Fprintf(tw, "%s\t%.1f\t%.1f\t\n", c, rate(with[c]), rate(only[c]))
	}
	tw.Flush()

	fmt.Fprintln(w)
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "phase\tp50\tp95\t")
	s.phase(tw, "since last frame (ms)", func(r Record) float64 { return r.SinceMs })
	s.phase(tw, "layout (ms)", func(r Record) float64 { return float64(r.LayoutUs) / 1000 })
	s.phase(tw, "render (ms)", func(r Record) float64 { return float64(r.RenderUs) / 1000 })
	s.phase(tw, "allocs (objects)", func(r Record) float64 { return float64(r.Allocs) })
	s.phase(tw, "allocs (KiB)", func(r Record) float64 { return float64(r.AllocBytes) / 1024 })
	tw.Flush()
}

// byCause counts the frames each cause took part in, and the ones it alone
// asked for.
func (s Summary) byCause() (with, only map[string]int) {
	with, only = map[string]int{}, map[string]int{}
	for _, r := range s.Frames[1:] {
		for _, c := range r.Causes {
			with[c]++
		}
		if len(r.Causes) == 1 {
			only[r.Causes[0]]++
		}
	}
	return with, only
}

func (s Summary) phase(w io.Writer, name string, of func(Record) float64) {
	v := make([]float64, 0, len(s.Frames))
	for _, r := range s.Frames[1:] {
		v = append(v, of(r))
	}
	slices.Sort(v)
	fmt.Fprintf(w, "%s\t%s\t%s\t\n", name, format(percentile(v, 50)), format(percentile(v, 95)))
}

// percentile is the nearest-rank p-th percentile of sorted v.
func percentile(v []float64, p int) float64 {
	return v[max(0, (len(v)*p+99)/100-1)]
}

func format(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", f), "0"), ".")
}
