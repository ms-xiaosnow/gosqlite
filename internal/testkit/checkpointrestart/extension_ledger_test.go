package checkpointrestart

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"sort"
	"testing"
)

// These are synthetic complete values, not database output or a physical run.
// The graph is a small deterministic valid value graph solely for model tests.
func syntheticLedger(role Role) Ledger {
	l := Ledger{Schema: SchemaVersion, Fixture: FixtureID, Role: role, Generation: uint64(role), Owner: uint64(role) * 1000, SchemaCookie: 42, Check: CheckResult{Scope: CheckScope, Pages: 64, Tables: 12, Indexes: 9, ExtensionIndexes: 3, TableRows: 979, IndexEntries: 789, PageReads: 2000}, CheckCalls: 1, CheckReturnedNil: true}
	for i, sql := range StreamSQL() {
		l.Streams = append(l.Streams, Stream{ID: uint8(i + 1), SQL: sql, Args: []Value{}, Columns: StreamColumns(uint8(i + 1)), Rows: [][]Value{}, Receipt: RowsReceipt{Owner: uint64(role)*1000 + uint64(i) + 1, CompleteEOF: true, ErrNil: true, CloseCalls: 1, CloseNil: true}})
	}
	users := ExpectedUserRows()
	for i := range users {
		l.Streams[i].Rows = users[i]
	}
	specs := expectedSchema()
	for i, table := range []string{"docs", "hitems", "iitems"} {
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			specs = append(specs, guardSpec(int64(i+1), table, op))
		}
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].name < specs[j].name })
	root := int64(2)
	for _, s := range specs {
		r := int64(0)
		if s.typ != "trigger" {
			r = root
			root++
		}
		l.Streams[3].Rows = append(l.Streams[3].Rows, []Value{Text(s.typ), Text(s.name), Text(s.table), Integer(r), Text(s.sql)})
	}
	for i, name := range IndexNames() {
		table := []string{"docs", "hitems", "iitems"}[i]
		field := "embedding"
		if i == 0 {
			field = "body"
		}
		config := ExpectedConfigs()[i]
		l.Streams[4].Rows = append(l.Streams[4].Rows, []Value{Integer(int64(i + 1)), Text(name), Text(table), Text(hashText(CreateStatements()[i])), Text([]string{"grams13", "hnsw", "ivfflat"}[i]), Integer(1), Integer(1), Text(config), Text(hashText(config)), Integer(1), Integer(0), Integer([]int64{7, 180, 128}[i]), Integer(42)})
		l.Streams[5].Rows = append(l.Streams[5].Rows, []Value{Integer(int64(i + 1)), Integer(int64(i + 1)), Integer(1), Text(field), Integer(1)})
		l.Checks = append(l.Checks, IndexCheck{Name: name, Calls: 1, ReturnedNil: true})
	}
	counts := map[GramKey]int64{}
	for i, g := range ExpectedGrams() {
		l.Streams[6].Rows = append(l.Streams[6].Rows, []Value{Integer(int64(i + 1)), Integer(1), Integer(1), Text(g.Namespace), Integer(g.Length), Text(g.Text), Integer(g.RowID)})
		g.RowID = 0
		counts[g]++
	}
	keys := []GramKey{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Length != b.Length {
			return a.Length < b.Length
		}
		return a.Text < b.Text
	})
	for i, g := range keys {
		l.Streams[7].Rows = append(l.Streams[7].Rows, []Value{Integer(int64(i + 1)), Integer(1), Integer(1), Text(g.Namespace), Integer(g.Length), Text(g.Text), Integer(counts[g])})
	}
	l.Streams[8].Rows = [][]Value{{Integer(1), Integer(2), Integer(1), Integer(0), Integer(180), Integer(0), Integer(181), Text("")}, {Integer(2), Integer(3), Integer(0), Integer(-1), Integer(128), Integer(0), Integer(1), Text("ready")}}
	for id := int64(1); id <= 180; id++ {
		ids := []int64{}
		if id > 1 {
			ids = append(ids, id-1)
		}
		if id < 180 {
			ids = append(ids, id+1)
		}
		a := make([]byte, 16+8*len(ids))
		copy(a, "GHA\x00")
		a[4] = 1
		binary.BigEndian.PutUint32(a[8:12], 1)
		binary.BigEndian.PutUint16(a[12:14], uint16(len(ids)))
		for i, x := range ids {
			binary.BigEndian.PutUint64(a[16+i*8:], uint64(x))
		}
		v, _ := FixtureVector("hitems", id)
		l.Streams[9].Rows = append(l.Streams[9].Rows, []Value{Integer(id), Integer(2), Integer(id), Integer(id), Integer(1), Integer(0), Blob(Float32Vector(v)), Blob(a)})
		l.Streams[10].Rows = append(l.Streams[10].Rows, []Value{Integer(id), Integer(2), Integer(id), Integer(id)})
	}
	for c := 0; c < 4; c++ {
		v := [3]float32{float32(float64(c*1000) + .155), float32(float64(c*500) + .0775), 1}
		l.Streams[11].Rows = append(l.Streams[11].Rows, []Value{Integer(int64(c + 1)), Integer(3), Integer(int64(c + 1)), Blob(Float32Vector(v)), Integer(32)})
	}
	for id := int64(1); id <= 128; id++ {
		l.Streams[12].Rows = append(l.Streams[12].Rows, []Value{Integer(id), Integer(3), Integer((id-1)/32 + 1), Integer(id)})
	}
	for i, id := range QueryIDs() {
		var raw []byte
		if i < 2 {
			p := NativeSimilarityPlan{NativeQueryPlan: NativeQueryPlan{Access: "SEARCH", Table: "docs", Indexes: []string{"docs_grams"}, Method: "grams13", MethodVersion: 1}, CandidateMode: "union-all-query-gram-postings", QueryTokens: 3, ConfigHash: hashText("fixture-math"), TokenizerHash: hashText("fixture-tokenizer"), Generation: 1, MinGram: 2, MaxGram: 2, Metric: "dice", Threshold: .6, Limit: 2, Exact: true}
			s := NativeSimilarityStats{NativeQueryStats: NativeQueryStats{BaseRowFetches: 3, Candidates: 3, PostingKeysRead: 7, ReturnedRows: 2}, PostingRangesRead: 3, FilteredRows: 1, ScoredRows: 2, MatchedRows: 2}
			if i == 1 {
				p.Access = "SCAN"
				p.CandidateMode = "full-scan"
				s.BaseRowFetches = 7
				s.Candidates = 0
				s.PostingKeysRead = 0
				s.PostingRangesRead = 0
				s.ScoredRows = 5
			}
			raw, _ = json.Marshal(NativeSimilarityResult{Hits: []NativeSimilarityHit{{1, NativeScore{float64(2) / 3, 3, 3, 2}}, {2, NativeScore{float64(2) / 3, 3, 3, 2}}}, Plan: p, Stats: s})
		} else {
			table, index, method, count := "hitems", "hitems_hnsw", "hnsw", int64(180)
			if i >= 4 {
				table, index, method, count = "iitems", "iitems_ivf", "ivfflat", 128
			}
			p := NativeVectorPlan{NativeQueryPlan: NativeQueryPlan{Access: "SEARCH", Table: table, Indexes: []string{index}, Method: method, MethodVersion: 1, ANNOpclass: "vector_l2_ops", ANNGeneration: 1}, Field: "embedding", Metric: "l2", Dimensions: 3, Limit: 5}
			s := NativeVectorStats{NativeQueryStats: NativeQueryStats{BaseRowFetches: 32, ReturnedRows: 5, ANNNodeReads: 40, ANNEdgeReads: 60, ANNDistanceEvaluations: 40, ANNExactRescores: 32, ANNIdentityLookups: 32, ANNIterations: 1}, ScoredRows: 32, ScoredComponents: 99}
			if i >= 4 {
				p.ANNLists = 4
				p.ANNProbes = 1
				p.ANNMaxProbes = 4
				p.ANNEffectiveProbes = 1
				p.ANNTrainingState = "ready"
				s.ANNNodeReads = 0
				s.ANNEdgeReads = 0
				s.ANNDistanceEvaluations = 0
				s.ANNCentroidReads = 4
				s.ANNCentroidScores = 4
				s.ANNListsOpened = 1
				s.ANNMemberKeysRead = 32
			}
			if i%2 == 1 {
				p.NativeQueryPlan = NativeQueryPlan{Access: "SCAN", Table: table, Method: "exact_vector", MethodVersion: 1}
				p.Exact = true
				s = NativeVectorStats{NativeQueryStats: NativeQueryStats{BaseRowFetches: count, ReturnedRows: 5}, ScoredRows: count, ScoredComponents: 3 + 3*count}
			}
			raw, _ = json.Marshal(NativeVectorResult{Hits: ExpectedVectorHits(table), Plan: p, Stats: s})
		}
		l.Queries = append(l.Queries, QueryEvidence{ID: id, Result: raw, Executed: true})
	}
	return l
}
func cloneLedger(l Ledger) Ledger {
	b, _ := json.Marshal(l)
	var out Ledger
	_ = json.Unmarshal(b, &out)
	return out
}
func requireLedger(t *testing.T, l Ledger) {
	t.Helper()
	if p := ValidateLedger(l); p != NoProblem {
		t.Fatalf("synthetic ledger inconsistent: %s", p)
	}
}
func rejectLedger(t *testing.T, l Ledger) {
	t.Helper()
	if p := ValidateLedger(l); p == NoProblem {
		t.Fatal("accepted finite invalid ledger")
	}
}

