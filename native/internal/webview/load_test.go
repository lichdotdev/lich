package webview

import (
	"slices"
	"testing"
)

type step struct {
	kind int
	uri  string
}

func told(l *loads, steps ...step) []Load {
	var out []Load
	for _, s := range steps {
		if got, ok := l.step(s.kind, s.uri); ok {
			out = append(out, got)
		}
	}
	return out
}

func TestALinkClickedInThePageLoadsAndSaysWhereItLanded(t *testing.T) {
	l := &loads{}
	l.ask("https://a.example/")
	told(l, step{loadStarted, "https://a.example/"}, step{loadCommitted, "https://a.example/"}, step{loadFinished, "https://a.example/"})
	got := told(l,
		step{loadStarted, "https://a.example/next"},
		step{loadRedirected, "https://a.example/next/"},
		step{loadCommitted, "https://a.example/next/"},
		step{loadFinished, "https://a.example/next/"})
	want := []Load{{Loading: true}, {Loading: true, URL: "https://a.example/next/"}, {}}
	if !slices.Equal(got, want) {
		t.Fatalf("a clicked link told the window %+v, want %+v", got, want)
	}
	if l.url() != "https://a.example/next/" {
		t.Fatalf("page is at %q, want where the link landed", l.url())
	}
}

func TestALoadTheWindowAskedForTellsNoURL(t *testing.T) {
	l := &loads{}
	l.ask("https://a.example/")
	got := told(l, step{loadStarted, "https://a.example/"}, step{loadCommitted, "https://a.example/"}, step{loadFinished, "https://a.example/"})
	want := []Load{{Loading: true}, {}}
	if !slices.Equal(got, want) {
		t.Fatalf("the window's own load told it %+v, want %+v", got, want)
	}
}

func TestAFailedLoadStopsLoading(t *testing.T) {
	l := &loads{}
	got := told(l, step{loadStarted, "https://down.example/"}, step{loadFinished, "https://down.example/"})
	want := []Load{{Loading: true}, {}}
	if !slices.Equal(got, want) {
		t.Fatalf("a failed load told the window %+v, want %+v", got, want)
	}
}
