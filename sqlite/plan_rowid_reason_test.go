package sqlite

import (
	"context"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"reflect"
	"strings"
	"testing"
)

// Descriptor-free proof: refusing a speculative order-only range must not
// rewrite the existing access plan or its more specific fallback reason.
func TestRowIDRangeRejectedOrderPreservesPlan(t *testing.T) {
	for _, reason := range []string{"high posting frequency", "empty normalized MATCH query", "candidate memory budget"} {
		for _, bounded := range []bool{false, true} {
			where := sqlast.Expr(&sqlast.Call{Name: "unknown_unproven", Args: []sqlast.Expr{&sqlast.Column{Name: "v"}}})
			if bounded {
				where = &sqlast.Binary{Op: "AND", Left: &sqlast.Binary{Op: ">", Left: &sqlast.Column{Name: "id"}, Right: &sqlast.Literal{Value: int64(4)}}, Right: where}
			}
			tab := &table{name: "t", columns: []sqlast.ColumnDef{{Name: "id", Type: "INTEGER", PrimaryKey: true}, {Name: "v", Type: "TEXT"}}, index: map[string]int{"id": 0, "v": 1}, ipk: 0}
			r := &Rows{table: tab, sel: &sqlast.Select{Where: where}, exprs: []sqlast.Expr{&sqlast.Column{Name: "id"}}, order: []orderExpr{{column: 0}}, queryPlan: QueryPlan{Access: "SCAN", Table: "t", Reason: reason}}
			before := r.queryPlan
			if err := (&DB{}).planRowIDRange(context.Background(), nil, r); err != nil {
				t.Fatal(err)
			}
			if bounded {
				if !strings.HasPrefix(r.queryPlan.Reason, reason+"; ") || !strings.Contains(r.queryPlan.Reason, "rowid range requires") {
					t.Fatalf("bounded fallback lost prior reason: %+v", r.queryPlan)
				}
				r.queryPlan.Reason = before.Reason
			}
			if !reflect.DeepEqual(r.queryPlan, before) || len(r.order) != 1 || r.source != nil {
				t.Fatalf("rejected planner mutated prior plan: before=%+v after=%+v", before, r.queryPlan)
			}
		}
	}
}
