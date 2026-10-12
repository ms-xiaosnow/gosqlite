package backupmanual

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// These ordered, local values are data for the portable decoder only. They are
// never written, launched, or passed to the tagged manual qualification helper.
type schemaTestMember struct {
	key string
	value any
}
type schemaTestObject []schemaTestMember
type schemaTestLocation struct {
	name string
	path []string
	keys []string
}

func (object schemaTestObject) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	for index, member := range object {
		if index != 0 { out.WriteByte(',') }
		key, err := json.Marshal(member.key)
		if err != nil { return nil, err }
		// Only schemaTestEscapeKey constructs this fixed, ASCII escape form.
		if strings.HasPrefix(member.key, `\u`) { key = []byte(`"` + member.key + `"`) }
		value, err := json.Marshal(member.value)
		if err != nil { return nil, err }
		out.Write(key)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

func schemaTestPin(path string) schemaTestObject {
	return schemaTestObject{{"path", path}, {"size", 1}, {"sha256", strings.Repeat("0", 64)}}
}

func schemaTestFixture(withOracle bool) schemaTestObject {
	root := "/tmp/manifest-schema-memory-only"
	selector := "^TestPlaintextBackupOrdinaryRoundTrip$"
	tags := "manual_backup_publication"
	runtimeEnv := schemaTestObject{
		{ackKey, "1"}, {rootKey, root}, {"GOTMPDIR", root + "/tmp"}, {"TMPDIR", root + "/tmp"},
		{"LC_ALL", "C"}, {"LANG", "C"}, {"TZ", "UTC"}, {"GOMAXPROCS", "2"},
	}
	var oracle any
	if withOracle {
		selector = "^TestPlaintextBackupOracleOrdinaryInteroperability$"
		tags = "oracle,manual_backup_publication"
		oracle = schemaTestObject{
			{"binary", schemaTestPin("/synthetic/sqlite3")}, {"version", "3.54.0"},
			{"source_id", "2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3"},
		}
		runtimeEnv = append(runtimeEnv, schemaTestMember{"SQLITE_ORACLE", "/synthetic/sqlite3"})
	}
	buildEnv := schemaTestObject{
		{"PATH", "/synthetic/go/bin"}, {"GOROOT", "/synthetic/go"}, {"GOPATH", "/synthetic/gopath"},
		{"GOMODCACHE", "/synthetic/modules"}, {"GOCACHE", "/synthetic/cache"},
		{"GOTMPDIR", root + "/tmp"}, {"TMPDIR", root + "/tmp"}, {"CGO_ENABLED", "0"},
		{"GOTOOLCHAIN", "local"}, {"GOENV", "off"}, {"GOWORK", "off"}, {"GOFLAGS", ""},
		{"GOPROXY", "off"}, {"GOSUMDB", "off"}, {"GOOS", "linux"}, {"GOARCH", "amd64"},
		{"GOMAXPROCS", "2"}, {"GOAMD64", "v1"},
	}
	modules := []any{}
	for _, item := range []struct{ path, version string }{
		{"golang.org/x/crypto", "v0.58.0"}, {"golang.org/x/sys", "v0.49.0"}, {"golang.org/x/text", "v0.43.0"},
	} {
		modules = append(modules, schemaTestObject{{"path", item.path}, {"version", item.version}, {"sum", "h1:" + strings.Repeat("A", 43) + "="}})
	}
	return schemaTestObject{
		{"schema", 1}, {"root", root}, {"profile", "linux-ext-family-unique-mount-openat2-noreplace-v1"},
		{"namespace", "stable-private-disposable-no-replacement-no-remount-compatible-locks-storage"},
		{"source_tree", strings.Repeat("0", 40)},
		{"source_inventory", schemaTestPin("source-inventory.json")},
		{"toolchain_inventory", schemaTestPin("toolchain-inventory.json")},
		{"module_inventory", schemaTestPin("module-inventory.json")},
		{"go_mod", schemaTestPin("go.mod")}, {"go_sum", schemaTestPin("go.sum")},
		{"go_version", "go1.27.2"}, {"goarch", "amd64"}, {"modules", modules}, {"build_tags", tags},
		{"build_argv", []any{"/synthetic/go/bin/go", "test", "-c", "-mod=readonly", "-buildvcs=false", "-p=1", "-tags", tags, "-o", "/synthetic/manual.test", "./sqlite"}},
		{"build_env", buildEnv},
		{"runtime_argv", []any{"/synthetic/manual.test", "-test.run", selector, "-test.v", "-test.count=1", "-test.failfast", "-test.parallel=1", "-test.shuffle=off", "-test.timeout=0"}},
		{"runtime_env", runtimeEnv}, {"selector", selector}, {"executable", schemaTestPin("/synthetic/manual.test")}, {"oracle", oracle},
	}
}

func schemaTestLocations() []schemaTestLocation {
	locations := []schemaTestLocation{{"manifest", nil, []string{
		"schema", "root", "profile", "namespace", "source_tree", "source_inventory", "toolchain_inventory", "module_inventory", "go_mod", "go_sum",
		"go_version", "goarch", "modules", "build_tags", "build_argv", "build_env", "runtime_argv", "runtime_env", "selector", "executable", "oracle",
	}}}
	for _, name := range []string{"source_inventory", "toolchain_inventory", "module_inventory", "go_mod", "go_sum", "executable"} {
		locations = append(locations, schemaTestLocation{name, []string{name}, []string{"path", "size", "sha256"}})
	}
	for index := 0; index < 3; index++ {
		name := "modules_" + strconv.Itoa(index)
		locations = append(locations, schemaTestLocation{name, []string{"modules", strconv.Itoa(index)}, []string{"path", "version", "sum"}})
	}
	locations = append(locations,
		schemaTestLocation{"oracle", []string{"oracle"}, []string{"binary", "version", "source_id"}},
		schemaTestLocation{"oracle_binary", []string{"oracle", "binary"}, []string{"path", "size", "sha256"}},
		schemaTestLocation{"build_env", []string{"build_env"}, []string{"PATH", "GOROOT", "GOPATH", "GOMODCACHE", "GOCACHE", "GOTMPDIR", "TMPDIR", "CGO_ENABLED", "GOTOOLCHAIN", "GOENV", "GOWORK", "GOFLAGS", "GOPROXY", "GOSUMDB", "GOOS", "GOARCH", "GOMAXPROCS", "GOAMD64"}},
		schemaTestLocation{"runtime_env", []string{"runtime_env"}, []string{ackKey, rootKey, "GOTMPDIR", "TMPDIR", "LC_ALL", "LANG", "TZ", "GOMAXPROCS", "SQLITE_ORACLE"}},
	)
	return locations
}

func schemaTestBytes(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil { t.Fatalf("marshal bounded fixture: %v", err) }
	return raw
}

func schemaTestMemberIndex(t *testing.T, object schemaTestObject, key string) int {
	t.Helper()
	for index, member := range object { if member.key == key { return index } }
	t.Fatalf("fixture member %q missing", key)
	return 0
}

func schemaTestEdit(t *testing.T, value any, path []string, edit func(schemaTestObject) any) any {
	t.Helper()
	if len(path) == 0 {
		object, ok := value.(schemaTestObject)
		if !ok { t.Fatalf("fixture edit target is %T, not an object", value) }
		return edit(object)
	}
	switch current := value.(type) {
	case schemaTestObject:
		index := schemaTestMemberIndex(t, current, path[0])
		current[index].value = schemaTestEdit(t, current[index].value, path[1:], edit)
		return current
	case []any:
		index, err := strconv.Atoi(path[0])
		if err != nil || index < 0 || index >= len(current) { t.Fatalf("fixture array index %q", path[0]) }
		current[index] = schemaTestEdit(t, current[index], path[1:], edit)
		return current
	default:
		t.Fatalf("fixture path enters %T", value)
		return nil
	}
}

func schemaTestSet(t *testing.T, value any, path []string, key string, replacement any) any {
	t.Helper()
	return schemaTestEdit(t, value, path, func(object schemaTestObject) any {
		object[schemaTestMemberIndex(t, object, key)].value = replacement
		return object
	})
}

func schemaTestEscapeKey(key string) string {
	return fmt.Sprintf(`\u%04x%s`, key[0], key[1:])
}

func schemaTestReorder(value any, escape bool) any {
	switch current := value.(type) {
	case schemaTestObject:
		result := make(schemaTestObject, len(current))
		for index, member := range current {
			key := member.key
			if escape { key = schemaTestEscapeKey(key) }
			result[len(current)-1-index] = schemaTestMember{key, schemaTestReorder(member.value, escape)}
		}
		return result
	case []any:
		result := make([]any, len(current))
		for index, member := range current { result[index] = schemaTestReorder(member, escape) }
		return result
	default:
		return value
	}
}

func schemaTestDifferent(value any) any {
	switch current := value.(type) {
	case string:
		return current + "-different"
	case int:
		return current + 1
	case schemaTestObject:
		result := append(schemaTestObject(nil), current...)
		result[0].value = schemaTestDifferent(result[0].value)
		return result
	case []any:
		result := append([]any(nil), current...)
		result[0] = schemaTestDifferent(result[0])
		return result
	default:
		return "different"
	}
}

func schemaTestKind(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case schemaTestObject:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case int, json.Number:
		return "number"
	case bool:
		return "boolean"
	default:
		return "invalid-fixture-kind"
	}
}

