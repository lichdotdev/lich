package main

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestAToastOpensItsSessionAndCarriesBothButtons(t *testing.T) {
	n := desktopNote{sessionID: "abc", status: statusDone, title: "Fix auth finished", body: "All green.", meta: "lich · fix/auth"}
	doc := toastXML(n, "Open", "Dismiss")
	for _, want := range []string{
		`launch="lich-session:abc"`,
		`<action activationType="protocol" arguments="lich-session:abc" content="Open"/>`,
		`<action activationType="system" arguments="dismiss" content="Dismiss"/>`,
		`<text>Fix auth finished</text><text>All green.</text><text>lich · fix/auth</text>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("toast lacks %s:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "reminder") {
		t.Errorf("a finished turn's toast is a reminder:\n%s", doc)
	}
}

func TestAWaitingToastStaysAndEscapesWhatTheSessionAsks(t *testing.T) {
	n := desktopNote{sessionID: "abc", status: statusWaiting, title: "x", body: `Allow Bash: echo "<a>" & rm`}
	doc := toastXML(n, "Open", "Dismiss")
	if !strings.Contains(doc, `scenario="reminder"`) {
		t.Errorf("a waiting toast is not a reminder:\n%s", doc)
	}
	if err := xml.Unmarshal([]byte(doc), new(struct{})); err != nil {
		t.Fatalf("toast is not well-formed XML: %v\n%s", err, doc)
	}
	if strings.Contains(doc, "<text></text>") {
		t.Errorf("an empty line became a text element:\n%s", doc)
	}
}
