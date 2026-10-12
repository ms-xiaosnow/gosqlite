package driver

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Connector 构造仅解析字符串和复制配置；本文件绝不调用 Connect。
func TestWALTailPublicationConnectorMemorySnapshot(t *testing.T) {
	for _, mode := range []string{"ro", "rw", "rwc"} {
		input := sqlite.Options{WALExclusive: true, WALTailPublication: &sqlite.WALTailPublicationOptions{}, PageSize: 512}
		connector, err := NewConnector("file:source-only.db?mode="+mode, input)
		if err != nil || connector == nil {
			t.Fatal("inert connector construction failed", err)
		}
		input.WALTailPublication = nil
		input.PageSize = 4096
		if connector.opts.WALTailPublication == nil || connector.opts.PageSize != 512 || !connector.opts.WALExclusive {
			t.Fatal("captured configuration changed")
		}
		if connector.opts.ReadOnly != (mode == "ro") || connector.opts.NoCreate != (mode == "rw") {
			t.Fatal("DSN restrictions lost", mode)
		}
	}
	plain, err := NewConnector("source-only.db", sqlite.Options{})
	if err != nil || plain.opts.WALTailPublication != nil {
		t.Fatal("nil default changed", err)
	}
}

func TestWALTailPublicationConnectorRejectsDSNActivation(t *testing.T) {
	for _, key := range []string{"wal_tail_publication", "WALTailPublication", "wal-tail-publication", "wal_exclusive", "WALExclusive"} {
		if connector, err := NewConnector("file:source-only.db?"+key+"=true", sqlite.Options{}); err == nil || connector != nil {
			t.Fatal("DSN activation accepted", key)
		}
	}
}

func TestWALTailPublicationConnectorSourceBoundary(t *testing.T) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "driver.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	functions := make(map[string]string)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			var text bytes.Buffer
			if err := printer.Fprint(&text, set, fn); err != nil {
				t.Fatal(err)
			}
			functions[fn.Name.Name] = text.String()
		}
	}
	constructor := functions["NewConnector"]
	copyAt := strings.Index(constructor, "copy := *opts.WALTailPublication")
	storeAt := strings.Index(constructor, "opts.WALTailPublication = &copy")
	returnAt := strings.Index(constructor, "return &Connector{path: path, opts: opts}, nil")
	if copyAt < 0 || storeAt < copyAt || returnAt < storeAt ||
		strings.Index(constructor, "opts.ReadOnly = true") > copyAt ||
		strings.Index(constructor, "opts.NoCreate = true") > copyAt {
		t.Fatal("value copy or DSN restriction ordering changed")
	}
	if strings.Count(constructor, "copy := *opts.WALTailPublication") != 1 || strings.Count(constructor, "opts.WALTailPublication = &copy") != 1 {
		t.Fatal("configuration snapshot must be single-use")
	}
	for _, forbidden := range []string{"OpenContext(", "Connect(", "Getenv(", "LookupEnv(", "NewWALPublication(", "OpenSessionTailPublicationAdmitted("} {
		if strings.Contains(constructor, forbidden) {
			t.Fatal("connector construction gained an execution path", forbidden)
		}
	}
	if strings.Count(functions["Connect"], "sqlite.OpenContext(ctx, c.path, c.opts)") != 1 {
		t.Fatal("connection no longer uses public policy preflight")
	}
	if !strings.Contains(functions["parseDSNMode"], `key != "mode"`) || strings.Contains(functions["parseDSNMode"], "WALTailPublication") {
		t.Fatal("DSN policy was expanded")
	}
}