func schemaTestAccept(t *testing.T, raw []byte) {
	t.Helper()
	decoded, err := decodeManifest(raw)
	if err != nil { t.Fatalf("canonical schema rejected: %v", err) }
	// Compare every decoded member, including nested pins/maps, against the
	// supplied JSON value. This is an actual decoder regression, not a hash test.
	var before, after any
	if err := json.Unmarshal(raw, &before); err != nil { t.Fatalf("fixture JSON: %v", err) }
	if err := json.Unmarshal(schemaTestBytes(t, decoded), &after); err != nil { t.Fatalf("decoded JSON: %v", err) }
	if !bytes.Equal(schemaTestBytes(t, before), schemaTestBytes(t, after)) { t.Fatal("typed decoder changed or omitted manifest members") }
}

func schemaTestReject(t *testing.T, raw []byte) {
	t.Helper()
	if _, err := decodeManifest(raw); err == nil { t.Fatal("invalid manifest accepted by actual decoder") }
}

func TestManualBackupManifestExactSchema(t *testing.T) {
	cases := 0
	run := func(name string, value any) {
		cases++
		raw := schemaTestBytes(t, value)
		t.Run(name, func(t *testing.T) { schemaTestAccept(t, raw) })
	}
	for _, withOracle := range []bool{false, true} {
		name := "nonoracle"
		if withOracle { name = "oracle" }
		run("canonical_" + name, schemaTestFixture(withOracle))
		run("reversed_members_" + name, schemaTestReorder(schemaTestFixture(withOracle), false))
		run("escaped_exact_members_" + name, schemaTestReorder(schemaTestFixture(withOracle), true))
	}
	for _, keepAMD64 := range []bool{false, true} {
		value := schemaTestSet(t, schemaTestFixture(false), nil, "goarch", "arm64")
		value = schemaTestSet(t, value, []string{"build_env"}, "GOARCH", "arm64")
		if !keepAMD64 {
			value = schemaTestEdit(t, value, []string{"build_env"}, func(object schemaTestObject) any {
				index := schemaTestMemberIndex(t, object, "GOAMD64")
				return append(object[:index], object[index+1:]...)
			})
		}
		run("nonamd64_keep_GOAMD64_" + strconv.FormatBool(keepAMD64), value)
	}
	for _, length := range []int{1, 16} {
		modules := make([]any, length)
		for index := range modules { modules[index] = schemaTestObject{{"path", "module-" + strconv.Itoa(index)}, {"version", "v1.0.0"}, {"sum", "synthetic"}} }
		run("modules_" + strconv.Itoa(length), schemaTestSet(t, schemaTestFixture(false), nil, "modules", modules))
	}
	for _, key := range []string{"build_argv", "runtime_argv"} {
		for _, length := range []int{1, 32} {
			values := make([]any, length)
			for index := range values { values[index] = "argument-" + strconv.Itoa(index) }
			run(key + "_" + strconv.Itoa(length), schemaTestSet(t, schemaTestFixture(false), nil, key, values))
		}
	}
	if cases != 14 { t.Fatalf("frozen exact-schema case count: got %d, want 14", cases) }
}

