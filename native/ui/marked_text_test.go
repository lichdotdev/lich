package ui

import "testing"

func TestMarkedTextLaysEverySegment(t *testing.T) {
	th := testTheme()
	plain := layoutAt(th.MarkedText([]MarkedSegment{{Text: "ab"}}).Layout, 0, 500)
	marked := layoutAt(th.MarkedText([]MarkedSegment{{Text: "ab"}, {Text: "cd", Marked: true}}).Layout, 0, 500)
	if marked.Size.X <= plain.Size.X || marked.Size.Y == 0 {
		t.Errorf("plain %v, with a mark %v", plain.Size, marked.Size)
	}
}
