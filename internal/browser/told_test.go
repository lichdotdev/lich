package browser

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestAnActionLeavesOutTheElementListTheAgentHeardLast(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	// Each read hands a list of its own, as Chrome's do.
	list := func(save Interactive) []Interactive {
		save.Index, save.Tag, save.Text = 1, "button", "Save"
		return []Interactive{{Index: 0, Tag: "a", Text: "Home", Href: "/"}, save}
	}
	p.info = PageInfo{Interactive: list(Interactive{})}
	click := func() api.BrowserPageInfo {
		t.Helper()
		info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#b"}})
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	if first := click(); first.InteractiveSame || len(first.Interactive) != 2 {
		t.Fatalf("the first answer left the list out: %+v", first)
	}
	p.info.Interactive = list(Interactive{Rect: Rect{X: 40}})
	if again := click(); !again.InteractiveSame || again.Interactive != nil {
		t.Fatalf("a list the agent heard, moved only, came again: %+v", again)
	}
	if again := click(); !again.InteractiveSame {
		t.Fatalf("a third answer of the same list repeated it: %+v", again)
	}
	filled := true
	p.info.Interactive = list(Interactive{Filled: &filled})
	if changed := click(); changed.InteractiveSame || len(changed.Interactive) != 2 {
		t.Fatalf("a list whose field was filled was left out: %+v", changed)
	}
	read, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if read.InteractiveSame || len(read.Interactive) != 2 {
		t.Fatalf("a read the agent asked for left the list out: %+v", read)
	}
	if after := click(); !after.InteractiveSame {
		t.Fatalf("an action after a read repeated the list it gave: %+v", after)
	}
}

func TestAnEmptyListIsNeverCalledTheSame(t *testing.T) {
	s, _, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#b"}})
		if err != nil {
			t.Fatal(err)
		}
		if info.InteractiveSame {
			t.Fatalf("a page with no elements answered the same list: %+v", info)
		}
	}
}

func TestAnAnnotatedScreenshotTellsNoList(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	click := func() api.BrowserPageInfo {
		t.Helper()
		info, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#b"}})
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	p.info = PageInfo{Interactive: []Interactive{{Index: 0, Tag: "a", Text: "Home"}}}
	click()
	p.info = PageInfo{Interactive: []Interactive{{Index: 0, Tag: "button", Text: "Save"}}}
	if _, err := s.Capture(t.Context(), api.BrowserScreenshotOptions{ID: "s", Annotate: true}); err != nil {
		t.Fatal(err)
	}
	if p.annotated != 1 {
		t.Fatalf("annotate took %d annotated captures, want 1", p.annotated)
	}
	if after := click(); after.InteractiveSame || len(after.Interactive) != 1 {
		t.Fatalf("the list only the image showed was left out of the next answer: %+v", after)
	}
}