func TestManualBackupManifestRejectsCaseAliases(t *testing.T) {
	cases, fields := 0, 0
	for _, location := range schemaTestLocations() {
		for _, key := range location.keys {
			fields++
			// First-letter and all-letter case swaps are two fixed variants for
			// every field, not a random/exponential case-permutation campaign.
			first := strings.ToUpper(key[:1]) + key[1:]
			all := strings.ToUpper(key)
			if first == key { first = strings.ToLower(key[:1]) + key[1:] }
			if all == key { all = strings.ToLower(key) }
			if first == key || all == key || first == all { t.Fatalf("fixture alias pair for %q is not distinct", key) }
			value := schemaTestEdit(t, schemaTestFixture(true), location.path, func(object schemaTestObject) any {
				index := schemaTestMemberIndex(t, object, key)
				object[index].value = schemaTestDifferent(object[index].value)
				return object
			})
			cases++
			t.Run(location.name + "/" + key + "/canonical_different", func(t *testing.T) { schemaTestAccept(t, schemaTestBytes(t, value)) })
			for aliasIndex, alias := range []string{first, all} {
				for _, mode := range []string{"alias_only", "canonical_first_equal", "alias_first_equal", "canonical_first_different", "alias_first_different"} {
					value := schemaTestEdit(t, schemaTestFixture(true), location.path, func(object schemaTestObject) any {
						index := schemaTestMemberIndex(t, object, key)
						member := object[index]
						aliased := schemaTestMember{alias, member.value}
						if strings.HasSuffix(mode, "different") { aliased.value = schemaTestDifferent(member.value) }
						if mode == "alias_only" { object[index] = aliased; return object }
						result := append(schemaTestObject(nil), object[:index]...)
						if strings.HasPrefix(mode, "alias_first") { result = append(result, aliased, member) } else { result = append(result, member, aliased) }
						return append(result, object[index+1:]...)
					})
					cases++
					name := location.name + "/" + key + "/alias_" + strconv.Itoa(aliasIndex) + "/" + mode
					t.Run(name, func(t *testing.T) { schemaTestReject(t, schemaTestBytes(t, value)) })
				}
			}
		}
	}
	if fields != 81 || cases != 891 { t.Fatalf("frozen alias counts: got %d fields/%d cases, want 81/891", fields, cases) }
}

