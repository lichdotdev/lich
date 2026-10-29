package browser

import (
	"testing"
)

func threeTabs() (*tabStack, []*tab) {
	tabs := []*tab{{id: "first"}, {id: "second"}, {id: "third"}}
	return &tabStack{open: append([]*tab{}, tabs...)}, tabs
}

func TestTabStackRemoveSaysWhetherTheTabWasInFront(t *testing.T) {
	s, tabs := threeTabs()
	if got, front := s.remove("second"); got != tabs[1] || front {
		t.Fatalf("remove(second) = %v, %v; want the second tab, not in front", got, front)
	}
	if got, front := s.remove("third"); got != tabs[2] || !front {
		t.Fatalf("remove(third) = %v, %v; want the third tab, in front", got, front)
	}
	if s.front() != tabs[0] {
		t.Fatal("the first tab is not in front once the others closed")
	}
}

func TestTabStackNeverRemovesTheFirstTab(t *testing.T) {
	s, tabs := threeTabs()
	if got, _ := s.remove("first"); got != nil {
		t.Fatalf("remove(first) = %v, want nothing", got)
	}
	if got, _ := s.remove("unknown"); got != nil {
		t.Fatalf("remove(unknown) = %v, want nothing", got)
	}
	if len(s.open) != len(tabs) {
		t.Fatalf("%d tabs open, want %d", len(s.open), len(tabs))
	}
}

func TestTabStackAnswersTheDialogWhereItWasOpened(t *testing.T) {
	s, tabs := threeTabs()
	if s.dialogHolder() != tabs[2] {
		t.Fatal("with no dialog open, the holder is not the front tab")
	}
	s.setDialog(tabs[0])
	if s.dialogHolder() != tabs[0] {
		t.Fatal("the dialog's tab behind the front one is not its holder")
	}
	if s.dialogClosed(tabs[1]) {
		t.Fatal("a tab holding no dialog closed it")
	}
	if !s.dialogClosed(tabs[0]) {
		t.Fatal("the tab holding the dialog did not close it")
	}
	if s.dialogHolder() != tabs[2] {
		t.Fatal("after the dialog closed, the holder is not the front tab")
	}
}
