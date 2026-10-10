package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSparseExactExample(t *testing.T) {
	var buf bytes.Buffer
	if err := example(context.Background(), filepath.Join(t.TempDir(), "sparse.db"), &buf); err != nil {
		t.Fatal(err)
	}
	var got output
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Dimensions != 1000000000 || got.Nonzeros != 2 || got.EncodedBytes != 32 || got.SourceRows != 5 || got.ScoredNonzeros != 13 || got.SQLNonzeros != 13 || got.SQLRepresentation != "sparsevec" || got.SQLRanking != "bounded_heap" || !reflect.DeepEqual(got.Hits, got.SQLHits) || len(got.Hits) != 3 || got.Hits[0].RowID != 10 || got.Hits[1].Distance != 1 {
		t.Fatal(got)
	}
}
