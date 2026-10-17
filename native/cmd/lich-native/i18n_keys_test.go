package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/i18n"
)

// keyFields are the settingEntry fields that hold a catalog key.
var keyFields = map[string]bool{"titleKey": true, "alsoKey": true, "groupKey": true}

// catalogKeys are the keys the package's source names literally: the key
// argument of i18n.T, i18n.Prompt and i18n.In, and a settingEntry's key
// fields. A key that only exists at runtime is not seen.
func catalogKeys(t *testing.T) map[string]token.Position {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	keys := map[string]token.Position{}
	note := func(e ast.Expr) {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil {
				keys[s] = fset.Position(lit.Pos())
			}
		}
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if arg, ok := keyArgument(n); ok {
					note(arg)
				}
			case *ast.KeyValueExpr:
				if id, ok := n.Key.(*ast.Ident); ok && keyFields[id.Name] {
					note(n.Value)
				}
			}
			return true
		})
	}
	return keys
}

// keyArgument is the key a call to i18n.T, i18n.Prompt or i18n.In passes.
func keyArgument(call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "i18n" {
		return nil, false
	}
	at := map[string]int{"T": 0, "Prompt": 0, "In": 1}
	i, ok := at[sel.Sel.Name]
	if !ok || len(call.Args) <= i {
		return nil, false
	}
	return call.Args[i], true
}

func TestEveryKeyTheSourceNamesIsInTheCatalog(t *testing.T) {
	keys := catalogKeys(t)
	if len(keys) == 0 {
		t.Fatal("found no keys: the scan is broken")
	}
	for key, pos := range keys {
		if !i18n.Has(key) {
			t.Errorf("%s: no message %q", pos, key)
		}
	}
}
