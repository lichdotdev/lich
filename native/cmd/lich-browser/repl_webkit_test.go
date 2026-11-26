//go:build linux || darwin

package main

import "testing"

func TestAScriptRunsAsTheConsoleRunsIt(t *testing.T) {
	for _, tc := range []struct{ name, script, program, body string }{
		{"an expression", "1 + 1", "\n;" + settled("1 + 1"), awaitValue},
		{"declarations become var", "let a = 1; const {b} = o; a", "var a = 1; var {b} = o;\n;" + settled("a"), awaitValue},
		{"a ; in a string is no boundary", `x = "a;b"; x`, `x = "a;b";` + "\n;" + settled("x"), awaitValue},
		{"a let inside a block stays", "{ let a = 1 }", "{ let a = 1 }\n;[0]", awaitValue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run, ok := consoleForm(tc.script, jsParses)
			if !ok || run.async {
				t.Fatalf("%q runs as %+v, %v; want a program", tc.script, run, ok)
			}
			if tc.body != "" && (run.program != tc.program || run.body != tc.body) {
				t.Fatalf("%q runs as\n%q then %q\nwant\n%q then %q", tc.script, run.program, run.body, tc.program, tc.body)
			}
		})
	}
}

func TestTopLevelAwaitRunsAsAnAsyncBody(t *testing.T) {
	run, ok := consoleForm("const r = await f()\nr.x", jsParses)
	want := "var r = await f()\n\n;return (r.x\n);"
	if !ok || !run.async || run.body != want {
		t.Fatalf("runs as %+v, want the async body %q", run, want)
	}
}

func TestAScriptThatParsesNoWayIsLeftAsWritten(t *testing.T) {
	if run, ok := consoleForm("(1", jsParses); ok {
		t.Fatalf("(1 runs as %+v, want it left to fail as written", run)
	}
}

func TestRedeclarableLeavesWhatIsNotADeclaration(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"let x = 1", "var x = 1"},
		{"const [a, b] = c", "var [a, b] = c"},
		{"class A extends B {}", "var A = class A extends B {}"},
		{"for (let i = 0; i < 1; i++) {}", "for (let i = 0; i < 1; i++) {}"},
		{"f(() => { const y = 2 })", "f(() => { const y = 2 })"},
		{"s = `let ${ {a: 1}.a } x`; let z", "s = `let ${ {a: 1}.a } x`; var z"},
		{"r = /let ;/g; const q = 1", "r = /let ;/g; var q = 1"},
		{"a = b / c; let d", "a = b / c; var d"},
		{"// let x\nlet y", "// let x\nvar y"},
	} {
		if got := redeclarable(tc.in); got != tc.want {
			t.Errorf("redeclarable(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
