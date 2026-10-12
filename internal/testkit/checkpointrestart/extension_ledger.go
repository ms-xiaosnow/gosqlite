// SOURCE_ONLY / NOT_RUN. This bounded pure ledger never opens a database or
// certifies that supplied native observations actually occurred.
package checkpointrestart

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	MaxLedgerBytes   = 4 << 20
	MaxResultRows    = 4096
	MaxCellBytes     = 64 << 10
	MaxSchemaEntries = 64
	CheckScope       = "gosqlite-supported-profile-v1"
)

type ValueKind string

const (
	NullValue    ValueKind = "null"
	IntegerValue ValueKind = "integer"
	RealValue    ValueKind = "real"
	TextValue    ValueKind = "text"
	BlobValue    ValueKind = "blob"
)

type Value struct {
	Kind     ValueKind `json:"kind"`
	Int      int64     `json:"int"`
	RealBits uint64    `json:"real_bits"`
	Text     string    `json:"text"`
	Blob     []byte    `json:"blob"`
}

func Null() Value           { return Value{Kind: NullValue} }
func Integer(v int64) Value { return Value{Kind: IntegerValue, Int: v} }
func Text(v string) Value   { return Value{Kind: TextValue, Text: v} }
func Blob(v []byte) Value   { return Value{Kind: BlobValue, Blob: append([]byte{}, v...)} }
func Real(v float64) Value  { return Value{Kind: RealValue, RealBits: math.Float64bits(v)} }

type RowsReceipt struct {
	Owner       uint64 `json:"owner"`
	CompleteEOF bool   `json:"complete_eof"`
	ErrNil      bool   `json:"err_nil"`
	CloseCalls  uint8  `json:"close_calls"`
	CloseNil    bool   `json:"close_nil"`
	ErrorToken  uint64 `json:"error_token"`
}
type Stream struct {
	ID      uint8       `json:"id"`
	SQL     string      `json:"sql"`
	Args    []Value     `json:"args"`
	Columns []string    `json:"columns"`
	Rows    [][]Value   `json:"rows"`
	Receipt RowsReceipt `json:"receipt"`
}
type QueryEvidence struct {
	ID         string `json:"id"`
	Result     []byte `json:"result"`
	Executed   bool   `json:"executed"`
	ErrorToken uint64 `json:"error_token"`
}
type IndexCheck struct {
	Name        string `json:"name"`
	Calls       uint8  `json:"calls"`
	ReturnedNil bool   `json:"returned_nil"`
	ErrorToken  uint64 `json:"error_token"`
}
type CheckResult struct {
	Scope            string `json:"scope"`
	Pages            uint32 `json:"pages"`
	FreePages        uint32 `json:"free_pages"`
	ReservedPages    uint32 `json:"reserved_pages"`
	Tables           int    `json:"tables"`
	Indexes          int    `json:"indexes"`
	ExtensionIndexes int    `json:"extension_indexes"`
	TableRows        uint64 `json:"table_rows"`
	IndexEntries     uint64 `json:"index_entries"`
	PageReads        uint64 `json:"page_reads"`
}
type Ledger struct {
	Schema           uint16          `json:"schema"`
	Fixture          string          `json:"fixture"`
	Role             Role            `json:"role"`
	Generation       uint64          `json:"generation"`
	Owner            uint64          `json:"owner"`
	SchemaCookie     uint32          `json:"schema_cookie"`
	Streams          []Stream        `json:"streams"`
	Queries          []QueryEvidence `json:"queries"`
	Checks           []IndexCheck    `json:"checks"`
	Check            CheckResult     `json:"check"`
	CheckCalls       uint8           `json:"check_calls"`
	CheckReturnedNil bool            `json:"check_returned_nil"`
	CheckErrorToken  uint64          `json:"check_error_token"`
}

func StreamSQL() [13]string {
	return [13]string{
		"SELECT id,body,active FROM docs ORDER BY id",
		"SELECT id,embedding,active,title FROM hitems ORDER BY id",
		"SELECT id,embedding,active,title FROM iitems ORDER BY id",
		"SELECT type,name,tbl_name,rootpage,sql FROM sqlite_schema ORDER BY name",
		"SELECT id,name,source_table,source_hash,method,method_version,storage_version,config,config_hash,generation,needs_reindex,source_rows,schema_cookie FROM __gosqlite_ext_catalog ORDER BY id",
		"SELECT id,index_id,field_id,column_name,column_ordinal FROM __gosqlite_ext_fields ORDER BY id",
		"SELECT id,index_id,field_id,namespace,gram_length,gram,content_rowid FROM __gosqlite_ext_postings ORDER BY id",
		"SELECT id,index_id,field_id,namespace,gram_length,gram,documents FROM __gosqlite_ext_frequency ORDER BY id",
		"SELECT id,index_id,entry_node,max_level,live_count,tombstone_count,next_node_id,training_state FROM __gosqlite_ext_ann_state ORDER BY id",
		"SELECT id,index_id,node_id,content_rowid,live,level,original_vector,adjacency FROM __gosqlite_ext_ann_nodes ORDER BY id",
		"SELECT id,index_id,content_rowid,node_id FROM __gosqlite_ext_ann_live ORDER BY id",
		"SELECT id,index_id,list_id,centroid,member_count FROM __gosqlite_ext_ann_centroids ORDER BY id",
		"SELECT id,index_id,list_id,content_rowid FROM __gosqlite_ext_ann_members ORDER BY id",
	}
}
func StreamColumns(id uint8) []string {
	if id < 1 || id > 13 {
		return nil
	}
	q := StreamSQL()[id-1]
	return strings.Split(q[len("SELECT "):strings.Index(q, " FROM ")], ",")
}
func QueryIDs() [6]string {
	return [6]string{"grams-indexed", "grams-scan", "hnsw-indexed", "hnsw-scan", "ivf-indexed", "ivf-scan"}
}
func IndexNames() [3]string { return [3]string{"docs_grams", "hitems_hnsw", "iitems_ivf"} }
func CreateStatements() [3]string {
	return [3]string{"CREATE TABLE docs(id INTEGER PRIMARY KEY,body,active INTEGER)", "CREATE TABLE hitems(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL,active INTEGER,title TEXT)", "CREATE TABLE iitems(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL,active INTEGER,title TEXT)"}
}

