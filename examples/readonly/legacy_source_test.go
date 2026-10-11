package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// The original executable is parsed as source data only, never compiled or run.
// The hash pins these bytes to the preserved 1352ae5 baseline main.go.
func TestReadonlyLegacySourceContract(t *testing.T) {
	const wantSHA256 = "69c32e2cf03e4a6f75099a111a645de562767f34decadfe89154dceec007053e"
	raw := []byte(readonlyLegacySource)
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != wantSHA256 {
		t.Fatalf("legacy source SHA256=%s, want %s", got, wantSHA256)
	}
	findings := strings.Join(readonlySourceContract(raw), "\n")
	for _, want := range []string{"main: forbidden call sql.Open", "main: forbidden call log.Fatal", "missing function example"} {
		if !strings.Contains(findings, want) {
			t.Fatalf("legacy source findings %q do not contain %q", findings, want)
		}
	}
	t.Log("Legacy source rejected by AST-only contract; old main was not executed")
}

const readonlyLegacySource = `// Read an existing supported SQLite database without CGO.
// Usage: go run ./examples/readonly ./testdata/readonly/page4096.db
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/ms-xiaosnow/gosqlite/driver"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: readonly <existing.db>")
	}
	db, err := sql.Open("gosqlite", os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rows, err := db.QueryContext(context.Background(), "SELECT id, label FROM samples WHERE id >= ? ORDER BY id LIMIT ?", int64(1), 3)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var label string
		if err := rows.Scan(&id, &label); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d\t%s\n", id, label)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}
`
