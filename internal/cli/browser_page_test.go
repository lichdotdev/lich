package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

func TestPageTextIsOneLinePerElement(t *testing.T) {
	yes := true
	info := api.BrowserPageInfo{
		URL: "https://app.notacil.com.br/invoices", Title: "Notacil", Text: "Notas fiscais",
		Interactive: []api.BrowserInteractive{
			{Index: 0, Tag: "button", Type: "button", Text: "Toggle Sidebar", Rect: api.BrowserRect{X: 207, Y: 21, W: 28, H: 28}},
			{Index: 1, Tag: "a", Role: "link", Text: "Dashboard", Href: "https://app.notacil.com.br/dashboard"},
			{Index: 2, Tag: "a", Text: "Docs", Href: "https://docs.example.com/x"},
			{Index: 3, Tag: "input", Type: "email", Text: "E-mail", Filled: &yes},
			{Index: 4, Tag: "div", Role: "button", Text: "Emitir nota \"já\""},
			{Index: 5, Tag: "input", Type: "checkbox", Text: "Lembrar", Checked: &yes},
		},
		Truncated: true,
	}
	want := strings.Join([]string{
		"url: https://app.notacil.com.br/invoices",
		"title: Notacil",
		"text: Notas fiscais",
		`interactive (index tag "label" -> link):`,
		`0 button "Toggle Sidebar"`,
		`1 a "Dashboard" -> /dashboard`,
		`2 a "Docs" -> https://docs.example.com/x`,
		`3 input:email "E-mail" (filled)`,
		`4 div[button] "Emitir nota \"já\""`,
		`5 input:checkbox "Lembrar" (checked)`,
		"(more elements below, scroll to reach them)",
	}, "\n")
	if got := pageText(info, prompt.English); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	raw, _ := json.Marshal(info)
	if len(pageText(info, prompt.English)) >= len(raw)*2/3 {
		t.Fatalf("text %d chars against %d for the JSON: not smaller enough", len(pageText(info, prompt.English)), len(raw))
	}
}

func TestDownloadLineSaysAFailedMove(t *testing.T) {
	d := api.BrowserDownload{Path: "/home/u/Downloads/report.csv", Bytes: 5, State: api.DownloadFailed}
	if got := downloadLine(d, prompt.For(prompt.English)); got != "download failed: /home/u/Downloads/report.csv" {
		t.Fatalf("got %q", got)
	}
}

func TestPageTextOfAnEmptyPage(t *testing.T) {
	if got := pageText(api.BrowserPageInfo{URL: "about:blank"}, prompt.English); got != "url: about:blank\ntitle: " {
		t.Fatalf("got %q", got)
	}
}

func TestPageTextNamesTheDownloadsAfterTheTitle(t *testing.T) {
	info := api.BrowserPageInfo{URL: "https://x.test/", Title: "X", Text: "Reports", Downloads: []api.BrowserDownload{
		{Path: "/home/u/Downloads/report.csv", Bytes: 12 * 1024, State: api.DownloadCompleted},
		{Path: "/home/u/Downloads/big.iso", Bytes: 3 << 20, State: api.DownloadInProgress},
		{Path: "/home/u/Downloads/gone.zip", State: api.DownloadCanceled},
		{Path: "/home/u/Downloads/tiny.txt", Bytes: 5, State: api.DownloadCompleted},
	}}
	want := strings.Join([]string{
		"url: https://x.test/",
		"title: X",
		"downloaded: /home/u/Downloads/report.csv (12 KB)",
		"downloading: /home/u/Downloads/big.iso (3.0 MB so far)",
		"download canceled: /home/u/Downloads/gone.zip",
		"downloaded: /home/u/Downloads/tiny.txt (5 B)",
		"text: Reports",
	}, "\n")
	if got := pageText(info, prompt.English); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPageTextSaysAWaitTimedOutBeforeThePage(t *testing.T) {
	info := api.BrowserPageInfo{URL: "https://x.test/", Title: "X", Text: "Loading", WaitTimedOut: true}
	want := "url: https://x.test/\ntitle: X\nwait: timed out, this is the page as it stands\ntext: Loading"
	if got := pageText(info, prompt.English); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPageTextNamesConsoleErrorsAndHowManyMore(t *testing.T) {
	info := api.BrowserPageInfo{URL: "about:blank", ConsoleErrors: []string{"Uncaught Error: x", "boom"}, ConsoleErrorsMore: 4}
	want := "url: about:blank\ntitle: \nconsole error: Uncaught Error: x\nconsole error: boom\n(4 more console errors)"
	if got := pageText(info, prompt.English); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPageTextNamesTheAnsweredDialogsThenTheOpenOne(t *testing.T) {
	info := api.BrowserPageInfo{URL: "https://x.test/", Title: "X",
		Dialogs: []api.BrowserDialog{{Type: "confirm", Message: "Delete?", Answer: api.DialogDismissed}},
		Dialog:  &api.BrowserDialog{Type: "prompt", Message: "Name?", Default: "Ada"},
	}
	want := strings.Join([]string{
		"url: https://x.test/",
		"title: X",
		`dialog answered: confirm "Delete?" -> dismissed`,
		`dialog open: prompt "Name?" default "Ada"`,
		"The page runs nothing until this dialog is answered: call browser_dialog.",
	}, "\n")
	if got := pageText(info, prompt.English); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

// The prose of a page answer follows the prompt language; the line tags the
// tool descriptions name stay English in every one.
func TestPageTextInEveryLanguage(t *testing.T) {
	yes := true
	info := api.BrowserPageInfo{
		URL: "https://x.test/", Title: "X", Text: "Hi", Truncated: true, WaitTimedOut: true,
		Downloads:         []api.BrowserDownload{{Path: "/dl/big.iso", Bytes: 3, State: api.DownloadInProgress}},
		ConsoleErrors:     []string{"boom"},
		ConsoleErrorsMore: 2,
		Dialog:            &api.BrowserDialog{Type: "alert", Message: "hi"},
		Interactive:       []api.BrowserInteractive{{Index: 1, Tag: "input", Filled: &yes, Checked: &yes}},
	}
	english := pageText(info, prompt.English)
	for _, lang := range prompt.Langs {
		got := pageText(info, lang)
		if strings.Contains(got, "%!") {
			t.Errorf("%s: a format argument is off:\n%s", lang, got)
		}
		for _, literal := range []string{
			"url: ", "title: ", "wait: ", "text: ", "downloading: /dl/big.iso", "3 B", "console error: boom", "2",
			`dialog open: alert "hi"`, "browser_dialog", "interactive (",
		} {
			if !strings.Contains(got, literal) {
				t.Errorf("%s: lost %q:\n%s", lang, literal, got)
			}
		}
		if lang != prompt.English && got == english {
			t.Errorf("%s: the page reads the same as in English", lang)
		}
	}
}

func TestTheCommandLineAsksForItsOwnVerb(t *testing.T) {
	info := api.BrowserPageInfo{URL: "https://x.test/", Dialog: &api.BrowserDialog{Type: "alert", Message: "hi"}}
	for _, lang := range prompt.Langs {
		got := cliPageText(info, lang)
		if !strings.Contains(got, "lich browser dialog accept|dismiss") || strings.Contains(got, "browser_dialog") {
			t.Errorf("%s: the command line answer names the wrong call:\n%s", lang, got)
		}
	}
}