func TestCheckpointExtensionFixtureRows(t *testing.T) {
	l := syntheticLedger(Controller)
	requireLedger(t, l)
	u := ExpectedUserRows()
	if len(u[0])+len(u[1])+len(u[2]) != 315 || len(l.Streams) != 13 {
		t.Fatal("closed fixture size")
	}
	for _, edit := range []func(*Ledger){func(l *Ledger) { l.Streams[0].Rows = l.Streams[0].Rows[:6] }, func(l *Ledger) { l.Streams[0].Rows[3][1] = Text("") }, func(l *Ledger) { l.Streams[1].Rows[179][1].Blob[23] ^= 1 }, func(l *Ledger) { l.Streams[2].Rows = l.Streams[2].Rows[:127] }, func(l *Ledger) { l.Streams[0].Receipt.CompleteEOF = false }, func(l *Ledger) { l.Streams[0].Receipt.CloseCalls = 2 }, func(l *Ledger) { l.Streams[0].SQL += " LIMIT 7" }, func(l *Ledger) { l.Streams[0].Args = []Value{Integer(1)} }, func(l *Ledger) { l.Streams[0].Rows[0][2].Text = "hidden" }} {
		x := cloneLedger(l)
		edit(&x)
		rejectLedger(t, x)
	}
	b, p := EncodeLedger(l)
	if p != NoProblem {
		t.Fatal(p)
	}
	if _, p = DecodeLedger(b); p != NoProblem {
		t.Fatal(p)
	}
	for _, bad := range [][]byte{append(append([]byte{}, b...), '\n'), append(append([]byte{}, b...), b...), bytes.Replace(b, []byte(`"schema":1`), []byte(`"schema":1,"schema":1`), 1), bytes.Replace(b, []byte(`"schema":1`), []byte(`"schema":1,"unknown":0`), 1)} {
		if _, p := DecodeLedger(bad); p == NoProblem {
			t.Fatal("accepted noncanonical ledger")
		}
	}
	other := syntheticLedger(Probe)
	other.Check.PageReads++
	if CompareLedgers(l, other) != NoProblem {
		t.Fatal("role and owner receipt independence")
	}
	other.Check.FreePages++
	if CompareLedgers(l, other) == NoProblem {
		t.Fatal("lost frozen free-page count")
	}
}
func TestCheckpointExtensionGramsLedger(t *testing.T) {
	l := syntheticLedger(Controller)
	requireLedger(t, l)
	if len(ExpectedGrams()) != 96 || len(l.Streams[7].Rows) != 68 {
		t.Fatal("complete two-namespace arithmetic")
	}
	for _, edit := range []func(*Ledger){func(l *Ledger) { l.Streams[6].Rows = l.Streams[6].Rows[1:] }, func(l *Ledger) { l.Streams[6].Rows[0][6] = Integer(999) }, func(l *Ledger) {
		l.Streams[6].Rows[1][3] = l.Streams[6].Rows[0][3]
		l.Streams[6].Rows[1][4] = l.Streams[6].Rows[0][4]
		l.Streams[6].Rows[1][5] = l.Streams[6].Rows[0][5]
		l.Streams[6].Rows[1][6] = l.Streams[6].Rows[0][6]
	}, func(l *Ledger) { l.Streams[7].Rows[0][6].Int++ }, func(l *Ledger) { l.Streams[4].Rows[0][7].Text += " " }, func(l *Ledger) { l.Streams[4].Rows[0][3] = Text(hashText("wrong source")) }, func(l *Ledger) { l.Streams[3].Rows = l.Streams[3].Rows[1:] }} {
		x := cloneLedger(l)
		edit(&x)
		rejectLedger(t, x)
	}
}
func TestCheckpointExtensionGraphMembershipLedger(t *testing.T) {
	l := syntheticLedger(Controller)
	requireLedger(t, l)
	for _, edit := range []func(*Ledger){func(l *Ledger) { l.Streams[9].Rows = l.Streams[9].Rows[:179] }, func(l *Ledger) { l.Streams[9].Rows[0][6] = l.Streams[9].Rows[1][6] }, func(l *Ledger) { binary.BigEndian.PutUint64(l.Streams[9].Rows[0][7].Blob[16:], 999) }, func(l *Ledger) { l.Streams[10].Rows[0][3] = Integer(2) }, func(l *Ledger) { l.Streams[12].Rows = l.Streams[12].Rows[:32] }, func(l *Ledger) { l.Streams[12].Rows[0][2] = Integer(2) }, func(l *Ledger) { l.Streams[12].Rows[1][3] = Integer(1) }, func(l *Ledger) { l.Streams[12].Rows[0][3] = Integer(999) }, func(l *Ledger) { l.Streams[11].Rows[0][4] = Integer(31) }, func(l *Ledger) { l.Streams[11].Rows = l.Streams[11].Rows[:3] }, func(l *Ledger) { l.Streams[8].Rows[1][7] = Text("untrained") }, func(l *Ledger) { l.Streams[9].Rows[0][1] = Integer(3) }} {
		x := cloneLedger(l)
		edit(&x)
		rejectLedger(t, x)
	}
	x := cloneLedger(l)
	x.Streams[9].Rows[0][0] = Integer(-1)
	rejectLedger(t, x)
	// Even a second semantically valid graph representation cannot replace C.
	x = cloneLedger(l)
	x.Streams[9].Rows[0][7] = Blob([]byte{'G', 'H', 'A', 0, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0})
	requireLedger(t, x)
	if CompareLedgers(l, x) == NoProblem {
		t.Fatal("complete adjacency was not frozen")
	}
}
func TestCheckpointExtensionQueryPathsAndCheck(t *testing.T) {
	l := syntheticLedger(Controller)
	requireLedger(t, l)
	want := [2][]int64{{20, 19, 18, 17, 23}, {1, 2, 3, 4, 5}}
	for i, table := range []string{"hitems", "iitems"} {
		for j, h := range ExpectedVectorHits(table) {
			if h.RowID != want[i][j] {
				t.Fatal("independent source-vector ranking")
			}
		}
	}
	for _, edit := range []func(*Ledger){func(l *Ledger) { l.Check.TableRows = 978 }, func(l *Ledger) { l.Check.IndexEntries = 788 }, func(l *Ledger) { l.Check.ExtensionIndexes = 2 }, func(l *Ledger) { l.Check.PageReads = 0 }, func(l *Ledger) { l.Check.Pages = 4097 }, func(l *Ledger) { l.Checks[1].ReturnedNil = false }, func(l *Ledger) { l.Queries[0].Executed = false }, func(l *Ledger) {
		var q NativeVectorResult
		_ = json.Unmarshal(l.Queries[2].Result, &q)
		q.Stats.ANNNodeReads = 0
		l.Queries[2].Result, _ = json.Marshal(q)
	}, func(l *Ledger) {
		var q NativeVectorResult
		_ = json.Unmarshal(l.Queries[4].Result, &q)
		q.Plan.Exact = true
		q.Plan.Access = "SCAN"
		l.Queries[4].Result, _ = json.Marshal(q)
	}, func(l *Ledger) {
		var q NativeVectorResult
		_ = json.Unmarshal(l.Queries[2].Result, &q)
		q.Hits[0].Distance = math.Sqrt(2)
		l.Queries[2].Result, _ = json.Marshal(q)
	}, func(l *Ledger) {
		var q NativeSimilarityResult
		_ = json.Unmarshal(l.Queries[0].Result, &q)
		q.Stats.PostingRangesRead = 0
		l.Queries[0].Result, _ = json.Marshal(q)
	}} {
		x := cloneLedger(l)
		edit(&x)
		rejectLedger(t, x)
	}
}
