package sql

import (
	"reflect"
	"strings"
	"testing"
)

func TestPGVectorIndexParseAccepted(t *testing.T) {
	for _, tc := range []struct {
		name, pg, local string
	}{
		{
			"goal_hnsw",
			`CREATE INDEX items_embedding_hnsw ON items USING hnsw
  (embedding vector_cosine_ops) WITH (m=16, ef_construction=64);`,
			`CREATE INDEX items_embedding_hnsw ON items(embedding) USING hnsw WITH(opclass='vector_cosine_ops',m=16,ef_construction=64);`,
		},
		{
			"goal_ivfflat",
			`CREATE INDEX items_embedding_ivf ON items USING ivfflat
  (embedding vector_l2_ops) WITH (lists=100);`,
			`CREATE INDEX items_embedding_ivf ON items(embedding) USING ivfflat WITH(opclass='vector_l2_ops',lists=100);`,
		},
		{
			"quoted_names_case_and_comments",
			`cReAtE InDeX IF NOT EXISTS "a""b" ON "table name" UsInG "hnsw" ("SELECT" "vector_cosine_ops") WITH ("m"=+16, /* construction */ EF_CONSTRUCTION=6_4); -- done`,
			`CREATE INDEX IF NOT EXISTS "a""b" ON "table name"("SELECT") USING hnsw WITH(opclass='vector_cosine_ops',m=16,ef_construction=64)`,
		},
		{"hnsw_l2_defaults", `CREATE INDEX i ON t USING hnsw (v vector_l2_ops)`, `CREATE INDEX i ON t(v) USING hnsw WITH(opclass='vector_l2_ops')`},
		{"hnsw_l1_defaults", `CREATE INDEX i ON t USING hnsw (v vector_l1_ops)`, `CREATE INDEX i ON t(v) USING hnsw WITH(opclass='vector_l1_ops')`},
		{"hnsw_ip_defaults", `CREATE INDEX i ON t USING hnsw (v vector_ip_ops)`, `CREATE INDEX i ON t(v) USING hnsw WITH(opclass='vector_ip_ops')`},
		{"ivf_ip_defaults", `CREATE INDEX i ON t USING IVFFLAT (v VECTOR_IP_OPS)`, `CREATE INDEX i ON t(v) USING ivfflat WITH(opclass='vector_ip_ops')`},
		{"ivf_cosine_defaults", `CREATE INDEX i ON t USING ivfflat (v vector_cosine_ops)`, `CREATE INDEX i ON t(v) USING ivfflat WITH(opclass='vector_cosine_ops')`},
		{"config_admits_values_later", `CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(m=-1,ef_construction=0)`, `CREATE INDEX i ON t(v) USING hnsw WITH(opclass='vector_l2_ops',m=-1,ef_construction=0)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := Parse(tc.pg)
			if err != nil {
				t.Fatal(err)
			}
			original, err := Parse(tc.local)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := parsed.(*CreateIndex)
			if !ok {
				t.Fatalf("statement type = %T", parsed)
			}
			want := original.(*CreateIndex)
			if got.SQL != tc.pg || got.Unique || got.ParameterCount() != 0 {
				t.Fatalf("unexpected source/metadata: %#v", got)
			}
			got.SQL, want.SQL = "", ""
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("PG AST differs from original grammar:\ngot  %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestPGVectorIndexParseRejectsUnsupported(t *testing.T) {
	for _, query := range []string{
		`CREATE UNIQUE INDEX i ON t USING hnsw (v vector_l2_ops)`,
		`CREATE INDEX CONCURRENTLY i ON t USING hnsw (v vector_l2_ops)`,
		`CREATE INDEX i ON ONLY t USING hnsw (v vector_l2_ops)`,
		`CREATE INDEX i ON t USING btree (v vector_l2_ops)`,
		`CREATE INDEX i ON t USING "HNSW" (v vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (v "VECTOR_L2_OPS")`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH("M"=16)`,
		`CREATE INDEX i ON t USING grams13 (v vector_l2_ops)`,
		`CREATE INDEX i ON t USING "hnsw; DROP TABLE t" (v vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw ()`,
		`CREATE INDEX i ON t USING hnsw (v)`,
		`CREATE INDEX i ON t USING hnsw (v 'vector_l2_ops')`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops vector_cosine_ops)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops, w vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops,)`,
		`CREATE INDEX i ON t USING hnsw (lower(v) vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw ((v) vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (v+1 vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (? vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops ASC)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops DESC)`,
		`CREATE INDEX i ON t USING hnsw (v COLLATE BINARY vector_l2_ops)`,
		`CREATE INDEX public.i ON t USING hnsw (v vector_l2_ops)`,
		`CREATE INDEX i ON public.t USING hnsw (v vector_l2_ops)`,
		`CREATE INDEX i ON t USING public.hnsw (v vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (t.v vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (v public.vector_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (v halfvec_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (v sparsevec_l2_ops)`,
		`CREATE INDEX i ON t USING hnsw (v bit_hamming_ops)`,
		`CREATE INDEX i ON t USING hnsw (v vector_other_ops)`,
		`CREATE INDEX i ON t USING ivfflat (v vector_l1_ops)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH()`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(m=16,)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(m=16,M=32)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(m=16,"M"=32)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(opclass='vector_l2_ops')`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH("OPCLASS"='vector_cosine_ops')`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(metric='cosine')`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(lists=100)`,
		`CREATE INDEX i ON t USING ivfflat (v vector_l2_ops) WITH(m=16)`,
		`CREATE INDEX i ON t USING ivfflat (v vector_l2_ops) WITH(ef_construction=64)`,
		`CREATE INDEX i ON t USING ivfflat (v vector_l2_ops) WITH(lists=100,"LISTS"=2)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(ef_search=40)`,
		`CREATE INDEX i ON t USING ivfflat (v vector_l2_ops) WITH(probes=1)`,
		`CREATE INDEX i ON t USING ivfflat (v vector_l2_ops) WITH(training_samples=100)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(max_nodes=100)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(seed=1)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(unknown=1)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WHERE v IS NOT NULL`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) INCLUDE(id)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) TABLESPACE other`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(m=16) WITH(m=16)`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) USING hnsw`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops); SELECT 1`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops);;`,
		`CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(m=16); DROP TABLE t; --`,
	} {
		t.Run(query, func(t *testing.T) {
			if got, err := Parse(query); err == nil {
				t.Fatalf("accepted unsupported syntax: %#v", got)
			}
		})
	}
	for _, value := range []string{"?", ":m", "NULL", "TRUE", "FALSE", "'16'", `"16"`, "X'10'", "16.0", "1.6e1", "0x10", "(16)", "16+0", "abs(16)", "(SELECT 16)", "--16", "-+16", "+-16", "++16", "9223372036854775808", "-9223372036854775809"} {
		t.Run("integer_"+value, func(t *testing.T) {
			query := "CREATE INDEX i ON t USING hnsw (v vector_l2_ops) WITH(m=" + value + ")"
			if got, err := Parse(query); err == nil {
				t.Fatalf("accepted noninteger option: %#v", got)
			}
		})
	}
}

func TestPGVectorIndexOriginalGrammarUnchanged(t *testing.T) {
	// The PG branch's narrow allowlist must not leak into project extension SQL.
	query := `CREATE INDEX IF NOT EXISTS "i" ON "t"(v COLLATE BINARY ASC,w DESC) USING "Custom" WITH(metric='cosine',opclass='custom_ops',seed=0x10,max_nodes=1_000,other=1.5,flag=TRUE,negative=-2)`
	parsed, err := Parse(query)
	if err != nil {
		t.Fatal(err)
	}
	got := parsed.(*CreateIndex)
	want := &CreateIndex{Base: Base{SQL: query, BindParams: []Parameter{}}, Name: "i", Table: "t", IfNotExists: true, Columns: []string{"v", "w"}, Desc: []bool{false, true}, Method: "Custom", Options: map[string]any{
		"metric": "cosine", "opclass": "custom_ops", "seed": int64(16), "max_nodes": int64(1000), "other": float64(1.5), "flag": int64(1), "negative": int64(-2),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("original extension AST changed: %#v", got)
	}
	if _, err = Parse(strings.Replace(query, "flag=TRUE", "flag=TRUE,FLAG=FALSE", 1)); err == nil {
		t.Fatal("original duplicate option rejection changed")
	}
}
