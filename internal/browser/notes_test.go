package browser

import (
	"context"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestElementAtAsksThePageAtThePoint(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	page := launch.pages[0]
	page.element = api.BrowserElement{Label: `button "Buy"`, Selector: "button.buy"}
	el, err := s.ElementAtPoint(context.Background(), api.BrowserPointOptions{ID: "s", X: 12, Y: 34})
	if err != nil {
		t.Fatal(err)
	}
	if el.Selector != "button.buy" || page.pointedAt != [2]float64{12, 34} {
		t.Fatalf("element %+v asked at %v, want button.buy at 12,34", el, page.pointedAt)
	}
}

func TestLocateAnswersABoxPerSelectorAndEmptyForOneGone(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].boxes = map[string]api.BrowserRect{"h1": {X: 1, Y: 2, W: 30, H: 40}}
	boxes, err := s.LocateElements(context.Background(), api.BrowserLocateOptions{ID: "s", Selectors: []string{"h1", "#gone"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []api.BrowserRect{{X: 1, Y: 2, W: 30, H: 40}, {}}
	if len(boxes) != 2 || boxes[0] != want[0] || boxes[1] != want[1] {
		t.Fatalf("boxes = %+v, want %+v", boxes, want)
	}
}

func TestElementAtWithoutABrowserFails(t *testing.T) {
	s, _, _ := viewService(t)
	if _, err := s.ElementAtPoint(context.Background(), api.BrowserPointOptions{ID: "s"}); err == nil {
		t.Fatal("ElementAt on a session with no browser answered")
	}
}