// Float32Vector is the source-derived GSV v1 encoding, independent of vector's
// parser/encoder. Each fixed fixture component is checked bit for bit.
func Float32Vector(v [3]float32) []byte {
	b := make([]byte, 24)
	copy(b, "GSV\x00")
	b[4], b[5], b[6] = 1, 1, 1
	binary.BigEndian.PutUint32(b[8:12], 3)
	for i, x := range v {
		binary.BigEndian.PutUint32(b[12+4*i:], math.Float32bits(x))
	}
	return b
}
func DecodeFloat32Vector(b []byte) ([3]float32, bool) {
	var v [3]float32
	if len(b) != 24 || string(b[:4]) != "GSV\x00" || b[4] != 1 || b[5] != 1 || b[6] != 1 || b[7] != 0 || binary.BigEndian.Uint32(b[8:12]) != 3 {
		return v, false
	}
	for i := range v {
		bits := binary.BigEndian.Uint32(b[12+4*i:])
		if bits&0x7f800000 == 0x7f800000 {
			return [3]float32{}, false
		}
		v[i] = math.Float32frombits(bits)
	}
	return v, true
}
func FixtureVector(table string, id int64) ([3]float32, bool) {
	if table == "hitems" && id >= 1 && id <= 180 {
		return [3]float32{float32(id), float32(id % 7), 1}, true
	}
	if table != "iitems" || id < 1 || id > 128 {
		return [3]float32{}, false
	}
	c, o := (id-1)/32, (id-1)%32
	// Preserve six decimal SQL formatting followed by direct binary32 parse.
	a, e1 := strconv.ParseFloat(fmt.Sprintf("%f", float64(c*1000)+float64(o)/100), 32)
	b, e2 := strconv.ParseFloat(fmt.Sprintf("%f", float64(c*500)+float64(o)/200), 32)
	return [3]float32{float32(a), float32(b), 1}, e1 == nil && e2 == nil
}
func ExpectedUserRows() [3][][]Value {
	r := [3][][]Value{}
	r[0] = [][]Value{{Integer(-9), Text("abcd"), Integer(0)}, {Integer(1), Text("abce"), Integer(1)}, {Integer(2), Text("abcf"), Integer(1)}, {Integer(3), Null(), Integer(1)}, {Integer(4), Text("disjoint"), Integer(1)}, {Integer(5), Text(""), Integer(1)}, {Integer(6), Text("a"), Integer(1)}}
	for k, n := range []int64{180, 128} {
		table := []string{"hitems", "iitems"}[k]
		for id := int64(1); id <= n; id++ {
			v, _ := FixtureVector(table, id)
			active := int64(0)
			if k == 0 && id%19 == 0 || k == 1 && id > 96 {
				active = 1
			}
			r[k+1] = append(r[k+1], []Value{Integer(id), Blob(Float32Vector(v)), Integer(active), Text(fmt.Sprintf("item %d", id))})
		}
	}
	return r
}
func validValue(v Value) bool {
	if len(v.Text) > MaxCellBytes || len(v.Blob) > MaxCellBytes {
		return false
	}
	switch v.Kind {
	case NullValue:
		return v.Int == 0 && v.RealBits == 0 && v.Text == "" && v.Blob == nil
	case IntegerValue:
		return v.RealBits == 0 && v.Text == "" && v.Blob == nil
	case RealValue:
		return v.Int == 0 && v.Text == "" && v.Blob == nil && !math.IsNaN(math.Float64frombits(v.RealBits)) && !math.IsInf(math.Float64frombits(v.RealBits), 0)
	case TextValue:
		return v.Int == 0 && v.RealBits == 0 && v.Blob == nil
	case BlobValue:
		return v.Int == 0 && v.RealBits == 0 && v.Text == "" && v.Blob != nil
	}
	return false
}
func valueEqual(a, b Value) bool {
	return a.Kind == b.Kind && a.Int == b.Int && a.RealBits == b.RealBits && a.Text == b.Text && bytes.Equal(a.Blob, b.Blob) && (a.Blob == nil) == (b.Blob == nil)
}
func rowsEqual(a, b [][]Value) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if !valueEqual(a[i][j], b[i][j]) {
				return false
			}
		}
	}
	return true
}
func isInt(v Value, n int64) bool   { return v.Kind == IntegerValue && v.Int == n }
func isText(v Value, s string) bool { return v.Kind == TextValue && v.Text == s }
func rowKinds(row []Value, kinds ...ValueKind) bool {
	if len(row) != len(kinds) {
		return false
	}
	for i, k := range kinds {
		if row[i].Kind != k {
			return false
		}
	}
	return true
}
func hashText(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

type schemaSpec struct{ typ, name, table, sql string }

func expectedSchema() []schemaSpec {
	var specs []schemaSpec
	for i, name := range []string{"docs", "hitems", "iitems"} {
		specs = append(specs, schemaSpec{"table", name, name, CreateStatements()[i]})
	}
	tables := [][2]string{
		{"catalog", "id INTEGER PRIMARY KEY, name TEXT NOT NULL, source_table TEXT NOT NULL, source_hash TEXT NOT NULL, method TEXT NOT NULL, method_version INTEGER NOT NULL, storage_version INTEGER NOT NULL, config TEXT NOT NULL, config_hash TEXT NOT NULL, generation INTEGER NOT NULL, needs_reindex INTEGER NOT NULL, source_rows INTEGER NOT NULL, schema_cookie INTEGER NOT NULL"},
		{"fields", "id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, field_id INTEGER NOT NULL, column_name TEXT NOT NULL, column_ordinal INTEGER NOT NULL"},
		{"postings", "id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, field_id INTEGER NOT NULL, namespace TEXT NOT NULL, gram_length INTEGER NOT NULL, gram TEXT NOT NULL, content_rowid INTEGER NOT NULL"},
		{"frequency", "id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, field_id INTEGER NOT NULL, namespace TEXT NOT NULL, gram_length INTEGER NOT NULL, gram TEXT NOT NULL, documents INTEGER NOT NULL"},
		{"ann_state", "id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, entry_node INTEGER NOT NULL, max_level INTEGER NOT NULL, live_count INTEGER NOT NULL, tombstone_count INTEGER NOT NULL, next_node_id INTEGER NOT NULL, training_state TEXT NOT NULL"},
		{"ann_nodes", "id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, node_id INTEGER NOT NULL, content_rowid INTEGER NOT NULL, live INTEGER NOT NULL, level INTEGER NOT NULL, original_vector BLOB NOT NULL, adjacency BLOB NOT NULL"},
		{"ann_live", "id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, content_rowid INTEGER NOT NULL, node_id INTEGER NOT NULL"},
		{"ann_centroids", "id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, list_id INTEGER NOT NULL, centroid BLOB NOT NULL, member_count INTEGER NOT NULL"},
		{"ann_members", "id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, list_id INTEGER NOT NULL, content_rowid INTEGER NOT NULL"},
	}
	for _, t := range tables {
		n := "__gosqlite_ext_" + t[0]
		specs = append(specs, schemaSpec{"table", n, n, "CREATE TABLE " + n + " (" + t[1] + ")"})
	}
	indexes := [][3]string{{"fields_lookup", "fields", "index_id, field_id"}, {"postings_lookup", "postings", "index_id, field_id, namespace, gram_length, gram, content_rowid"}, {"frequency_lookup", "frequency", "index_id, field_id, namespace, gram_length, gram"}, {"ann_state_lookup", "ann_state", "index_id"}, {"ann_nodes_lookup", "ann_nodes", "index_id, node_id"}, {"ann_live_lookup", "ann_live", "index_id, content_rowid"}, {"ann_centroids_lookup", "ann_centroids", "index_id, list_id"}, {"ann_members_lookup", "ann_members", "index_id, list_id, content_rowid"}, {"ann_members_reverse", "ann_members", "index_id, content_rowid"}}
	for _, x := range indexes {
		n, t := "__gosqlite_ext_"+x[0], "__gosqlite_ext_"+x[1]
		specs = append(specs, schemaSpec{"index", n, t, "CREATE INDEX " + n + " ON " + t + " (" + x[2] + ")"})
	}
	return specs
}
func guardSpec(id int64, table, op string) schemaSpec {
	n := fmt.Sprintf("__gosqlite_ext_guard_%d_%s", id, strings.ToLower(op))
	return schemaSpec{"trigger", n, table, fmt.Sprintf("CREATE TRIGGER %s AFTER %s ON %s BEGIN UPDATE %s SET needs_reindex = 1 WHERE id = %d; END", strconv.Quote(n), op, strconv.Quote(table), strconv.Quote("__gosqlite_ext_catalog"), id)}
}

// Exact canonical defaults are frozen from grams13/config.go, registry.go,
// ann/config.go and ivf_config.go, including Go 1.27's Unicode 17 tables.
func ExpectedConfigs() [3]string {
	limits := `{"max_input_bytes":1048576,"max_output_bytes":1048576,"max_input_runes":262144,"max_output_runes":262144,"max_distinct_tokens":262144,"max_token_bytes":2097152}`
	tail := `,"normalization_unicode_version":"17.0.0","case_fold_unicode_version":"17.0.0","category_unicode_version":"17.0.0","dependency_version":"golang.org/x/text@v0.43.0","limits":` + limits + `}`
	like := `{"version":1,"namespace":"LIKE_RAW_V1","normalization":"none","case_fold":"ascii-A-Z","whitespace":"preserve","punctuation":"preserve","nul":"first-NUL","stream_safe":"not-applicable"` + tail
	match := `{"version":1,"namespace":"MATCH_CONFIG_V1","normalization":"NFC","case_fold":"none","whitespace":"preserve","punctuation":"preserve","nul":"preserve","stream_safe":"reject-inserted-CGJ-v1"` + tail
	return [3]string{`{"version":1,"like":` + like + `,"match":` + match + `}`,
		`{"method":"hnsw","version":1,"dimensions":3,"metric":"l2","opclass":"vector_l2_ops","seed":2611923443488327891,"m":16,"ef_construction":64,"max_nodes":100000,"max_tombstones":25000,"max_visited":4096,"max_distance_evaluations":1000000,"max_memory_bytes":33554432,"build_policy":"splitmix64-geometric-v1","zero_policy":"reject-cosine-zero","max_level":32}`,
		`{"method":"ivfflat","version":1,"dimensions":3,"metric":"l2","opclass":"vector_l2_ops","seed":2611923443488327891,"lists":4,"training_samples":10000,"training_iterations":20,"max_training_work":268435456,"max_nodes":100000,"max_visited":32768,"max_distance_evaluations":1000000,"max_memory_bytes":33554432,"build_policy":"farthest-first-lloyd-f64-spherical-v1","sample_policy":"splitmix64-bottom-hash-rowid-v1","zero_policy":"reject-cosine-zero-skip-ip-zero-training","empty_policy":"retain-prior-empty-or-zero-sum"}`}
}
func validateSchemaCatalog(l Ledger) ([3]int64, bool) {
	var ids [3]int64
	catalog, fields := l.Streams[4].Rows, l.Streams[5].Rows
	if len(catalog) != 3 || len(fields) != 3 || l.SchemaCookie == 0 {
		return ids, false
	}
	configs := ExpectedConfigs()
	tables := [3]string{"docs", "hitems", "iitems"}
	counts := [3]int64{7, 180, 128}
	methods := [3]string{"grams13", "hnsw", "ivfflat"}
	for i, r := range catalog {
		if !rowKinds(r, IntegerValue, TextValue, TextValue, TextValue, TextValue, IntegerValue, IntegerValue, TextValue, TextValue, IntegerValue, IntegerValue, IntegerValue, IntegerValue) || r[0].Int <= 0 || !isText(r[1], IndexNames()[i]) || !isText(r[2], tables[i]) || !isText(r[3], hashText(CreateStatements()[i])) || !isText(r[4], methods[i]) || !isInt(r[5], 1) || !isInt(r[6], 1) || !isText(r[7], configs[i]) || !isText(r[8], hashText(configs[i])) || !isInt(r[9], 1) || !isInt(r[10], 0) || !isInt(r[11], counts[i]) || !isInt(r[12], int64(l.SchemaCookie)) {
			return ids, false
		}
		ids[i] = r[0].Int
		f := fields[i]
		field := "embedding"
		if i == 0 {
			field = "body"
		}
		if !rowKinds(f, IntegerValue, IntegerValue, IntegerValue, TextValue, IntegerValue) || f[0].Int <= 0 || !isInt(f[1], ids[i]) || !isInt(f[2], 1) || !isText(f[3], field) || !isInt(f[4], 1) {
			return ids, false
		}
	}
	specs := expectedSchema()
	for i, id := range ids {
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			specs = append(specs, guardSpec(id, tables[i], op))
		}
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].name < specs[j].name })
	rows := l.Streams[3].Rows
	if len(rows) != len(specs) {
		return ids, false
	}
	roots := map[int64]bool{}
	for i, r := range rows {
		s := specs[i]
		if !rowKinds(r, TextValue, TextValue, TextValue, IntegerValue, TextValue) || !isText(r[0], s.typ) || !isText(r[1], s.name) || !isText(r[2], s.table) || !isText(r[4], s.sql) {
			return ids, false
		}
		if s.typ == "trigger" {
			if r[3].Int != 0 {
				return ids, false
			}
		} else {
			if r[3].Int < 2 || r[3].Int > int64(l.Check.Pages) || roots[r[3].Int] {
				return ids, false
			}
			roots[r[3].Int] = true
		}
	}
	return ids, true
}

