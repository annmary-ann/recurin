package money_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoFloats fails if float32 or float64 appears anywhere in this package,
// including its tests. Money is int64 minor units; see ADR 0003. Other money
// paths (tax, invoices, ledger) add their directories here as they arrive.
func TestNoFloats(t *testing.T) {
	dirs := []string{"."}
	fset := token.NewFileSet()
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && (id.Name == "float32" || id.Name == "float64") {
					t.Errorf("%s: %s is not allowed in a money path", fset.Position(id.Pos()), id.Name)
				}
				return true
			})
		}
	}
}
