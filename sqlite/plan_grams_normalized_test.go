package sqlite_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"golang.org/x/text/unicode/norm"
)

// This is an additional NFC-normalized INPUT corpus for goal §5.2. Production
// LIKE_RAW_V1 is intentionally unchanged, as are the separate raw/decomposed
// adversarial fixtures. The counting unit is query/text memberships, not queries.
func TestGramsNFCInputs12800RandomizedMemberships(t *testing.T) {
	const rows, queries = 128, 100
	const seed int64 = 2026101052
	rng := rand.New(rand.NewSource(seed))
	tokens := []string{"a", "B", "z", "数据库", "日本語", "かな", "한글", "🙂", "👩‍💻", "e\u0301", "A\u030a", "か\u3099", "한", "%", "_", "\\", " ", "\t", "é", "ß", "Σ"}
	texts := make([]string, rows)
	changed := 0
	hash := sha256.New()
	hashString := func(s string) {
		var size [8]byte
		binary.LittleEndian.PutUint64(size[:], uint64(len(s)))
		hash.Write(size[:])
		hash.Write([]byte(s))
	}
	for i := range texts {
		var raw strings.Builder
		for n := 4 + rng.Intn(12); n > 0; n-- {
			raw.WriteString(tokens[rng.Intn(len(tokens))])
		}
		texts[i] = norm.NFC.String(raw.String())
		if texts[i] != raw.String() {
			changed++
		}
		if !norm.NFC.IsNormalString(texts[i]) {
			t.Fatal("non-NFC input")
		}
		hashString(texts[i])
	}
	if changed < 32 {
		t.Fatalf("only %d randomized inputs changed under NFC", changed)
	}
	db, _ := writable(t, sqlite.Options{MaxTransactionPages: 8192})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range texts {
		txExecute(t, tx, "INSERT INTO docs VALUES(?,?)", i+1, s)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Record the real pre-index scan too; NOT INDEXED below is independently
	// required to remain a real scan once the persisted index exists.
	patterns := make([]string, queries)
	preIndex := make([][][]any, queries)
	for i := range patterns {
		rr := []rune(texts[rng.Intn(rows)])
		start := rng.Intn(len(rr))
		n := min(1+rng.Intn(5), len(rr)-start)
		literal := norm.NFC.String(string(rr[start : start+n]))
		pattern := escapeGramLiteral(literal, '\\')
		switch i % 4 {
		case 0:
			pattern = "%" + pattern + "%"
		case 1:
			pattern = "_" + pattern + "%"
		case 2:
			pattern = "%" + pattern + "_"
		}
		if !norm.NFC.IsNormalString(pattern) {
			t.Fatalf("pattern not NFC: %q", pattern)
		}
		patterns[i] = pattern
		hashString(pattern)
		var p sqlite.QueryPlan
		preIndex[i], p, _ = gramResults(t, db, "SELECT id FROM docs WHERE title LIKE ? ESCAPE '\\' ORDER BY id", pattern)
		if p.Access != "SCAN" {
			t.Fatalf("pre-index path: %+v", p)
		}
	}
	execute(t, db, "CREATE INDEX docs_nfc ON docs(title) USING grams13")
	searches, nonempty := 0, 0
	lengths := map[int]bool{}
	for i, pattern := range patterns {
		p, s, scan := gramCompare(t, db, "title LIKE ? ESCAPE '\\'", pattern)
		got, _, _ := gramResults(t, db, "SELECT id FROM docs WHERE title LIKE ? ESCAPE '\\' ORDER BY id", pattern)
		if !reflect.DeepEqual(got, preIndex[i]) {
			t.Fatalf("pre-build/full-scan mismatch for %q", pattern)
		}
		if p.Access == "SEARCH" {
			searches++
			if p.Method != "grams13" || p.Namespace != "LIKE_RAW_V1" || !p.Recheck {
				t.Fatalf("not raw grams: %+v", p)
			}
			if s.PostingKeysRead <= 0 || s.BaseRowFetches > scan.BaseRowFetches {
				t.Fatalf("not genuine posting work: %+v", s)
			}
			for _, n := range p.GramLengths {
				lengths[n] = true
			}
			if s.ReturnedRows > 0 {
				nonempty++
			}
		} else if p.Access != "SCAN" || p.Reason == "" {
			t.Fatalf("unexplained fallback: %+v", p)
		}
	}
	if searches < 60 || nonempty < 10 || !lengths[1] || !lengths[2] || !lengths[3] {
		t.Fatalf("SEARCH=%d nonempty=%d lengths=%v", searches, nonempty, lengths)
	}
	if err := db.CheckExtensionIndex(context.Background(), "docs_nfc"); err != nil {
		t.Fatal(err)
	}
	const wantHash = "ab4a4a4d2cbb9d85630b88d0d7b1bdeabf7d74820f69702ba4cf5431f3472944"
	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != wantHash {
		t.Fatalf("corpus identity changed: %s", got)
	}
	t.Logf("seed=%d NFC-changed=%d; %d patterns x %d randomized NFC texts = %d memberships; SEARCH=%d SCAN=%d nonempty_SEARCH=%d; length-prefixed UTF-8 corpus/pattern SHA256=%x", seed, changed, queries, rows, queries*rows, searches, queries-searches, nonempty, hash.Sum(nil))
}

// Ordinary DML/savepoint/rollback/checkpoint/reopen only. No fault/recovery path.
func TestGramsNFCLifecyclePostingConsistency(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared=%t", shared), func(t *testing.T) {
			opts := sqlite.Options{WALShared: shared}
			db, path := writable(t, opts)
			execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT)")
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 48; i++ {
				txExecute(t, tx, "INSERT INTO docs VALUES(?,?)", i+1, norm.NFC.String(fmt.Sprintf("filler%d e\u0301", i)))
			}
			txExecute(t, tx, "INSERT INTO docs VALUES(100,'甲'),(101,'甲乙'),(102,'甲乙丙')")
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			execute(t, db, "CREATE INDEX docs_nfc ON docs(title) USING grams13")
			check := func(q gramQuerier, checkIndex func(context.Context, string) error) {
				t.Helper()
				for n, needle := range []string{"甲", "甲乙", "甲乙丙"} {
					p, s, ss := gramCompare(t, q, "title LIKE ?", "%"+needle+"%")
					if p.Access != "SEARCH" || p.Method != "grams13" || p.Namespace != "LIKE_RAW_V1" || !p.Recheck || !reflect.DeepEqual(p.GramLengths, []int{n + 1}) || s.PostingKeysRead < 1 || s.BaseRowFetches >= ss.BaseRowFetches {
						t.Fatalf("plan/work %+v %+v %+v", p, s, ss)
					}
				}
				if err := checkIndex(ctx, "docs_nfc"); err != nil {
					t.Fatal(err)
				}
			}
			check(db, db.CheckExtensionIndex)
			tx, err = db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			txExecute(t, tx, "INSERT INTO docs VALUES(200,'甲乙丙')")
			txExecute(t, tx, "UPDATE docs SET id=201,title='甲乙丙é' WHERE id=100")
			txExecute(t, tx, "DELETE FROM docs WHERE id=101")
			check(tx, tx.CheckExtensionIndex)
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			check(db, db.CheckExtensionIndex)
			execute(t, db, "SAVEPOINT undo")
			execute(t, db, "UPDATE docs SET title='rolled back' WHERE id=102")
			if err = db.CheckExtensionIndex(ctx, "docs_nfc"); err != nil {
				t.Fatal(err)
			}
			execute(t, db, "ROLLBACK TO undo")
			execute(t, db, "RELEASE undo")
			check(db, db.CheckExtensionIndex)
			execute(t, db, "UPDATE docs SET id=201,title='甲乙丙é' WHERE id=100")
			execute(t, db, "DELETE FROM docs WHERE id=101")
			execute(t, db, "INSERT INTO docs VALUES(200,'甲乙丙')")
			check(db, db.CheckExtensionIndex)
			execute(t, db, "REINDEX docs_nfc")
			check(db, db.CheckExtensionIndex)
			if shared {
				peer, err := sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				check(peer, peer.CheckExtensionIndex)
				if err = peer.Close(); err != nil {
					t.Fatal(err)
				}
				if err = db.Checkpoint(ctx); err != nil {
					t.Fatal(err)
				}
				check(db, db.CheckExtensionIndex)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			check(reopened, reopened.CheckExtensionIndex)
			got, _, _ := gramResults(t, reopened, "SELECT id FROM docs WHERE title LIKE '%甲%' ORDER BY id")
			if !reflect.DeepEqual(got, [][]any{{int64(102)}, {int64(200)}, {int64(201)}}) {
				t.Fatal(got)
			}
			for _, pattern := range []string{"%", "%e%"} {
				p, _, _ := gramCompare(t, reopened, "title LIKE ?", pattern)
				if p.Access != "SCAN" || p.Reason == "" {
					t.Fatalf("fallback changed: %+v", p)
				}
			}
		})
	}
}