type GramKey struct {
	Namespace string `json:"namespace"`
	Length    int64  `json:"length"`
	Text      string `json:"text"`
	RowID     int64  `json:"row_id"`
}

func ExpectedGrams() []GramKey {
	out := []GramKey{}
	for _, r := range ExpectedUserRows()[0] {
		if r[1].Kind != TextValue {
			continue
		}
		s := r[1].Text
		for _, ns := range []string{"LIKE_RAW_V1", "MATCH_CONFIG_V1"} {
			for n := 1; n <= 3; n++ {
				seen := map[string]bool{}
				for i := 0; i+n <= len(s); i++ {
					t := s[i : i+n]
					if !seen[t] {
						out = append(out, GramKey{ns, int64(n), t, r[0].Int})
						seen[t] = true
					}
				}
			}
		}
	}
	return out
}
func validateGrams(l Ledger, index int64) bool {
	postings, frequency := l.Streams[6].Rows, l.Streams[7].Rows
	if len(postings) != 96 || len(frequency) != 68 {
		return false
	}
	expect := map[GramKey]bool{}
	counts := map[GramKey]int64{}
	for _, g := range ExpectedGrams() {
		expect[g] = true
		g.RowID = 0
		counts[g]++
	}
	for _, r := range postings {
		if !rowKinds(r, IntegerValue, IntegerValue, IntegerValue, TextValue, IntegerValue, TextValue, IntegerValue) || !isInt(r[1], index) || !isInt(r[2], 1) {
			return false
		}
		g := GramKey{r[3].Text, r[4].Int, r[5].Text, r[6].Int}
		if !expect[g] {
			return false
		}
		delete(expect, g)
	}
	for _, r := range frequency {
		if !rowKinds(r, IntegerValue, IntegerValue, IntegerValue, TextValue, IntegerValue, TextValue, IntegerValue) || !isInt(r[1], index) || !isInt(r[2], 1) {
			return false
		}
		g := GramKey{r[3].Text, r[4].Int, r[5].Text, 0}
		if counts[g] == 0 || counts[g] != r[6].Int {
			return false
		}
		delete(counts, g)
	}
	return len(expect) == 0 && len(counts) == 0
}

