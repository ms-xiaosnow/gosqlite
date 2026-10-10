// Package postgres inspects offline, versioned PostgreSQL catalog exports and
// converts finite real[] values without connecting to a PostgreSQL server.
// Catalog facts are supplied evidence, not executable SQL or proof of their
// authenticity. Unsupported identities and incomplete inventories fail closed.
package postgres

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

const CatalogVersion = 1
const CollectorVersion = "gosqlite-pgembedding-catalog-v1"

// Limits can only lower the bounded default profile. A zero value selects it.
type Limits struct{ MaxBytes, MaxObjects, MaxDependencies, MaxDepth, MaxTokens int }

func DefaultLimits() Limits { return Limits{16 << 20, 10000, 100000, 32, 1 << 20} }
func (l Limits) normalized() (Limits, error) {
	d := DefaultLimits()
	if l == (Limits{}) {
		return d, nil
	}
	if l.MaxBytes < 1 || l.MaxBytes > d.MaxBytes || l.MaxObjects < 1 || l.MaxObjects > d.MaxObjects || l.MaxDependencies < 1 || l.MaxDependencies > d.MaxDependencies || l.MaxDepth < 1 || l.MaxDepth > d.MaxDepth || l.MaxTokens < 1 || l.MaxTokens > d.MaxTokens {
		return l, fmt.Errorf("catalog: invalid resource limits")
	}
	return l, nil
}

