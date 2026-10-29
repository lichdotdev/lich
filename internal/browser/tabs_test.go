package browser

import (
	"slices"
	"testing"

	"github.com/chromedp/cdproto/target"
)

// stackOf starts a stack on a tab "root" and adds tabs named by id, each
// opened by the tab named in openers (absent for none).
func stackOf(ids []target.ID, openers map[target.ID]target.ID) *tabStack {
	s := &tabStack{}
	s.start(&tab{id: "root"})
	for _, id := range ids {
		s.add(&tab{id: id, opener: s.byID(openers[id])})
	}
	return s
}

func order(s *tabStack) []target.ID {
	var out []target.ID
	for _, t := range s.list() {
		out = append(out, target.ID(t.ID))
	}
	return out
}

func TestTabStackPutsANewTabAfterTheOnesItsOpenerOpened(t *testing.T) {
	s := stackOf([]target.ID{"blank", "a", "b"}, map[target.ID]target.ID{"a": "root", "b": "root"})
	if got, want := order(s), []target.ID{"root", "a", "b", "blank"}; !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestTabStackClosingTheFrontTabBringsItsOpenerBack(t *testing.T) {
	s := stackOf([]target.ID{"a", "b"}, map[target.ID]target.ID{"a": "root"})
	s.setFront(s.byID("a"))
	if got, front := s.remove("a"); got == nil || !front {
		t.Fatalf("remove(a) = %v, %v; want the tab, in front", got, front)
	}
	if s.frontTab().id != "root" {
		t.Fatalf("front = %s, want the tab that opened the closed one", s.frontTab().id)
	}
}

func TestTabStackClosingAFrontTabNobodyOpenedBringsItsNeighbour(t *testing.T) {
	s := stackOf([]target.ID{"a", "b"}, nil)
	s.setFront(s.byID("a"))
	s.remove("a")
	if s.frontTab().id != "b" {
		t.Fatalf("front = %s, want the tab that took its place", s.frontTab().id)
	}
	s.remove("b")
	if s.frontTab().id != "root" {
		t.Fatalf("front = %s, want the tab before the last one", s.frontTab().id)
	}
}

func TestTabStackClosingATabBehindKeepsTheFront(t *testing.T) {
	s := stackOf([]target.ID{"a", "b"}, map[target.ID]target.ID{"a": "root", "b": "a"})
	s.setFront(s.byID("b"))
	if _, front := s.remove("a"); front {
		t.Fatal("removing a tab behind said it was in front")
	}
	if s.frontTab().id != "b" {
		t.Fatalf("front = %s, want it unchanged", s.frontTab().id)
	}
	if s.byID("b").opener == nil || s.byID("b").opener.id != "root" {
		t.Fatal("a tab whose opener closed does not fall back to its opener's opener")
	}
}

func TestTabStackNeverRemovesTheLastTab(t *testing.T) {
	s := stackOf(nil, nil)
	if got, _ := s.remove("root"); got != nil {
		t.Fatalf("remove(root) = %v, want nothing", got)
	}
	if got, _ := stackOf([]target.ID{"a"}, nil).remove("unknown"); got != nil {
		t.Fatalf("remove(unknown) = %v, want nothing", got)
	}
}

func TestTabStackListsInfoAndMarksTheFront(t *testing.T) {
	s := stackOf([]target.ID{"a"}, nil)
	a := s.byID("a")
	s.setFront(a)
	if !s.setInfo(a, "https://example.com/", "Example") {
		t.Fatal("setInfo with a new URL said nothing changed")
	}
	if s.setInfo(a, "https://example.com/", "Example") {
		t.Fatal("setInfo with the same URL and title said it changed")
	}
	got := s.list()[1]
	if got.URL != "https://example.com/" || got.Title != "Example" || !got.Front || s.list()[0].Front {
		t.Fatalf("list = %+v, want a's info with a alone in front", s.list())
	}
}

func TestTabStackAnswersTheDialogWhereItWasOpened(t *testing.T) {
	s := stackOf([]target.ID{"a"}, nil)
	root, a := s.byID("root"), s.byID("a")
	s.setFront(a)
	if s.dialogHolder() != a {
		t.Fatal("with no dialog open, the holder is not the front tab")
	}
	s.setDialog(root)
	if s.dialogHolder() != root {
		t.Fatal("the dialog's tab behind the front one is not its holder")
	}
	if s.dialogClosed(a) {
		t.Fatal("a tab holding no dialog closed it")
	}
	if !s.dialogClosed(root) || s.dialogHolder() != a {
		t.Fatal("once the dialog closed, the holder is not the front tab")
	}
}
