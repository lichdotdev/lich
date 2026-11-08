package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// boundaryNonceBytes is 8 hex characters: the page must guess them to forge a
// marker, and a longer one is tokens every answer pays.
const boundaryNonceBytes = 4

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
// of the agent. What the page writes (title, dialog messages, console errors,
// text, element labels) sits between boundary markers; lich's own lines stay
// outside.
func pageTextAsking(info api.BrowserPageInfo, text *prompt.Catalog, waits string) string {
	var head, body, tail strings.Builder
	fmt.Fprintf(&head, "url: %s\n", info.URL)
	if info.WaitTimedOut {
		head.WriteString(text.PageWaitTimedOut + "\n")
	}
	for _, d := range info.Downloads {
		head.WriteString(downloadLine(d, text))
		head.WriteByte('\n')
	}
	fmt.Fprintf(&body, "title: %s\n", info.Title)
	writeDialogs(&body, info)
	for _, e := range info.ConsoleErrors {
		fmt.Fprintf(&body, "console error: %s\n", e)
	}
	if n := info.ConsoleErrorsMore; n > 0 {
		fmt.Fprintf(&body, text.Pick(n, text.PageConsoleMore)+"\n", n)
	}
	if info.Text != "" {
		fmt.Fprintf(&body, "text: %s\n", info.Text)
	}
	if info.InteractiveSame {
		body.WriteString(text.PageListSame + "\n")
	}
	if len(info.Interactive) > 0 {
		body.WriteString(text.PageInteractive + "\n")
	}
	base, _ := url.Parse(info.URL)
	for _, n := range info.Interactive {
		body.WriteString(elementLine(n, base, text))
		body.WriteByte('\n')
	}
	if info.Dialog != nil {
		tail.WriteString(waits + "\n")
	}
	if info.Truncated {
		tail.WriteString(text.PageMore + "\n")
	}
	return strings.TrimSuffix(head.String()+pageBlock(strings.TrimSuffix(body.String(), "\n"))+"\n"+tail.String(), "\n")
}

// pageBlock marks content a page controls. The nonce is fresh per answer: the
// MCP server lives as long as the session, so one per process would let a page
// that saw an answer forge the closing marker of the next. A page cannot close
// the block with a marker it cannot predict.
func pageBlock(content string) string {
	var n [boundaryNonceBytes]byte
	_, _ = rand.Read(n[:]) // never fails since Go 1.24
	nonce := hex.EncodeToString(n[:])
	return fmt.Sprintf("<page %s>\n%s\n</page %s>", nonce, content, nonce)
}

// writeDialogs names the dialogs the user answered since the last page
// answer, then the one the page holds open, which the agent must answer
// before anything else.
func writeDialogs(b *strings.Builder, info api.BrowserPageInfo) {
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
	b.WriteByte('\n')
}

func elementLine(n api.BrowserInteractive, base *url.URL, text *prompt.Catalog) string {
	parts := []string{strconv.Itoa(n.Index), elementKind(n)}
	if n.Text != "" {
		parts = append(parts, strconv.Quote(n.Text))
	}
	// A labeled link is clicked by its index: its href was 16% of a page
	// answer, and 17 of 198 navigations went to one, all labeled (measured
	// 2026-11-07).
	if n.Href != "" && n.Text == "" {
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
