package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Independent membership oracle: ordinary valid SQL only. It computes distances
// directly from persisted centroid/source components, without ann.Assign.
func reviewIVFMemberships(t *testing.T, db *sqlite.DB) {
	t.Helper()
	configs := extensionRead(t, db, "SELECT id,name FROM __gosqlite_ext_catalog ORDER BY id")
	sources := extensionRead(t, db, "SELECT id,v FROM review_items WHERE v IS NOT NULL ORDER BY id")
	metrics := map[string]vector.MetricKind{"review_l2": vector.L2, "review_ip": vector.NegativeIP, "review_cosine": vector.Cosine}
	for _, config := range configs {
		metric, ok := metrics[config[1].(string)]
		if !ok {
			continue
		}
		indexID := config[0].(int64)
		rows := extensionRead(t, db, fmt.Sprintf("SELECT list_id,centroid,member_count FROM __gosqlite_ext_ann_centroids WHERE index_id=%d ORDER BY list_id", indexID))
		centers := make([][]float32, len(rows))
		counts := make([]int64, len(rows))
		for i, row := range rows {
			v, err := vector.Decode(row[1].([]byte))
			if err != nil {
				t.Fatal(err)
			}
			centers[i] = v.Values()
		}
		members := extensionRead(t, db, fmt.Sprintf("SELECT content_rowid,list_id FROM __gosqlite_ext_ann_members WHERE index_id=%d ORDER BY content_rowid", indexID))
		if len(members) != len(sources) {
			t.Fatalf("membership cardinality: %d != %d", len(members), len(sources))
		}
		for i, source := range sources {
			v, err := vector.Decode(source[1].([]byte))
			if err != nil {
				t.Fatal(err)
			}
			values := v.Values()
			best, score := 0, math.Inf(1)
			for j, center := range centers {
				var distance, dot, aa, bb float64
				for k, x := range values {
					a, b := float64(x), float64(center[k])
					distance += (a - b) * (a - b)
					dot += a * b
					aa += a * a
					bb += b * b
				}
				if metric == vector.NegativeIP {
					distance = -dot
				}
				if metric == vector.Cosine {
					distance = 1 - math.Max(-1, math.Min(1, dot/math.Sqrt(aa*bb)))
				}
				if distance < score {
					best, score = j, distance
				}
			}
			if members[i][0] != source[0] || members[i][1] != int64(best+1) {
				t.Fatalf("%s row %v assigned %v; independent nearest list %d", config[1], source[0], members[i], best+1)
			}
			counts[best]++
		}
		for i, row := range rows {
			if row[2] != counts[i] {
				t.Fatalf("centroid count mismatch: %v != %d", row, counts[i])
			}
		}
	}
}

func TestIVFReviewIndependentMembershipExtremeRowIDsAndStatementUndo(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE review_items(id INTEGER PRIMARY KEY,v VECTOR(4),note TEXT)")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 37; i++ {
		id := int64(i - 19)
		if i == 0 {
			id = math.MinInt64
		}
		if i == 36 {
			id = math.MaxInt64
		}
		txExecute(t, tx, "INSERT INTO review_items VALUES(?,?,'base')", id, fmt.Sprintf("[%d,%d,%d,%d]", (i*17)%29-14, (i*7)%19-9, i%11+1, (i*13)%23-11))
	}
	txExecute(t, tx, "INSERT INTO review_items VALUES(700,NULL,'null')")
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct {
		name   string
		metric vector.MetricKind
	}{{"review_l2", vector.L2}, {"review_ip", vector.NegativeIP}, {"review_cosine", vector.Cosine}} {
		if err = db.CreateIVFFlatIndex(ctx, m.name, "review_items", "v", sqlite.IVFFlatIndexOptions{Lists: 5, TrainingSamples: 9, Metric: m.metric}); err != nil {
			t.Fatal(err)
		}
	}
	reviewIVFMemberships(t, db)
	centers := extensionRead(t, db, "SELECT index_id,list_id,centroid FROM __gosqlite_ext_ann_centroids ORDER BY index_id,list_id")
	execute(t, db, "UPDATE review_items SET id=?,v='[120,-20,11,9]' WHERE id=?", int64(math.MinInt64+1), int64(math.MinInt64))
	execute(t, db, "UPDATE review_items SET v=NULL WHERE id=-17")
	execute(t, db, "UPDATE review_items SET v='[-14,99,3,71]' WHERE id=700")
	execute(t, db, "DELETE FROM review_items WHERE id=?", int64(math.MaxInt64))
	reviewIVFMemberships(t, db)
	if got := extensionRead(t, db, "SELECT index_id,list_id,centroid FROM __gosqlite_ext_ann_centroids ORDER BY index_id,list_id"); !reflect.DeepEqual(got, centers) {
		t.Fatal("ordinary DML changed frozen centroid bits")
	}
	before := extensionRead(t, db, "SELECT * FROM __gosqlite_ext_ann_members ORDER BY id")
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "UPDATE review_items SET note='kept' WHERE id=700")
	if _, err = tx.Exec(ctx, "INSERT INTO review_items VALUES(800,'[1,2,3,4]','partial'),(801,'[0,0,0,0]','bad')"); !errors.Is(err, sqlite.ErrConstraint) {
		t.Fatalf("late cosine failure: %v", err)
	}
	for _, name := range []string{"review_l2", "review_ip", "review_cosine"} {
		if err = tx.CheckExtensionIndex(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := extensionRead(t, db, "SELECT * FROM __gosqlite_ext_ann_members ORDER BY id"); !reflect.DeepEqual(got, before) {
		t.Fatal("failed later-index update changed memberships")
	}
	if got := extensionRead(t, db, "SELECT note FROM review_items WHERE id=700"); got[0][0] != "kept" {
		t.Fatal("statement failure lost prior statement")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reviewIVFMemberships(t, db)
	for _, metric := range []vector.MetricKind{vector.L2, vector.NegativeIP, vector.Cosine} {
		o := sqlite.VectorSearchOptions{Metric: metric, Limit: 100, Where: "id<10"}
		want, err := db.VectorSearch(ctx, "review_items", "v", "[3,1,2,4]", o)
		if err != nil {
			t.Fatal(err)
		}
		o.Mode = sqlite.VectorModeApproximate
		got, err := db.VectorSearch(ctx, "review_items", "v", "[3,1,2,4]", o)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want.Hits, got.Hits) || !got.Plan.Exact || got.Stats.ANNListsOpened != 5 || got.Stats.ANNCentroidScores != 5 || got.Stats.ANNNodeReads != 0 {
			t.Fatalf("all-list fallback differs: want=%+v got=%+v", want, got)
		}
	}
}