func adjacency(b []byte, id, level int64, levels map[int64]int64) bool {
	if level < 0 || level > 32 || len(b) < 12 || len(b) > 12+4*33+8*(32+32*16) || string(b[:4]) != "GHA\x00" || b[4] != 1 || b[5] != 0 || b[6] != 0 || b[7] != 0 || binary.BigEndian.Uint32(b[8:12]) != uint32(level+1) {
		return false
	}
	pos := 12
	for layer := int64(0); layer <= level; layer++ {
		if len(b)-pos < 4 {
			return false
		}
		n := int(binary.BigEndian.Uint16(b[pos : pos+2]))
		cap := 16
		if layer == 0 {
			cap = 32
		}
		if n > cap || b[pos+2] != 0 || b[pos+3] != 0 {
			return false
		}
		pos += 4
		if n > (len(b)-pos)/8 {
			return false
		}
		var prior int64
		for j := 0; j < n; j++ {
			raw := binary.BigEndian.Uint64(b[pos : pos+8])
			if raw > 180 {
				return false
			}
			target := int64(raw)
			lev, ok := levels[target]
			if !ok || target == id || target <= prior || lev < layer {
				return false
			}
			prior = target
			pos += 8
		}
	}
	return pos == len(b)
}
func validateGraphMembership(l Ledger, ids [3]int64) bool {
	states, nodes, live, centers, members := l.Streams[8].Rows, l.Streams[9].Rows, l.Streams[10].Rows, l.Streams[11].Rows, l.Streams[12].Rows
	if len(states) != 2 || len(nodes) != 180 || len(live) != 180 || len(centers) != 4 || len(members) != 128 {
		return false
	}
	for _, r := range states {
		if !rowKinds(r, IntegerValue, IntegerValue, IntegerValue, IntegerValue, IntegerValue, IntegerValue, IntegerValue, TextValue) {
			return false
		}
	}
	h, i := states[0], states[1]
	if !isInt(h[1], ids[1]) || h[2].Int < 1 || h[2].Int > 180 || h[3].Int < 0 || h[3].Int > 32 || !isInt(h[4], 180) || !isInt(h[5], 0) || !isInt(h[6], 181) || !isText(h[7], "") || !isInt(i[1], ids[2]) || !isInt(i[2], 0) || !isInt(i[3], -1) || !isInt(i[4], 128) || !isInt(i[5], 0) || !isInt(i[6], 1) || !isText(i[7], "ready") {
		return false
	}
	levels := map[int64]int64{}
	source := map[int64]int64{}
	maximum := int64(-1)
	for _, r := range nodes {
		if !rowKinds(r, IntegerValue, IntegerValue, IntegerValue, IntegerValue, IntegerValue, IntegerValue, BlobValue, BlobValue) || !isInt(r[1], ids[1]) || r[2].Int < 1 || r[2].Int > 180 || r[3].Int < 1 || r[3].Int > 180 || !isInt(r[4], 1) || r[5].Int < 0 || r[5].Int > 32 {
			return false
		}
		if _, ok := levels[r[2].Int]; ok {
			return false
		}
		if _, ok := source[r[3].Int]; ok {
			return false
		}
		levels[r[2].Int] = r[5].Int
		source[r[3].Int] = r[2].Int
		if r[5].Int > maximum {
			maximum = r[5].Int
		}
		want, _ := FixtureVector("hitems", r[3].Int)
		if !bytes.Equal(r[6].Blob, Float32Vector(want)) {
			return false
		}
	}
	if maximum != h[3].Int || levels[h[2].Int] != maximum {
		return false
	}
	for _, r := range nodes {
		if !adjacency(r[7].Blob, r[2].Int, r[5].Int, levels) {
			return false
		}
	}
	for _, r := range live {
		if !rowKinds(r, IntegerValue, IntegerValue, IntegerValue, IntegerValue) || !isInt(r[1], ids[1]) || source[r[2].Int] == 0 || source[r[2].Int] != r[3].Int {
			return false
		}
		delete(source, r[2].Int)
	}
	if len(source) != 0 {
		return false
	}
	var centroid [4][3]float32
	var counts [4]int64
	var seenCenters [4]bool
	for _, r := range centers {
		if !rowKinds(r, IntegerValue, IntegerValue, IntegerValue, BlobValue, IntegerValue) || !isInt(r[1], ids[2]) || r[2].Int < 1 || r[2].Int > 4 || r[4].Int < 1 || r[4].Int > 128 {
			return false
		}
		n := int(r[2].Int - 1)
		if seenCenters[n] {
			return false
		}
		v, ok := DecodeFloat32Vector(r[3].Blob)
		if !ok {
			return false
		}
		centroid[n] = v
		seenCenters[n] = true
		counts[n] = r[4].Int
	}
	assigned := map[int64]bool{}
	var actual [4]int64
	for _, r := range members {
		if !rowKinds(r, IntegerValue, IntegerValue, IntegerValue, IntegerValue) || !isInt(r[1], ids[2]) || r[2].Int < 1 || r[2].Int > 4 || r[3].Int < 1 || r[3].Int > 128 || assigned[r[3].Int] {
			return false
		}
		assigned[r[3].Int] = true
		v, _ := FixtureVector("iitems", r[3].Int)
		best := 0
		distance := math.Inf(1)
		for n, c := range centroid {
			var d float64
			for k, x := range v {
				diff := float64(x) - float64(c[k])
				d += diff * diff
			}
			if d < distance {
				distance = d
				best = n
			}
		}
		if r[2].Int != int64(best+1) {
			return false
		}
		actual[best]++
	}
	return actual == counts && len(assigned) == 128
}