type CatalogSnapshot struct {
	FormatVersion    int            `json:"format_version"`
	Collector        string         `json:"collector"`
	ExtractedAt      string         `json:"extracted_at"`
	ServerVersionNum int            `json:"server_version_num"`
	Scope            Scope          `json:"scope"`
	Extensions       []Extension    `json:"extensions"`
	Types            []Type         `json:"types"`
	AccessMethods    []AccessMethod `json:"access_methods"`
	Functions        []Function     `json:"functions"`
	Operators        []Operator     `json:"operators"`
	OpClasses        []OpClass      `json:"opclasses"`
	Relations        []Relation     `json:"relations"`
	Indexes          []Index        `json:"indexes"`
	Dependencies     []Dependency   `json:"dependencies"`
	Profiles         []DataProfile  `json:"profiles"`
	SQLCorpus        []SQLSource    `json:"sql_corpus"`
	sourceSHA256     string
	semanticSHA256   string
}
type Scope struct {
	Schemas  []string   `json:"schemas"`
	Coverage []Coverage `json:"coverage"`
}
type Coverage struct {
	Category string `json:"category"`
	Complete bool   `json:"complete"`
	Reason   string `json:"reason"`
}
type Extension struct {
	OID     uint32 `json:"oid"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Schema  string `json:"schema"`
}
type Type struct {
	OID        uint32 `json:"oid"`
	Schema     string `json:"schema"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	ElementOID uint32 `json:"element_oid"`
	BaseOID    uint32 `json:"base_oid"`
}
type AccessMethod struct {
	OID          uint32 `json:"oid"`
	Name         string `json:"name"`
	ExtensionOID uint32 `json:"extension_oid"`
	HandlerOID   uint32 `json:"handler_oid"`
}
type Function struct {
	OID          uint32   `json:"oid"`
	Schema       string   `json:"schema"`
	Name         string   `json:"name"`
	ArgumentOIDs []uint32 `json:"argument_oids"`
	ReturnOID    uint32   `json:"return_oid"`
	ExtensionOID uint32   `json:"extension_oid"`
}
type Operator struct {
	OID          uint32 `json:"oid"`
	Schema       string `json:"schema"`
	Name         string `json:"name"`
	LeftOID      uint32 `json:"left_oid"`
	RightOID     uint32 `json:"right_oid"`
	FunctionOID  uint32 `json:"function_oid"`
	ExtensionOID uint32 `json:"extension_oid"`
}
type OpClass struct {
	OID          uint32 `json:"oid"`
	Schema       string `json:"schema"`
	Name         string `json:"name"`
	MethodOID    uint32 `json:"method_oid"`
	InputOID     uint32 `json:"input_oid"`
	Default      bool   `json:"default"`
	ExtensionOID uint32 `json:"extension_oid"`
	OperatorOID  uint32 `json:"operator_oid"`
	FunctionOID  uint32 `json:"function_oid"`
}
type Column struct {
	AttNum          int    `json:"attnum"`
	Name            string `json:"name"`
	TypeOID         uint32 `json:"type_oid"`
	TypeMod         int    `json:"typmod"`
	ArrayDimensions int    `json:"array_dimensions"`
	NotNull         bool   `json:"not_null"`
	Default         string `json:"default"`
	Generated       string `json:"generated"`
	Identity        string `json:"identity"`
	Dropped         bool   `json:"dropped"`
}
type Relation struct {
	OID         uint32   `json:"oid"`
	Schema      string   `json:"schema"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Persistence string   `json:"persistence"`
	RLS         bool     `json:"rls"`
	Parents     []uint32 `json:"parents"`
	PrimaryKey  []int    `json:"primary_key"`
	Columns     []Column `json:"columns"`
}
type Index struct {
	OID        uint32   `json:"oid"`
	Schema     string   `json:"schema"`
	Name       string   `json:"name"`
	TableOID   uint32   `json:"table_oid"`
	MethodOID  uint32   `json:"method_oid"`
	Keys       []int    `json:"keys"`
	OpClasses  []uint32 `json:"opclasses"`
	Collations []uint32 `json:"collations"`
	Options    []string `json:"options"`
	Expression string   `json:"expression"`
	Predicate  string   `json:"predicate"`
	Valid      bool     `json:"valid"`
	Ready      bool     `json:"ready"`
	Definition string   `json:"definition"`
}
type Dependency struct {
	Class           string `json:"class"`
	OID             uint32 `json:"oid"`
	SubID           int    `json:"sub_id"`
	Identity        string `json:"identity"`
	ReferencedClass string `json:"referenced_class"`
	ReferencedOID   uint32 `json:"referenced_oid"`
	ReferencedSubID int    `json:"referenced_sub_id"`
	Kind            string `json:"kind"`
	Definition      string `json:"definition"`
}
type Shape struct {
	Dimensions  []int `json:"dimensions"`
	LowerBounds []int `json:"lower_bounds"`
	Rows        int64 `json:"rows"`
}
type DataProfile struct {
	RelationOID       uint32   `json:"relation_oid"`
	AttNum            int      `json:"attnum"`
	Exhaustive        bool     `json:"exhaustive"`
	Rows              int64    `json:"rows"`
	NullRows          int64    `json:"null_rows"`
	NullElementRows   int64    `json:"null_element_rows"`
	NonFiniteRows     int64    `json:"nonfinite_rows"`
	ZeroNormRows      int64    `json:"zero_norm_rows"`
	Shapes            []Shape  `json:"shapes"`
	MinNorm           *float64 `json:"min_norm"`
	MaxNorm           *float64 `json:"max_norm"`
	ExportID          string   `json:"export_id"`
	PrimaryKeyUnique  bool     `json:"primary_key_unique"`
	PrimaryKeyOrdered bool     `json:"primary_key_ordered"`
}
type SQLSource struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
}

// DecodeCatalog checks duplicate/exact keys, required presence, scalar types,
// depth/tokens/bytes, identities and cross references. Errors omit data values.
func DecodeCatalog(r io.Reader, limits Limits) (CatalogSnapshot, error) {
	var c CatalogSnapshot
	l, err := limits.normalized()
	if err != nil {
		return c, err
	}
	if r == nil {
		return c, fmt.Errorf("catalog: nil reader")
	}
	b, err := io.ReadAll(io.LimitReader(r, int64(l.MaxBytes)+1))
	if err != nil {
		return c, err
	}
	if len(b) > l.MaxBytes {
		return c, fmt.Errorf("catalog: byte limit exceeded")
	}
	if err = strictJSON(b, &c, l); err != nil {
		return CatalogSnapshot{}, err
	}
	if err = validateCatalog(c, l); err != nil {
		return CatalogSnapshot{}, err
	}
	h := sha256.Sum256(b)
	c.sourceSHA256 = hex.EncodeToString(h[:])
	canonical, _ := json.Marshal(c)
	semantic := sha256.Sum256(canonical)
	c.semanticSHA256 = hex.EncodeToString(semantic[:])
	return c, nil
}

func strictJSON(b []byte, dst any, l Limits) error {
	if err := checkJSONUnicodeEscapes(b); err != nil {
		return err
	}
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return fmt.Errorf("json: invalid UTF-8 or NUL")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	count := 0
	var walk func(int) error
	walk = func(depth int) error {
		if depth > l.MaxDepth {
			return fmt.Errorf("json: nesting limit exceeded")
		}
		count++
		if count > l.MaxTokens {
			return fmt.Errorf("json: token limit exceeded")
		}
		t, err := d.Token()
		if err != nil {
			return fmt.Errorf("json: invalid token at byte %d", d.InputOffset())
		}
		if s, ok := t.(string); ok {
			if len(s) > 1<<20 || strings.IndexByte(s, 0) >= 0 {
				return fmt.Errorf("json: invalid string length or NUL")
			}
			return nil
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return fmt.Errorf("json: invalid object key")
				}
				s, ok := k.(string)
				if !ok || keys[s] {
					return fmt.Errorf("json: duplicate or invalid object key at byte %d", d.InputOffset())
				}
				if len(s) > 256 || strings.IndexByte(s, 0) >= 0 {
					return fmt.Errorf("json: invalid object key")
				}
				keys[s] = true
				count++
				if count > l.MaxTokens {
					return fmt.Errorf("json: token limit exceeded")
				}
				if err = walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err = walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("json: unexpected delimiter")
		}
		_, err = d.Token()
		if err != nil {
			return fmt.Errorf("json: unclosed container")
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("json: trailing value or malformed suffix")
	}
	if err := checkJSONShape(b, reflect.TypeOf(dst).Elem(), 0, l.MaxDepth); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("json: value does not match schema")
	}
	return nil
}
func checkJSONShape(b []byte, t reflect.Type, depth, maxDepth int) error {
	if depth > maxDepth {
		return fmt.Errorf("json: schema depth exceeded")
	}
	if t.Kind() == reflect.Pointer {
		if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
			return nil
		}
		return checkJSONShape(b, t.Elem(), depth+1, maxDepth)
	}
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return fmt.Errorf("json: null is not valid for required value")
	}
	switch t.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(b, &obj); err != nil {
			return fmt.Errorf("json: expected object")
		}
		fields := map[string]reflect.StructField{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")
			if tag[0] == "-" {
				continue
			}
			name := tag[0]
			if name == "" {
				name = f.Name
			}
			fields[name] = f
			v, ok := obj[name]
			optional := len(tag) > 1 && tag[1] == "omitempty"
			if !ok && !optional {
				return fmt.Errorf("json: missing required field %s", name)
			}
			if ok {
				if err := checkJSONShape(v, f.Type, depth+1, maxDepth); err != nil {
					return err
				}
			}
		}
		for k := range obj {
			if _, ok := fields[k]; !ok {
				return fmt.Errorf("json: unknown field")
			}
		}
	case reflect.Slice:
		var a []json.RawMessage
		if err := json.Unmarshal(b, &a); err != nil {
			return fmt.Errorf("json: expected array")
		}
		for _, v := range a {
			if err := checkJSONShape(v, t.Elem(), depth+1, maxDepth); err != nil {
				return err
			}
		}
	}
	return nil
}

func validName(s string) bool {
	return len(s) > 0 && len(s) <= 63 && utf8.ValidString(s) && strings.IndexByte(s, 0) < 0
}
func objectKey(schema, name string) string { return quoteIdent(schema) + "." + quoteIdent(name) }
func quoteIdent(s string) string           { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func validateCatalog(c CatalogSnapshot, l Limits) error {
	if err := budgetCatalog(c, l); err != nil {
		return err
	}
	fail := func(s string) error { return fmt.Errorf("catalog: %s", s) }
	if c.FormatVersion != CatalogVersion || c.Collector != CollectorVersion {
		return fail("unsupported format or collector version")
	}
	tm, err := time.Parse(time.RFC3339Nano, c.ExtractedAt)
	if err != nil || tm.Location() != time.UTC || !strings.HasSuffix(c.ExtractedAt, "Z") {
		return fail("extraction timestamp must be UTC RFC3339")
	}
	if c.ServerVersionNum < 90000 || c.ServerVersionNum > 999999 {
		return fail("invalid server version")
	}
	total := len(c.Extensions) + len(c.Types) + len(c.AccessMethods) + len(c.Functions) + len(c.Operators) + len(c.OpClasses) + len(c.Relations) + len(c.Indexes) + len(c.Profiles) + len(c.SQLCorpus)
	for _, r := range c.Relations {
		total += len(r.Columns)
	}
	if total > l.MaxObjects || len(c.Dependencies) > l.MaxDependencies {
		return fail("object limit exceeded")
	}
	schemaSet := map[string]bool{}
	for _, s := range c.Scope.Schemas {
		if !validName(s) || schemaSet[s] {
			return fail("invalid or duplicate scope schema")
		}
		schemaSet[s] = true
	}
	if len(schemaSet) == 0 {
		return fail("empty schema scope")
	}
	cov := map[string]bool{}
	for _, v := range c.Scope.Coverage {
		if !validName(v.Category) || cov[v.Category] || (!v.Complete && v.Reason == "") {
			return fail("invalid coverage record")
		}
		cov[v.Category] = true
	}
	ext := map[uint32]Extension{}
	names := map[string]bool{}
	for _, x := range c.Extensions {
		if x.OID == 0 || !validName(x.Name) || !validName(x.Schema) || x.Version == "" || len(x.Version) > 63 || ext[x.OID].OID != 0 || names[x.Name] {
			return fail("invalid or duplicate extension")
		}
		ext[x.OID] = x
		names[x.Name] = true
	}
	ty := map[uint32]Type{}
	names = map[string]bool{}
	for _, x := range c.Types {
		k := objectKey(x.Schema, x.Name)
		if x.OID == 0 || !validName(x.Name) || !validName(x.Schema) || len(x.Kind) != 1 || ty[x.OID].OID != 0 || names[k] {
			return fail("invalid or duplicate type")
		}
		ty[x.OID] = x
		names[k] = true
	}
	for _, x := range c.Types {
		if x.ElementOID != 0 && ty[x.ElementOID].OID == 0 || x.BaseOID != 0 && ty[x.BaseOID].OID == 0 {
			return fail("unresolved type reference")
		}
	}
	owner := func(id uint32) bool { return id == 0 || ext[id].OID != 0 }
	fn := map[uint32]Function{}
	names = map[string]bool{}
	for _, x := range c.Functions {
		k := fmt.Sprintf("%s/%v", objectKey(x.Schema, x.Name), x.ArgumentOIDs)
		if x.OID == 0 || !validName(x.Name) || !validName(x.Schema) || !owner(x.ExtensionOID) || fn[x.OID].OID != 0 || names[k] || len(x.ArgumentOIDs) > 100 || ty[x.ReturnOID].OID == 0 {
			return fail("invalid or duplicate function")
		}
		for _, id := range x.ArgumentOIDs {
			if ty[id].OID == 0 {
				return fail("unresolved function argument type")
			}
		}
		fn[x.OID] = x
		names[k] = true
	}
	am := map[uint32]AccessMethod{}
	names = map[string]bool{}
	for _, x := range c.AccessMethods {
		if x.OID == 0 || !validName(x.Name) || !owner(x.ExtensionOID) || am[x.OID].OID != 0 || names[x.Name] || fn[x.HandlerOID].OID == 0 {
			return fail("invalid or duplicate access method")
		}
		am[x.OID] = x
		names[x.Name] = true
	}
	op := map[uint32]Operator{}
	names = map[string]bool{}
	for _, x := range c.Operators {
		k := fmt.Sprintf("%s/%d/%d", objectKey(x.Schema, x.Name), x.LeftOID, x.RightOID)
		if x.OID == 0 || !validName(x.Name) || !validName(x.Schema) || !owner(x.ExtensionOID) || op[x.OID].OID != 0 || names[k] || ty[x.LeftOID].OID == 0 || ty[x.RightOID].OID == 0 || fn[x.FunctionOID].OID == 0 {
			return fail("invalid or duplicate operator")
		}
		op[x.OID] = x
		names[k] = true
	}
	oc := map[uint32]OpClass{}
	names = map[string]bool{}
	for _, x := range c.OpClasses {
		k := fmt.Sprintf("%s/%d", objectKey(x.Schema, x.Name), x.MethodOID)
		if x.OID == 0 || !validName(x.Name) || !validName(x.Schema) || !owner(x.ExtensionOID) || oc[x.OID].OID != 0 || names[k] || am[x.MethodOID].OID == 0 || ty[x.InputOID].OID == 0 || op[x.OperatorOID].OID == 0 || fn[x.FunctionOID].OID == 0 {
			return fail("invalid or duplicate opclass")
		}
		oc[x.OID] = x
		names[k] = true
	}
	rel := map[uint32]Relation{}
	names = map[string]bool{}
	attrs := map[uint32]map[int]Column{}
	for _, x := range c.Relations {
		k := objectKey(x.Schema, x.Name)
		if x.OID == 0 || !validName(x.Name) || !validName(x.Schema) || !schemaSet[x.Schema] || len(x.Kind) != 1 || len(x.Persistence) != 1 || rel[x.OID].OID != 0 || names[k] {
			return fail("invalid or duplicate relation")
		}
		cs := map[int]Column{}
		cn := map[string]bool{}
		for _, a := range x.Columns {
			if a.AttNum < 1 || a.AttNum > 1600 || !validName(a.Name) || cs[a.AttNum].AttNum != 0 || cn[a.Name] || a.ArrayDimensions < 0 || a.ArrayDimensions > 6 || (!a.Dropped && ty[a.TypeOID].OID == 0) {
				return fail("invalid or duplicate column")
			}
			cs[a.AttNum] = a
			cn[a.Name] = true
		}
		seen := map[int]bool{}
		for _, a := range x.PrimaryKey {
			if cs[a].AttNum == 0 || cs[a].Dropped || seen[a] {
				return fail("invalid primary key reference")
			}
			seen[a] = true
		}
		rel[x.OID] = x
		attrs[x.OID] = cs
		names[k] = true
	}
	indexIDs := map[uint32]bool{}
	for _, x := range c.Indexes {
		k := objectKey(x.Schema, x.Name)
		if x.OID == 0 || !validName(x.Name) || !validName(x.Schema) || rel[x.OID].OID != 0 || indexIDs[x.OID] || names[k] || rel[x.TableOID].OID == 0 || am[x.MethodOID].OID == 0 || len(x.Keys) == 0 || len(x.Keys) != len(x.OpClasses) || len(x.Keys) != len(x.Collations) || len(x.Keys) > 32 {
			return fail("invalid or duplicate index")
		}
		for i, a := range x.Keys {
			if a < 0 || a != 0 && attrs[x.TableOID][a].AttNum == 0 || oc[x.OpClasses[i]].OID == 0 {
				return fail("unresolved index key or opclass")
			}
		}
		indexIDs[x.OID] = true
		names[k] = true
	}
	seenProfile := map[string]bool{}
	for _, p := range c.Profiles {
		k := fmt.Sprintf("%d/%d", p.RelationOID, p.AttNum)
		if seenProfile[k] || attrs[p.RelationOID][p.AttNum].AttNum == 0 || p.Rows < 0 || p.NullRows < 0 || p.NullRows > p.Rows || p.NullElementRows < 0 || p.NullElementRows > p.Rows-p.NullRows || p.NonFiniteRows < 0 || p.NonFiniteRows > p.Rows-p.NullRows || p.ZeroNormRows < 0 || p.ZeroNormRows > p.Rows-p.NullRows || len(p.ExportID) > 256 {
			return fail("invalid data profile")
		}
		seenProfile[k] = true
		sum := int64(0)
		shapes := map[string]bool{}
		for _, s := range p.Shapes {
			key := fmt.Sprint(s.Dimensions, s.LowerBounds)
			if shapes[key] || len(s.Dimensions) != len(s.LowerBounds) || len(s.Dimensions) > 6 || s.Rows < 0 || s.Rows > p.Rows-p.NullRows-sum {
				return fail("invalid shape histogram")
			}
			for _, dim := range s.Dimensions {
				if dim < 0 {
					return fail("negative array dimension")
				}
			}
			shapes[key] = true
			sum += s.Rows
		}
		if sum != p.Rows-p.NullRows {
			return fail("incomplete shape histogram")
		}
		if p.MinNorm != nil && (math.IsNaN(*p.MinNorm) || math.IsInf(*p.MinNorm, 0) || *p.MinNorm < 0) || p.MaxNorm != nil && (math.IsNaN(*p.MaxNorm) || math.IsInf(*p.MaxNorm, 0) || *p.MaxNorm < 0) || p.MinNorm != nil && p.MaxNorm != nil && *p.MinNorm > *p.MaxNorm {
			return fail("invalid norm range")
		}
	}
	depSeen := map[string]bool{}
	for _, d := range c.Dependencies {
		k := fmt.Sprintf("%s/%d/%d/%s/%d/%d/%s", d.Class, d.OID, d.SubID, d.ReferencedClass, d.ReferencedOID, d.ReferencedSubID, d.Kind)
		if d.OID == 0 || d.ReferencedOID == 0 || d.SubID < 0 || d.ReferencedSubID < 0 || !validName(d.Class) || !validName(d.ReferencedClass) || !validName(d.Kind) || d.Identity == "" || depSeen[k] {
			return fail("invalid or duplicate dependency")
		}
		depSeen[k] = true
	}
	sqlNames := map[string]bool{}
	for _, s := range c.SQLCorpus {
		if s.Name == "" || len(s.Name) > 256 || len(s.SQL) > 1<<20 || sqlNames[s.Name] {
			return fail("invalid SQL corpus record")
		}
		sqlNames[s.Name] = true
	}
	return nil
}
