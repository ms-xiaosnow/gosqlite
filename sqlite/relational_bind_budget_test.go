package sqlite

import (
	"context"
	"database/sql/driver"
	"errors"
	"runtime"
	"strings"
	"testing"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

type relationalBlobValuer struct {
	data  []byte
	calls *int
}

func (v relationalBlobValuer) Value() (driver.Value, error) { *v.calls++; return v.data, nil }

// Descriptor-free admission tests. Input/callback allocations are caller-owned;
// the engine must charge the converted bytes before allocating its own clone.
func TestRelationalParameterClonePreflight(t *testing.T) {
	stmt, err := sqlast.Parse("SELECT ? FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 8<<20)
	for _, converted := range []bool{false, true} {
		calls := 0
		var arg any = data
		if converted {
			arg = relationalBlobValuer{data, &calls}
		}
		db := &DB{opts: Options{MaxSortBytes: 64 << 20}}
		ctx := WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 1 << 20})
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, _, err := db.bindSelect(ctx, stmt.(*sqlast.Select), []any{arg})
		runtime.ReadMemStats(&after)
		if !errors.Is(err, ErrMemoryLimit) {
			t.Fatal(err)
		}
		if after.TotalAlloc-before.TotalAlloc >= uint64(len(data))/2 {
			t.Fatal("BLOB allocated before admission", after.TotalAlloc-before.TotalAlloc)
		}
		if converted && calls != 1 {
			t.Fatal("converter repeated", calls)
		}
	}
	// A successful bind still owns a defensive BLOB copy.
	db := &DB{opts: Options{MaxSortBytes: 64 << 20}}
	input := []byte{1, 2, 3}
	_, params, err := db.bindSelect(context.Background(), stmt.(*sqlast.Select), []any{input})
	if err != nil {
		t.Fatal(err)
	}
	input[0] = 9
	if params[0].([]byte)[0] != 1 {
		t.Fatal("bound BLOB aliases caller")
	}
}

func TestRelationalAliasExpansionPreflight(t *testing.T) {
	projected, err := sqlast.ParseExpr("1" + strings.Repeat("+1", 90))
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := sqlast.ParseExpr("calculated" + strings.Repeat("+calculated", 90))
	if err != nil {
		t.Fatal(err)
	}
	b, err := newRelationalBudget(WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: 1000}), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	scope := &relationalScope{budget: b, slots: &table{index: map[string]int{}, ipk: -1}}
	if _, err := scope.bind(predicate, map[string]sqlast.Expr{"calculated": projected}); !errors.Is(err, ErrQueryWorkLimit) {
		t.Fatal(err, b.stats)
	}
}

func TestRelationalReservationCancellationPreflight(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b, err := newRelationalBudget(ctx, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := b.reserve(1 << 19); !errors.Is(err, context.Canceled) || b.stats.CurrentBytes != 0 {
		t.Fatal(err, b.stats)
	}
	if err := b.reserve(0); !errors.Is(err, context.Canceled) {
		t.Fatal("failure was not sticky", err)
	}
}
