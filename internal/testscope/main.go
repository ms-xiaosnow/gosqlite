// Command testscope checks high-risk references in ordinary test sources.
// This is a prevention aid, not a substitute for reviewing test behavior.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func inspectSource(path string, raw []byte) ([]string, error) {
	first := strings.SplitN(string(raw), "\n", 2)[0]
	// Exact approved constraints only: a prefix could hide "manual_namespace || linux".
	if first == "//go:build manual_recovery && linux" || first == "//go:build manual_namespace" {
		return nil, nil
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, 0)
	if err != nil {
		return nil, err
	}
	imports := map[string]string{}
	var findings []string
	for _, i := range file.Imports {
		p, e := strconv.Unquote(i.Path.Value)
		if e != nil {
			return nil, e
		}
		name := filepath.Base(p)
		if i.Name != nil {
			name = i.Name.Name
		}
		imports[name] = p
		if name == "." && (p == "os" || p == "os/exec" || p == "syscall" || p == "golang.org/x/sys/unix") {
			findings = append(findings, fmt.Sprintf("%s: high-risk dot import requires review", set.Position(i.Pos())))
		}
	}
	report := func(pos token.Pos, name string) {
		findings = append(findings, fmt.Sprintf("%s: restricted reference %s requires separate manual source and review", set.Position(pos), name))
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "RecoverRollback" {
				report(call.Pos(), id.Name)
			}
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		prohibited := false
		if x, ok := sel.X.(*ast.Ident); ok {
			p := imports[x.Name]
			if p == "os" {
				switch sel.Sel.Name {
				case "Rename", "Symlink", "Link", "Chmod", "Remove", "RemoveAll", "Truncate", "Exit":
					prohibited = true
				}
			}
			if p == "os/exec" && sel.Sel.Name == "CommandContext" {
				prohibited = true
			}
			if p == "syscall" || p == "golang.org/x/sys/unix" {
				switch sel.Sel.Name {
				case "Kill", "Rename", "Renameat", "Renameat2", "Unlink", "Unlinkat", "Truncate", "Ftruncate", "Chmod", "Fchmod":
					prohibited = true
				}
			}
		}
		switch sel.Sel.Name {
		case "Kill", "Signal", "Truncate", "Chmod", "RecoverRollback":
			prohibited = true
		}
		if prohibited {
			report(sel.Pos(), sel.Sel.Name)
		}
		return true
	})
	return findings, nil
}
func main() {
	if strings.Contains(os.Getenv("GOFLAGS"), "manual_") {
		fmt.Fprintln(os.Stderr, "manual test tags are forbidden in ordinary checks")
		os.Exit(1)
	}
	failed := false
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		findings, err := inspectSource(path, raw)
		if err != nil {
			return err
		}
		for _, f := range findings {
			fmt.Fprintln(os.Stderr, f)
			failed = true
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		failed = true
	}
	if failed {
		os.Exit(1)
	}
	fmt.Println("Ordinary test source-reference guard PASS; behavioral review is still required.")
}