// Native result mirrors are data only. Their explicit JSON keys and declaration
// order match the public result structs in sqlite/{plan_grams,similarity_search,
// vector_search}.go. Decoding and exact re-encoding reject missing, duplicate,
// extra, reordered and noncanonical fields; these values do not certify calls.
type NativeQueryPlan struct {
	ANNOpclass           string   `json:"ANNOpclass"`
	ANNGeneration        int64    `json:"ANNGeneration"`
	ANNLists             int      `json:"ANNLists"`
	ANNTrainingState     string   `json:"ANNTrainingState"`
	ANNEFSearch          int      `json:"ANNEFSearch"`
	ANNMaxEFSearch       int      `json:"ANNMaxEFSearch"`
	ANNEffectiveEF       int      `json:"ANNEffectiveEF"`
	ANNProbes            int      `json:"ANNProbes"`
	ANNMaxProbes         int      `json:"ANNMaxProbes"`
	ANNEffectiveProbes   int      `json:"ANNEffectiveProbes"`
	ANNStopReason        string   `json:"ANNStopReason"`
	ANNFallback          string   `json:"ANNFallback"`
	VectorMetric         string   `json:"VectorMetric"`
	VectorBitMetric      string   `json:"VectorBitMetric"`
	VectorRepresentation string   `json:"VectorRepresentation"`
	VectorColumn         string   `json:"VectorColumn"`
	VectorDimensions     int      `json:"VectorDimensions"`
	VectorTopK           int64    `json:"VectorTopK"`
	VectorExact          bool     `json:"VectorExact"`
	VectorRanking        string   `json:"VectorRanking"`
	Access               string   `json:"Access"`
	Table                string   `json:"Table"`
	Indexes              []string `json:"Indexes"`
	Method               string   `json:"Method"`
	MethodVersion        int      `json:"MethodVersion"`
	Fields               []string `json:"Fields"`
	Namespace            string   `json:"Namespace"`
	GramLengths          []int    `json:"GramLengths"`
	EstimatedCandidates  int64    `json:"EstimatedCandidates"`
	Recheck              bool     `json:"Recheck"`
	Reason               string   `json:"Reason"`
}
type NativeQueryStats struct {
	IndexKeysRead          int64 `json:"IndexKeysRead"`
	IndexPageReads         int64 `json:"IndexPageReads"`
	RowIDSeeks             int64 `json:"RowIDSeeks"`
	ANNStateReads          int64 `json:"ANNStateReads"`
	ANNNodeReads           int64 `json:"ANNNodeReads"`
	ANNEdgeReads           int64 `json:"ANNEdgeReads"`
	ANNVisited             int64 `json:"ANNVisited"`
	ANNDistanceEvaluations int64 `json:"ANNDistanceEvaluations"`
	ANNIdentityLookups     int64 `json:"ANNIdentityLookups"`
	ANNCentroidScores      int64 `json:"ANNCentroidScores"`
	ANNCentroidReads       int64 `json:"ANNCentroidReads"`
	ANNListsOpened         int64 `json:"ANNListsOpened"`
	ANNMemberKeysRead      int64 `json:"ANNMemberKeysRead"`
	ANNIterations          int64 `json:"ANNIterations"`
	ANNExactRescores       int64 `json:"ANNExactRescores"`
	ANNPeakBytes           int64 `json:"ANNPeakBytes"`
	VectorRankedRows       int64 `json:"VectorRankedRows"`
	VectorFilteredRows     int64 `json:"VectorFilteredRows"`
	VectorNullRows         int64 `json:"VectorNullRows"`
	VectorRankComponents   int64 `json:"VectorRankComponents"`
	VectorRankNonzeros     int64 `json:"VectorRankNonzeros"`
	VectorRankPackedBytes  int64 `json:"VectorRankPackedBytes"`
	FrequencyLookups       int64 `json:"FrequencyLookups"`
	PostingKeysRead        int64 `json:"PostingKeysRead"`
	Candidates             int64 `json:"Candidates"`
	BaseRowFetches         int64 `json:"BaseRowFetches"`
	ReturnedRows           int64 `json:"ReturnedRows"`
}
type NativeSimilarityPlan struct {
	NativeQueryPlan
	CandidateMode string  `json:"CandidateMode"`
	QueryTokens   int     `json:"QueryTokens"`
	ConfigHash    string  `json:"ConfigHash"`
	TokenizerHash string  `json:"TokenizerHash"`
	Generation    int64   `json:"Generation"`
	MinGram       int     `json:"MinGram"`
	MaxGram       int     `json:"MaxGram"`
	Metric        string  `json:"Metric"`
	Threshold     float64 `json:"Threshold"`
	Limit         int     `json:"Limit"`
	Exact         bool    `json:"Exact"`
}
type NativeSimilarityStats struct {
	NativeQueryStats
	PostingRangesRead int64 `json:"PostingRangesRead"`
	FilteredRows      int64 `json:"FilteredRows"`
	ScoredRows        int64 `json:"ScoredRows"`
	MatchedRows       int64 `json:"MatchedRows"`
}
type NativeVectorPlan struct {
	NativeQueryPlan
	Field      string `json:"Field"`
	Metric     string `json:"Metric"`
	Dimensions int    `json:"Dimensions"`
	Limit      int    `json:"Limit"`
	Exact      bool   `json:"Exact"`
}
type NativeVectorStats struct {
	NativeQueryStats
	FilteredRows     int64 `json:"FilteredRows"`
	ScoredRows       int64 `json:"ScoredRows"`
	NullRows         int64 `json:"NullRows"`
	ScoredComponents int64 `json:"ScoredComponents"`
}

