package markdown

import (
	"fmt"
	"strings"
	"testing"
)

func init() {
	names := map[Alert]string{AlertNote: "Note", AlertTip: "Tip", AlertImportant: "Important", AlertWarning: "Warning", AlertCaution: "Caution"}
	Words = Labels{
		Alert: func(a Alert) string { return names[a] },
		More:  func(count int) string { return fmt.Sprintf("Show %d more lines", count) },
		Less:  func() string { return "Show less" },
	}
}

func blocksText(blocks []markdownBlock) string {
	var s strings.Builder
	for _, b := range blocks {
		s.WriteString(proseText(b))
	}
	return s.String()
}

func TestAGitHubAlertDropsItsMarkerAndKeepsItsText(t *testing.T) {
	for src, want := range map[string]Alert{
		"> [!NOTE]\n> Read this.":       AlertNote,
		"> [!tip]\n> Read this.":        AlertTip,
		"> [!IMPORTANT]\n> Read this.":  AlertImportant,
		"> [!WARNING]\n>\n> Read this.": AlertWarning,
		"> [!CAUTION]  \n> Read this.":  AlertCaution,
	} {
		blocks := renderBlocks(markdownTheme(), src)
		if len(blocks) != 1 || blocks[0].alert != want || !blocks[0].quote {
			t.Errorf("%q: blocks = %+v, want one %v alert", src, blocks, want)
			continue
		}
		if got := blocksText(blocks[0].blocks); got != "Read this." {
			t.Errorf("%q reads %q, want its text without the marker", src, got)
		}
	}
}

func TestAQuoteThatIsNoAlertStaysAQuote(t *testing.T) {
	for _, src := range []string{
		"> [!NOTE]",
		"> [!DANGER]\n> Read this.",
		"> Before [!NOTE]\n> Read this.",
	} {
		blocks := renderBlocks(markdownTheme(), src)
		if len(blocks) != 1 || blocks[0].alert != noAlert || !blocks[0].quote {
			t.Errorf("%q: blocks = %+v, want a plain quote", src, blocks)
		}
	}
}

// An alert's text reads as prose, not muted as a quote's, under its name.
func TestAnAlertIsProseUnderItsName(t *testing.T) {
	th := markdownTheme()
	blocks := renderBlocks(th, "> [!WARNING]\n> Back up first.")
	for _, span := range blocks[0].blocks[0].prose.spans {
		if span.Color != markdownProseColor(th) {
			t.Errorf("alert span %q color = %v, want prose %v", span.Content, span.Color, markdownProseColor(th))
		}
	}
	quote := renderBlocks(th, "> Back up first.")
	gtx, _ := markdownFrame(400)
	if alert, plain := blocks[0].layout(gtx, th, Links{}).Size.Y, quote[0].layout(gtx, th, Links{}).Size.Y; alert <= plain {
		t.Errorf("an alert is %dpx tall and the same quote %dpx, want its name above the text", alert, plain)
	}
}

func TestEachAlertHasItsOwnTone(t *testing.T) {
	th := markdownTheme()
	th.ToneNote, th.ToneImportant = th.Ring, th.Primary
	seen := map[any]Alert{}
	for _, a := range []Alert{AlertNote, AlertTip, AlertImportant, AlertWarning, AlertCaution} {
		tone := a.tone(th)
		if other, dup := seen[tone]; dup || tone == th.Border {
			t.Errorf("alert %v has tone %v, shared with %v or the quote's bar", a, tone, other)
		}
		seen[tone] = a
	}
}