func TestManualBackupManifestRejectsMissingAndUnknownFields(t *testing.T) {
	cases := 0
	run := func(name string, value any) {
		cases++
		raw := schemaTestBytes(t, value)
		t.Run(name, func(t *testing.T) { schemaTestReject(t, raw) })
	}
	wrongKinds := []any{nil, schemaTestObject{}, []any{}, "wrong-shape", 1, true}
	for _, location := range schemaTestLocations() {
		unknown := schemaTestEdit(t, schemaTestFixture(true), location.path, func(object schemaTestObject) any {
			return append(object, schemaTestMember{"unapproved_member", "synthetic"})
		})
		run(location.name + "/unknown", unknown)
		for _, key := range location.keys {
			missing := schemaTestEdit(t, schemaTestFixture(true), location.path, func(object schemaTestObject) any {
				index := schemaTestMemberIndex(t, object, key)
				return append(object[:index], object[index+1:]...)
			})
			run(location.name + "/" + key + "/missing", missing)
			for _, replacement := range wrongKinds {
				kind := schemaTestKind(replacement)
				include := true
				value := schemaTestEdit(t, schemaTestFixture(true), location.path, func(object schemaTestObject) any {
					index := schemaTestMemberIndex(t, object, key)
					if schemaTestKind(object[index].value) == kind || (location.name == "manifest" && key == "oracle" && kind == "null") { include = false }
					object[index].value = replacement
					return object
				})
				if include { run(location.name + "/" + key + "/wrong_" + kind, value) }
			}
		}
	}
	for _, replacement := range wrongKinds {
		kind := schemaTestKind(replacement)
		if kind == "object" { continue }
		run("root/wrong_" + kind, replacement)
		for index := 0; index < 3; index++ {
			value := schemaTestEdit(t, schemaTestFixture(true), []string{"modules", strconv.Itoa(index)}, func(schemaTestObject) any { return replacement })
			run("modules_" + strconv.Itoa(index) + "/wrong_" + kind, value)
		}
	}
	for _, key := range []string{"build_argv", "runtime_argv"} {
		for _, replacement := range wrongKinds {
			if schemaTestKind(replacement) != "string" { run(key + "/element_wrong_" + schemaTestKind(replacement), schemaTestSet(t, schemaTestFixture(true), nil, key, []any{replacement})) }
		}
	}
	for _, location := range schemaTestLocations() {
		key := "size"
		if location.name == "manifest" { key = "schema" } else if len(location.keys) != 3 || location.keys[1] != "size" { continue }
		for _, number := range []string{"1.5", "1e0", "9223372036854775808"} {
			run(location.name + "/" + key + "/invalid_integer_" + number, schemaTestSet(t, schemaTestFixture(true), location.path, key, json.Number(number)))
		}
	}
	for _, key := range []string{"modules", "build_argv", "runtime_argv"} {
		maximum := 32
		if key == "modules" { maximum = 16 }
		for _, length := range []int{0, maximum + 1} {
			values := make([]any, length)
			for index := range values {
				values[index] = "argument"
				if key == "modules" { values[index] = schemaTestObject{{"path", "module-" + strconv.Itoa(index)}, {"version", "v1.0.0"}, {"sum", "synthetic"}} }
			}
			run(key + "/out_of_bounds_" + strconv.Itoa(length), schemaTestSet(t, schemaTestFixture(true), nil, key, values))
		}
	}
	value := schemaTestEdit(t, schemaTestFixture(false), []string{"runtime_env"}, func(object schemaTestObject) any { return append(object, schemaTestMember{"SQLITE_ORACLE", "/synthetic/sqlite3"}) })
	run("nonoracle/unexpected_SQLITE_ORACLE", value)
	for _, location := range []string{"build_env", "runtime_env"} {
		value := schemaTestEdit(t, schemaTestFixture(true), []string{location}, func(object schemaTestObject) any { return append(object, schemaTestMember{"GOARM64", "v8.0"}) })
		run(location + "/unapproved_GOARM64", value)
	}
	if cases != 562 { t.Fatalf("frozen missing/unknown/shape count: got %d, want 562", cases) }
}

