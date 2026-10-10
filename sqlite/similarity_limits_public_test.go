package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestSimilarityPublicErrorCodes(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "limits.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(context.Background(), "SELECT levenshtein(?,?)", strings.Repeat("a", 4097), "b")
	if err == nil {
		for rows.Next() {
		}
		err = rows.Err()
		rows.Close()
	}
	var coded *sqlite.Error
	if !errors.Is(err, sqlite.ErrMemoryLimit) || !errors.As(err, &coded) || coded.Code != sqlite.CodeNoMem {
		t.Fatalf("limit classification: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rows, err = db.Query(ctx, "SELECT ngram_similarity('a','b',1,'dice')")
	if rows != nil {
		rows.Close()
	}
	if !errors.Is(err, context.Canceled) || !errors.As(err, &coded) || coded.Code != sqlite.CodeInterrupt {
		t.Fatalf("cancel classification: %v", err)
	}
}