type NativeScore struct {
	Value        float64 `json:"Value"`
	LeftTokens   int     `json:"LeftTokens"`
	RightTokens  int     `json:"RightTokens"`
	Intersection int     `json:"Intersection"`
}
type NativeSimilarityHit struct {
	RowID int64       `json:"RowID"`
	Score NativeScore `json:"Score"`
}
type NativeVectorHit struct {
	RowID    int64   `json:"RowID"`
	Distance float64 `json:"Distance"`
}
type NativeSimilarityResult struct {
	Hits  []NativeSimilarityHit `json:"Hits"`
	Plan  NativeSimilarityPlan  `json:"Plan"`
	Stats NativeSimilarityStats `json:"Stats"`
}
type NativeVectorResult struct {
	Hits  []NativeVectorHit `json:"Hits"`
	Plan  NativeVectorPlan  `json:"Plan"`
	Stats NativeVectorStats `json:"Stats"`
}

func decodeCanonical(b []byte, v any, max int) bool {
	if len(b) == 0 || len(b) > max {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return false
	}
	canonical, err := json.Marshal(v)
	return err == nil && bytes.Equal(canonical, b)
}
func nativeStatsBounded(s NativeQueryStats) bool {
	// All native counters must stay nonnegative and finite within this tiny
	// fixture's admitted logical-work envelope, including unused counters.
	b, err := json.Marshal(s)
	if err != nil {
		return false
	}
	var fields map[string]int64
	if json.Unmarshal(b, &fields) != nil {
		return false
	}
	for _, n := range fields {
		if n < 0 || n > 1<<30 {
			return false
		}
	}
	return true
}
func independentL2(a, b [3]float32) float64 {
	var sum, correction float64
	for i, x := range a {
		d := float64(x) - float64(b[i])
		v := d * d
		t := sum + v
		if math.Abs(sum) >= math.Abs(v) {
			correction += (sum - t) + v
		} else {
			correction += (v - t) + sum
		}
		sum = t
	}
	return math.Sqrt(sum + correction)
}
func ExpectedVectorHits(table string) []NativeVectorHit {
	n, q := 180, [3]float32{20, 6, 1}
	if table == "iitems" {
		n, q = 128, [3]float32{0, 0, 1}
	} else if table != "hitems" {
		return nil
	}
	hits := make([]NativeVectorHit, 0, n)
	for id := 1; id <= n; id++ {
		v, _ := FixtureVector(table, int64(id))
		hits = append(hits, NativeVectorHit{int64(id), independentL2(v, q)})
	}
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].Distance < hits[j].Distance || hits[i].Distance == hits[j].Distance && hits[i].RowID < hits[j].RowID
	})
	return hits[:5]
}
func validateQueries(q []QueryEvidence) bool {
	if len(q) != 6 {
		return false
	}
	var grams [2]NativeSimilarityResult
	var vectors [4]NativeVectorResult
	for n, e := range q {
		if e.ID != QueryIDs()[n] || !e.Executed || e.ErrorToken != 0 {
			return false
		}
		if n < 2 {
			r := &grams[n]
			if !decodeCanonical(e.Result, r, MaxCellBytes) {
				return false
			}
			p, s := r.Plan, r.Stats
			if !p.Exact || p.Table != "docs" || p.Method != "grams13" || p.MethodVersion != 1 || len(p.Indexes) != 1 || p.Indexes[0] != "docs_grams" || p.Generation != 1 || p.QueryTokens != 3 || p.MinGram != 2 || p.MaxGram != 2 || p.Metric != "dice" || p.Threshold != 0.6 || p.Limit != 2 || len(p.ConfigHash) != 64 || len(p.TokenizerHash) != 64 || len(r.Hits) != 2 || !nativeStatsBounded(s.NativeQueryStats) || s.ReturnedRows != 2 || s.FilteredRows != 1 || s.ScoredRows < 2 || s.MatchedRows != 2 {
				return false
			}
			for i, h := range r.Hits {
				if h.RowID != int64(i+1) || h.Score != (NativeScore{Value: float64(2) / 3, LeftTokens: 3, RightTokens: 3, Intersection: 2}) {
					return false
				}
			}
			if n == 0 {
				if p.Access != "SEARCH" || p.CandidateMode != "union-all-query-gram-postings" || s.BaseRowFetches != 3 || s.Candidates != 3 || s.PostingKeysRead != 7 || s.PostingRangesRead != 3 || s.ScoredRows != 2 {
					return false
				}
			} else if p.Access != "SCAN" || p.CandidateMode != "full-scan" || s.BaseRowFetches != 7 || s.PostingKeysRead != 0 || s.PostingRangesRead != 0 {
				return false
			}
		} else {
			r := &vectors[n-2]
			if !decodeCanonical(e.Result, r, MaxCellBytes) {
				return false
			}
			p, s := r.Plan, r.Stats
			table, index, count := "hitems", "hitems_hnsw", int64(180)
			if n >= 4 {
				table, index, count = "iitems", "iitems_ivf", 128
			}
			expected := ExpectedVectorHits(table)
			if p.Table != table || p.Field != "embedding" || p.Dimensions != 3 || p.Metric != "l2" || p.Limit != 5 || len(r.Hits) != 5 || s.ReturnedRows != 5 || !nativeStatsBounded(s.NativeQueryStats) || s.FilteredRows != 0 || s.NullRows != 0 || s.ScoredRows <= 0 || s.ScoredComponents < 3 || p.ANNFallback != "" {
				return false
			}
			for j, h := range r.Hits {
				if h != expected[j] {
					return false
				}
			}
			if n%2 == 1 {
				if !p.Exact || p.Access != "SCAN" || p.Method != "exact_vector" || s.BaseRowFetches != count || s.ScoredRows != count || s.ANNNodeReads != 0 || s.ANNEdgeReads != 0 || s.ANNCentroidReads != 0 || s.ANNMemberKeysRead != 0 {
					return false
				}
			} else {
				if p.Exact || p.Access != "SEARCH" || len(p.Indexes) != 1 || p.Indexes[0] != index || p.ANNOpclass != "vector_l2_ops" || p.ANNGeneration != 1 || s.ANNIdentityLookups != s.BaseRowFetches {
					return false
				}
				if n == 2 {
					if p.Method != "hnsw" || s.ANNNodeReads <= 0 || s.ANNEdgeReads <= 0 || s.ANNDistanceEvaluations <= 0 || s.ANNExactRescores <= 0 || s.BaseRowFetches <= 0 || s.BaseRowFetches >= 180 {
						return false
					}
				} else if p.Method != "ivfflat" || p.ANNLists != 4 || p.ANNProbes != 1 || p.ANNMaxProbes != 4 || p.ANNEffectiveProbes != 1 || p.ANNTrainingState != "ready" || s.ANNCentroidReads != 4 || s.ANNCentroidScores != 4 || s.ANNListsOpened != 1 || s.ANNMemberKeysRead != 32 || s.BaseRowFetches != 32 || s.ANNExactRescores != 32 || s.ANNIdentityLookups != 32 || s.ANNIterations != 1 || s.ANNNodeReads != 0 || s.ANNDistanceEvaluations != 0 {
					return false
				}
			}
		}
	}
	return true
}
func ValidateLedger(l Ledger) Problem {
	if l.Schema != SchemaVersion || l.Fixture != FixtureID || l.Role < Controller || l.Role > Probe || l.Generation == 0 || l.Owner == 0 || len(l.Streams) != 13 || len(l.Checks) != 3 || l.CheckCalls != 1 || !l.CheckReturnedNil || l.CheckErrorToken != 0 {
		return ProblemSQL
	}
	c := l.Check
	if c.Scope != CheckScope || c.Pages == 0 || c.Pages > MaxPages || c.FreePages >= c.Pages || c.ReservedPages != 0 || c.Tables != 12 || c.Indexes != 9 || c.ExtensionIndexes != 3 || c.TableRows != 979 || c.IndexEntries != 789 || c.PageReads == 0 || c.PageReads > 1<<20 {
		return ProblemSQL
	}
	owners := map[uint64]bool{l.Owner: true}
	rows, extent := 0, 0
	for i, s := range l.Streams {
		if s.ID != uint8(i+1) || s.SQL != StreamSQL()[i] || len(s.Args) != 0 || len(s.Columns) != len(StreamColumns(s.ID)) || s.Receipt.Owner == 0 || owners[s.Receipt.Owner] || !s.Receipt.CompleteEOF || !s.Receipt.ErrNil || s.Receipt.CloseCalls != 1 || !s.Receipt.CloseNil || s.Receipt.ErrorToken != 0 {
			return ProblemSQL
		}
		owners[s.Receipt.Owner] = true
		for j, col := range s.Columns {
			if col != StreamColumns(s.ID)[j] {
				return ProblemSQL
			}
		}
		rows += len(s.Rows)
		if rows > MaxResultRows || i == 3 && len(s.Rows) > MaxSchemaEntries {
			return ProblemSQL
		}
		extent += len(s.SQL)
		for j, r := range s.Rows {
			if len(r) != len(s.Columns) {
				return ProblemSQL
			}
			for _, v := range r {
				if !validValue(v) {
					return ProblemSQL
				}
				extent += len(v.Text) + len(v.Blob) + 32
				if extent > MaxLedgerBytes {
					return ProblemSQL
				}
			}
			if i != 3 {
				if r[0].Kind != IntegerValue || i >= 4 && r[0].Int <= 0 || j > 0 && s.Rows[j-1][0].Int >= r[0].Int {
					return ProblemSQL
				}
			}
		}
	}
	expected := ExpectedUserRows()
	for i := range expected {
		if !rowsEqual(l.Streams[i].Rows, expected[i]) {
			return ProblemSQL
		}
	}
	ids, ok := validateSchemaCatalog(l)
	if !ok || !validateGrams(l, ids[0]) || !validateGraphMembership(l, ids) || !validateQueries(l.Queries) {
		return ProblemSQL
	}
	for i, x := range l.Checks {
		if x.Name != IndexNames()[i] || x.Calls != 1 || !x.ReturnedNil || x.ErrorToken != 0 {
			return ProblemSQL
		}
	}
	return NoProblem
}
func EncodeLedger(l Ledger) ([]byte, Problem) {
	if ValidateLedger(l) != NoProblem {
		return nil, ProblemSQL
	}
	b, err := json.Marshal(l)
	if err != nil || len(b) > MaxLedgerBytes {
		return nil, ProblemSQL
	}
	return b, NoProblem
}
func DecodeLedger(b []byte) (Ledger, Problem) {
	var l Ledger
	if !decodeCanonical(b, &l, MaxLedgerBytes) || ValidateLedger(l) != NoProblem {
		return Ledger{}, ProblemSQL
	}
	return l, NoProblem
}