func TestManualBackupManifestRejectsDuplicateAndUnboundedJSON(t *testing.T) {
	cases := 0
	runDecode := func(name string, raw []byte, accepted bool) {
		cases++
		t.Run(name, func(t *testing.T) {
			if accepted { schemaTestAccept(t, raw) } else { schemaTestReject(t, raw) }
		})
	}
	runUnique := func(name string, raw []byte, accepted bool) {
		cases++
		t.Run(name, func(t *testing.T) {
			err := uniqueJSON(raw)
			if (err == nil) != accepted { t.Fatalf("uniqueJSON boundary accepted=%t, want %t; error=%v", err == nil, accepted, err) }
		})
	}
	for _, location := range schemaTestLocations() {
		for _, key := range location.keys {
			for _, mode := range []string{"exact_equal", "exact_different", "escaped_after_equal", "escaped_before_equal", "escaped_after_different", "escaped_before_different"} {
				value := schemaTestEdit(t, schemaTestFixture(true), location.path, func(object schemaTestObject) any {
					index := schemaTestMemberIndex(t, object, key)
					member := object[index]
					duplicate := member
					if strings.HasPrefix(mode, "escaped_") { duplicate.key = schemaTestEscapeKey(key) }
					if strings.HasSuffix(mode, "different") { duplicate.value = schemaTestDifferent(member.value) }
					result := append(schemaTestObject(nil), object[:index]...)
					if strings.Contains(mode, "_before_") { result = append(result, duplicate, member) } else { result = append(result, member, duplicate) }
					return append(result, object[index+1:]...)
				})
				runDecode(location.name + "/" + key + "/" + mode, schemaTestBytes(t, value), false)
			}
		}
	}
	canonical := schemaTestBytes(t, schemaTestFixture(false))
	for _, suffix := range []string{"{}", "null", "0", "true", "[]", "trailing"} {
		runDecode("trailing_" + suffix, append(append([]byte(nil), canonical...), []byte(" " + suffix)...), false)
	}
	for index, raw := range []string{"", " ", "{", "[", "}", `{"schema":1,}`, `{"schema":`, `{"schema":"\x"}`} {
		runDecode("malformed_" + strconv.Itoa(index), []byte(raw), false)
	}
	if len(canonical) >= manifestLimit { t.Fatal("canonical fixture exceeds raw-byte boundary") }
	atLimit := append(append([]byte(nil), canonical...), bytes.Repeat([]byte(" "), manifestLimit-len(canonical))...)
	runDecode("raw_bytes_262144", atLimit, true)
	runDecode("raw_bytes_262145", append(append([]byte(nil), atLimit...), ' '), false)
	for _, length := range []int{4096, 4097} {
		value := schemaTestSet(t, schemaTestFixture(false), nil, "source_tree", strings.Repeat("x", length))
		runDecode("decoded_string_bytes_" + strconv.Itoa(length), schemaTestBytes(t, value), length == 4096)
		value = schemaTestSet(t, schemaTestFixture(false), nil, "source_tree", json.RawMessage(`"` + strings.Repeat(`\u0061`, length) + `"`))
		runDecode("escaped_string_bytes_" + strconv.Itoa(length), schemaTestBytes(t, value), length == 4096)
	}
	for _, length := range []int{2048, 2049} {
		value := schemaTestSet(t, schemaTestFixture(false), nil, "source_tree", strings.Repeat("é", length))
		runDecode("utf8_string_bytes_" + strconv.Itoa(length * 2), schemaTestBytes(t, value), length == 2048)
	}
	// schema has no 256-byte field or depth-16 path. These direct calls test
	// the token pass's real contract, rather than expecting invalid schemas to
	// be admitted or attributing their rejection to an unobserved quota.
	for _, depth := range []int{16, 17} {
		raw := []byte(strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth))
		runUnique("unique_depth_" + strconv.Itoa(depth), raw, depth == 16)
	}
	for _, values := range []int{11999, 12000} {
		raw := []byte("[" + strings.Repeat("0,", values-1) + "0]")
		runUnique("unique_value_count_" + strconv.Itoa(values+1), raw, values == 11999)
	}
	for _, length := range []int{256, 257} {
		raw := schemaTestBytes(t, schemaTestObject{{strings.Repeat("k", length), 0}})
		runUnique("unique_key_bytes_" + strconv.Itoa(length), raw, length == 256)
		raw = []byte(`{"` + strings.Repeat(`\u006b`, length) + `":0}`)
		runUnique("unique_escaped_key_bytes_" + strconv.Itoa(length), raw, length == 256)
	}
	if cases != 516 { t.Fatalf("frozen duplicate/bounds count: got %d, want 516", cases) }
}
