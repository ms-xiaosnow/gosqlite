package vfs

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Source-only regression: these portable calls can conceal native Windows
// metadata resources. The non-Windows implementation remains explicitly
// isolated by its build constraint. Reading source creates no database state.
func TestMetadataOwnershipCallChainSource(t *testing.T) {
	for _, path := range []string{"metadata_default.go", "../../sqlite/path_default.go"} {
		portable, err := os.ReadFile(path)
		if err != nil || !strings.HasPrefix(string(portable), "//go:build !(windows && amd64)\n") {
			t.Fatal("portable metadata is not excluded from Windows/amd64", path, err)
		}
	}
	for _, dir := range []string{".", "../pager", "../walindex", "../../sqlite", "../../driver"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || (dir == "." && name == "metadata_default.go" || dir == "../../sqlite" && name == "path_default.go") {
				continue
			}
			path := filepath.Join(dir, name)
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			imports := map[string]string{}
			for _, spec := range file.Imports {
				p, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				alias := filepath.Base(p)
				if spec.Name != nil {
					alias = spec.Name.Name
				}
				if alias == "." && (p == "os" || p == "path/filepath") {
					t.Fatalf("unreviewable metadata dot import in %s", path)
				}
				imports[alias] = p
			}
			ast.Inspect(file, func(n ast.Node) bool {
				s, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				x, ok := s.X.(*ast.Ident)
				if !ok {
					return true
				}
				p := imports[x.Name]
				if p == "os" && (s.Sel.Name == "Stat" || s.Sel.Name == "Lstat" || s.Sel.Name == "SameFile") || p == "path/filepath" && s.Sel.Name == "EvalSymlinks" {
					t.Errorf("%s: bypasses platform-owned metadata: %s.%s", set.Position(s.Pos()), x.Name, s.Sel.Name)
				}
				return true
			})
		}
	}
}
