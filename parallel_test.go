package tt

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// callsParallel tells whether fn starts by calling t.Parallel().
func callsParallel(fn *ast.FuncDecl) bool {
	if len(fn.Body.List) == 0 {
		return false
	}

	stmt, ok := fn.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)

	return ok && sel.Sel.Name == "Parallel"
}

// The tests share no state, and run in parallel to keep it that way: a test
// relying on a global would race with the others under -race.
func TestEveryTestRunsInParallel(t *testing.T) {
	t.Parallel()

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") && fn.Name.Name != "TestMain" && !callsParallel(fn) {
				t.Errorf("%s: %s doesn't start with t.Parallel()", path, fn.Name.Name)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