// LedgerDigest uses a domain-separated, explicit-length value encoding. Every
// stream ID, SQL/arg, column/row count, storage-class tag, cell byte length and
// complete cell bytes is retained. Query results include their full canonical
// native hits, plans and stats. Only role/owner bookkeeping and the permitted
// adapter-specific Check.PageReads counter are excluded from this C identity.
func appendU64(dst []byte, n uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	return append(dst, b[:]...)
}
func appendBytes(dst, b []byte) []byte {
	dst = appendU64(dst, uint64(len(b)))
	return append(dst, b...)
}
func appendCell(dst []byte, v Value) []byte {
	switch v.Kind {
	case NullValue:
		dst = append(dst, 0)
		return appendU64(dst, 0)
	case IntegerValue:
		dst = append(dst, 1)
		dst = appendU64(dst, 8)
		return appendU64(dst, uint64(v.Int))
	case RealValue:
		dst = append(dst, 2)
		dst = appendU64(dst, 8)
		return appendU64(dst, v.RealBits)
	case TextValue:
		dst = append(dst, 3)
		return appendBytes(dst, []byte(v.Text))
	case BlobValue:
		dst = append(dst, 4)
		return appendBytes(dst, v.Blob)
	}
	return nil
}
func LedgerDigest(l Ledger) (Hash, Problem) {
	if ValidateLedger(l) != NoProblem {
		return Hash{}, ProblemSQL
	}
	b := []byte("gosqlite/checkpoint-restart/complete-ledger/v1\x00")
	b = appendU64(b, uint64(l.Schema))
	b = appendBytes(b, []byte(l.Fixture))
	b = appendU64(b, uint64(l.SchemaCookie))
	b = appendU64(b, uint64(len(l.Streams)))
	for _, s := range l.Streams {
		b = appendU64(b, uint64(s.ID))
		b = appendBytes(b, []byte(s.SQL))
		b = appendU64(b, uint64(len(s.Args)))
		for _, a := range s.Args {
			b = appendCell(b, a)
		}
		b = appendU64(b, uint64(len(s.Columns)))
		for _, column := range s.Columns {
			b = appendBytes(b, []byte(column))
		}
		b = appendU64(b, uint64(len(s.Rows)))
		for _, row := range s.Rows {
			b = appendU64(b, uint64(len(row)))
			for _, v := range row {
				b = appendCell(b, v)
			}
		}
		if len(b) > MaxLedgerBytes {
			return Hash{}, ProblemSQL
		}
	}
	b = appendU64(b, uint64(len(l.Queries)))
	for _, q := range l.Queries {
		b = appendBytes(b, []byte(q.ID))
		b = appendBytes(b, q.Result)
	}
	b = appendU64(b, uint64(len(l.Checks)))
	for _, c := range l.Checks {
		b = appendBytes(b, []byte(c.Name))
	}
	c := l.Check
	b = appendBytes(b, []byte(c.Scope))
	for _, n := range []uint64{uint64(c.Pages), uint64(c.FreePages), uint64(c.ReservedPages), uint64(c.Tables), uint64(c.Indexes), uint64(c.ExtensionIndexes), c.TableRows, c.IndexEntries} {
		b = appendU64(b, n)
	}
	if len(b) > MaxLedgerBytes {
		return Hash{}, ProblemSQL
	}
	return sha256.Sum256(b), NoProblem
}
func CompareLedgers(a, b Ledger) Problem {
	x, p := LedgerDigest(a)
	if p != NoProblem {
		return p
	}
	y, p := LedgerDigest(b)
	if p != NoProblem || x != y {
		return ProblemSQL
	}
	return NoProblem
}
