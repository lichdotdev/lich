package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// pageText is a page as an agent reads it: one line per interactive element
// rather than JSON, whose keys and boxes cost about twice the tokens for the
// same page (a session reading the Notacil app spent 3 to 4 thousand per
// read, 93% of it the element list; measured 2026-10-26). Positions are left
// out: an agent clicks by index, and a box is a screenshot's job.
func pageText(info api.BrowserPageInfo, lang prompt.Lang) string {
	text := prompt.For(lang)
	return pageTextAsking(info, text, text.PageDialogWaits)
}

// cliPageText is pageText for `lich browser`, whose answer to an open dialog
// is its own verb rather than the MCP tool.
func cliPageText(info api.BrowserPageInfo, lang prompt.Lang) string {
	text := prompt.For(lang)
	return pageTextAsking(info, text, text.PageDialogWaitsCLI)
}

// pageTextAsking is the page answer, with waits as what an open dialog asks
// of the agent.
func pageTextAsking(info api.BrowserPageInfo, text *prompt.Catalog, waits string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "url: %s\ntitle: %s\n", info.URL, info.Title)
	if info.WaitTimedOut {
		b.WriteString(text.PageWaitTimedOut + "\n")
	}
	writeDialogs(&b, info, waits)
	for _, d := range info.Downloads {
		b.WriteString(downloadLine(d, text))
		b.WriteByte('\n')
	}
	for _, e := range info.ConsoleErrors {
		fmt.Fprintf(&b, "console error: %s\n", e)
	}
	if n := info.ConsoleErrorsMore; n > 0 {
		fmt.Fprintf(&b, text.Pick(n, text.PageConsoleMore)+"\n", n)
	}
	if info.Text != "" {
		fmt.Fprintf(&b, "text: %s\n", info.Text)
	}
	if len(info.Interactive) > 0 {
		b.WriteString(text.PageInteractive + "\n")
	}
	base, _ := url.Parse(info.URL)
	for _, n := range info.Interactive {
		b.WriteString(elementLine(n, base, text))
		b.WriteByte('\n')
	}
	if info.Truncated {
		b.WriteString(text.PageMore + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// writeDialogs names the dialogs the user answered since the last page
// answer, then the one the page holds open, which the agent must answer
// before anything else.
func writeDialogs(b *strings.Builder, info api.BrowserPageInfo, waits string) {
	for _, d := range info.Dialogs {
		fmt.Fprintf(b, "dialog answered: %s %q -> %s\n", d.Type, d.Message, d.Answer)
	}
	d := info.Dialog
	if d == nil {
		return
	}
	fmt.Fprintf(b, "dialog open: %s %q", d.Type, d.Message)
	if d.Default != "" {
		fmt.Fprintf(b, " default %q", d.Default)
	}
	b.WriteString("\n" + waits + "\n")
}

func elementLine(n api.BrowserInteractive, base *url.URL, text *prompt.Catalog) string {
	parts := []string{strconv.Itoa(n.Index), elementKind(n)}
	if n.Text != "" {
		parts = append(parts, strconv.Quote(n.Text))
	}
	if n.Href != "" {
		parts = append(parts, "-> "+shortHref(n.Href, base))
	}
	if n.Filled != nil && *n.Filled {
		parts = append(parts, text.PageFilled)
	}
	if n.Checked != nil && *n.Checked {
		parts = append(parts, text.PageChecked)
	}
	return strings.Join(parts, " ")
}

// elementKind is the tag, with an input's type and a role the tag does not
// already say: input:email, div[button].
func elementKind(n api.BrowserInteractive) string {
	kind := n.Tag
	if n.Type != "" && n.Tag == "input" {
		kind += ":" + n.Type
	}
	if n.Role != "" && n.Role != n.Tag && !(n.Role == "link" && n.Tag == "a") {
		kind += "[" + n.Role + "]"
	}
	return kind
}

// shortHref drops the page's own origin from a link, which every link on the
// page would otherwise repeat.
func shortHref(href string, base *url.URL) string {
	if base == nil || base.Host == "" {
		return href
	}
	origin := base.Scheme + "://" + base.Host
	if rest, ok := strings.CutPrefix(href, origin); ok && strings.HasPrefix(rest, "/") {
		return rest
	}
	return href
}

// downloadLine says where a file the page downloaded is, and how far it got.
func downloadLine(d api.BrowserDownload, text *prompt.Catalog) string {
	switch d.State {
	case api.DownloadCompleted:
		return fmt.Sprintf("downloaded: %s (%s)", d.Path, sizeText(d.Bytes))
	case api.DownloadInProgress:
		return fmt.Sprintf(text.PageDownloading, d.Path, sizeText(d.Bytes))
	case api.DownloadFailed:
		return "download failed: " + d.Path
	default:
		return "download canceled: " + d.Path
	}
}

func sizeText(n int64) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
}
