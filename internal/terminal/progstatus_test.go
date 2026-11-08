package terminal

import (
	"encoding/base64"
	"strings"
	"testing"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func parseBody(body string) (progReport, bool) {
	return parseProgReport(body, progSeqLen(len(progPrefix)+len(body), false))
}

func TestParseProgReportReadsEveryKey(t *testing.T) {
	t.Parallel()
	r, ok := parseBody("state=blocked:kind=permission:app=terraform:id=deploy/us-east:title=" + b64("Apply") + ":msg=" + b64("Apply 3 to add?") + ":progress=40")
	want := progReport{state: progBlocked, id: "deploy/us-east", app: "terraform", title: "Apply", msg: "Apply 3 to add?"}
	if !ok || r != want {
		t.Fatalf("parse = %+v, %v; want %+v", r, ok, want)
	}
}

func TestParseProgReportGrammar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body string
		ok         bool
		want       progReport
	}{
		{"blanks trimmed", " state = working \t", true, progReport{state: progWorking}},
		{"pair without = skipped", "junk:state=done", true, progReport{state: progDone}},
		{"bad value skipped", "state=done:app=a b", true, progReport{state: progDone}},
		{"unknown key ignored", "state=idle:future=1", true, progReport{state: progIdle}},
		{"last repeat wins", "state=working:state=done", true, progReport{state: progDone}},
		{"earlier bad value replaced", "app=a;b:app=cargo:state=done", true, progReport{state: progDone, app: "cargo"}},
		{"no state", "app=cargo", false, progReport{}},
		{"unknown state", "state=sleeping", false, progReport{}},
		{"key over the limit", "state=done:abcdefghijklmnopq=1", false, progReport{}},
		{"id outside the value set", "state=done:id=a b", false, progReport{}},
		{"id outside the grammar", "state=done:id=a//b", false, progReport{}},
		{"empty id", "state=done:id=", false, progReport{}},
		{"id too deep", "state=done:id=a/b/c/d/e/f/g/h/i", false, progReport{}},
		{"app over 32 bytes", "state=done:app=" + strings.Repeat("a", 33), false, progReport{}},
		{"app outside its set", "state=done:app=a,b", true, progReport{state: progDone}},
		{"clear keeps only its id", "state=clear:id=build:app=x", true, progReport{state: progClear, id: "build"}},
		{"msg unpadded", "state=done:msg=" + strings.TrimRight(b64("ab"), "="), true, progReport{state: progDone, msg: "ab"}},
		{"msg wrong padding", "state=done:msg=YQ=", false, progReport{}},
		{"msg non-zero tail bits", "state=done:msg=YWJ", false, progReport{}},
		{"msg not UTF-8", "state=done:msg=" + b64("\xff"), false, progReport{}},
		{"msg with a control", "state=done:msg=" + b64("a\nb"), false, progReport{}},
		{"msg with a C1 control", "state=done:msg=" + b64("a\u0085b"), false, progReport{}},
		{"title over its limit", "state=done:title=" + b64(strings.Repeat("a", 193)), false, progReport{}},
		{"invisible stripped", "state=done:msg=" + b64("ok\u202egnp.exe\u200b"), true, progReport{state: progDone, msg: "okgnp.exe"}},
	}
	for _, c := range cases {
		r, ok := parseBody(c.body)
		if ok != c.ok || r != c.want {
			t.Errorf("%s: parse(%q) = %+v, %v; want %+v, %v", c.name, c.body, r, ok, c.want, c.ok)
		}
	}
}

func TestParseProgReportRefusesAnOverlongSequence(t *testing.T) {
	t.Parallel()
	if _, ok := parseProgReport("state=done", progMaxSequence+1); ok {
		t.Fatal("a sequence over 4096 bytes was applied")
	}
}

func TestProgQuery(t *testing.T) {
	t.Parallel()
	if !progQuery(" ? ") || progQuery("?x") {
		t.Fatal("query detection")
	}
}
