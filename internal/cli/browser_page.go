package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// pageText is a page as an agent reads it: one line per interactive element
// rather than JSON, whose keys and boxes cost about twice the tokens for the
// same page (a session reading the Notacil app spent 3 to 4 thousand per
// read, 93% of it the element list; measured 2026-10-26). Positions are left
// out: an agent clicks by index, and a box is a screenshot's job.
func pageText(info api.BrowserPageInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "url: %s\ntitle: %s\n", info.URL, info.Title)
	if info.WaitTimedOut {
		b.WriteString("wait: timed out, this is the page as it stands\n")
	}
	for _, d := range info.Downloads {
		b.WriteString(downloadLine(d))
		b.WriteByte('\n')
	}
	for _, e := range info.ConsoleErrors {
		fmt.Fprintf(&b, "console error: %s\n", e)
	}
	if info.ConsoleErrorsMore > 0 {
		fmt.Fprintf(&b, "(%d more console errors)\n", info.ConsoleErrorsMore)
	}
	if info.Text != "" {
		fmt.Fprintf(&b, "text: %s\n", info.Text)
	}
	if len(info.Interactive) > 0 {
		b.WriteString("interactive (index tag \"label\" -> link):\n")
	}
	base, _ := url.Parse(info.URL)
	for _, n := range info.Interactive {
		b.WriteString(elementLine(n, base))
		b.WriteByte('\n')
	}
	if info.Truncated {
		b.WriteString("(more elements below, scroll to reach them)\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func elementLine(n api.BrowserInteractive, base *url.URL) string {
	parts := []string{strconv.Itoa(n.Index), elementKind(n)}
	if n.Text != "" {
		parts = append(parts, strconv.Quote(n.Text))
	}
	if n.Href != "" {
		parts = append(parts, "-> "+shortHref(n.Href, base))
	}
	if n.Filled != nil && *n.Filled {
		parts = append(parts, "(filled)")
	}
	if n.Checked != nil && *n.Checked {
		parts = append(parts, "(checked)")
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
func downloadLine(d api.BrowserDownload) string {
	switch d.State {
	case api.DownloadCompleted:
		return fmt.Sprintf("downloaded: %s (%s)", d.Path, sizeText(d.Bytes))
	case api.DownloadInProgress:
		return fmt.Sprintf("downloading: %s (%s so far)", d.Path, sizeText(d.Bytes))
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
