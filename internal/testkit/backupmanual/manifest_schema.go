// Package backupmanual includes a portable, descriptor-free manifest decoder.
// Native manual qualification lives in the separately tagged Linux file.
package backupmanual

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const (
	ackKey = "GOSQLITE_MANUAL_BACKUP_PUBLICATION"
	rootKey = "GOSQLITE_MANUAL_BACKUP_ROOT"
	manifestLimit = 256 << 10
)

type filePin struct {
	Path string `json:"path"`
	Size int64 `json:"size"`
	SHA256 string `json:"sha256"`
}
type modulePin struct {
	Path string `json:"path"`
	Version string `json:"version"`
	Sum string `json:"sum"`
}
type oraclePin struct {
	Binary filePin `json:"binary"`
	Version string `json:"version"`
	SourceID string `json:"source_id"`
}
type manifest struct {
	Schema int `json:"schema"`
	Root string `json:"root"`
	Profile string `json:"profile"`
	Namespace string `json:"namespace"`
	SourceTree string `json:"source_tree"`
	SourceInventory filePin `json:"source_inventory"`
	ToolchainInventory filePin `json:"toolchain_inventory"`
	ModuleInventory filePin `json:"module_inventory"`
	GoMod filePin `json:"go_mod"`
	GoSum filePin `json:"go_sum"`
	GoVersion string `json:"go_version"`
	GOARCH string `json:"goarch"`
	Modules []modulePin `json:"modules"`
	BuildTags string `json:"build_tags"`
	BuildArgv []string `json:"build_argv"`
	BuildEnv map[string]string `json:"build_env"`
	RuntimeArgv []string `json:"runtime_argv"`
	RuntimeEnv map[string]string `json:"runtime_env"`
	Selector string `json:"selector"`
	Executable filePin `json:"executable"`
	Oracle *oraclePin `json:"oracle"`
}
// Reject duplicate keys, excessive nesting/entry counts, long strings, and a
// second JSON value before typed decoding. DisallowUnknownFields alone cannot
// reject duplicate keys. This parser never executes manifest content.
func uniqueJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	count := 0
	var value func(int) error
	value = func(depth int) error {
		count++
		if depth > 16 || count > 12000 { return fmt.Errorf("JSON bounds") }
		tok, err := d.Token()
		if err != nil { return err }
		if s, ok := tok.(string); ok && len(s) > 4096 { return fmt.Errorf("JSON string bounds") }
		delim, ok := tok.(json.Delim)
		if !ok { return nil }
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				tok, err := d.Token()
				if err != nil { return err }
				key, ok := tok.(string)
				if !ok || len(key) > 256 || keys[key] { return fmt.Errorf("duplicate or invalid JSON key") }
				keys[key] = true
				if err := value(depth+1); err != nil { return err }
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') { return fmt.Errorf("JSON object end") }
		case '[':
			for d.More() { if err := value(depth+1); err != nil { return err } }
			end, err := d.Token()
			if err != nil || end != json.Delim(']') { return fmt.Errorf("JSON array end") }
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
		return nil
	}
	if err := value(0); err != nil { return err }
	if _, err := d.Token(); err != io.EOF { return fmt.Errorf("multiple JSON values") }
	return nil
}

// schemaObject rejects every non-exact decoded key before struct decoding.
// Go's JSON struct decoder accepts folded spellings; DisallowUnknownFields is
// retained as defense in depth, never treated as an exact-key proof.
func schemaObject(raw json.RawMessage, required []string) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' { return nil, fmt.Errorf("schema object required") }
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil { return nil, err }
	if len(object) != len(required) { return nil, fmt.Errorf("schema exact member count") }
	for _, key := range required {
		if _, ok := object[key]; !ok { return nil, fmt.Errorf("schema missing exact member %q", key) }
	}
	return object, nil
}
func schemaString(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '"' { return fmt.Errorf("schema string required") }
	var value string
	return json.Unmarshal(raw, &value)
}
func schemaInteger(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] == 'n' { return fmt.Errorf("schema nonnull integer required") }
	var value int64
	return json.Unmarshal(raw, &value)
}
func schemaArray(raw json.RawMessage, minimum, maximum int) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' { return nil, fmt.Errorf("schema array required") }
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil { return nil, err }
	if len(values) < minimum || len(values) > maximum { return nil, fmt.Errorf("schema array bounds") }
	return values, nil
}
func schemaFilePin(raw json.RawMessage) error {
	object, err := schemaObject(raw, []string{"path", "size", "sha256"})
	if err != nil { return err }
	if err := schemaString(object["path"]); err != nil { return err }
	if err := schemaInteger(object["size"]); err != nil { return err }
	return schemaString(object["sha256"])
}
func schemaModule(raw json.RawMessage) error {
	object, err := schemaObject(raw, []string{"path", "version", "sum"})
	if err != nil { return err }
	for _, key := range []string{"path", "version", "sum"} {
		if err := schemaString(object[key]); err != nil { return err }
	}
	return nil
}
func schemaOracle(raw json.RawMessage) (bool, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) { return false, nil }
	object, err := schemaObject(raw, []string{"binary", "version", "source_id"})
	if err != nil { return false, err }
	if err := schemaFilePin(object["binary"]); err != nil { return false, err }
	if err := schemaString(object["version"]); err != nil { return false, err }
	if err := schemaString(object["source_id"]); err != nil { return false, err }
	return true, nil
}
func schemaStringArray(raw json.RawMessage) error {
	values, err := schemaArray(raw, 1, 32)
	if err != nil { return err }
	for _, value := range values {
		if err := schemaString(value); err != nil { return err }
	}
	return nil
}
func schemaEnvironment(raw json.RawMessage, allowed, required []string) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' { return nil, fmt.Errorf("schema environment object required") }
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil { return nil, err }
	if len(object) > len(allowed) { return nil, fmt.Errorf("schema environment member count") }
	for key, value := range object {
		known := false
		for _, exact := range allowed { if key == exact { known = true } }
		if !known { return nil, fmt.Errorf("schema unknown environment member %q", key) }
		if err := schemaString(value); err != nil { return nil, err }
	}
	for _, exact := range required {
		if _, ok := object[exact]; !ok { return nil, fmt.Errorf("schema missing environment member %q", exact) }
	}
	return object, nil
}
func validateManifestSchema(raw []byte) error {
	object, err := schemaObject(raw, []string{
		"schema", "root", "profile", "namespace", "source_tree",
		"source_inventory", "toolchain_inventory", "module_inventory", "go_mod", "go_sum",
		"go_version", "goarch", "modules", "build_tags", "build_argv", "build_env",
		"runtime_argv", "runtime_env", "selector", "executable", "oracle",
	})
	if err != nil { return err }
	if err := schemaInteger(object["schema"]); err != nil { return err }
	for _, key := range []string{"root", "profile", "namespace", "source_tree", "go_version", "goarch", "build_tags", "selector"} {
		if err := schemaString(object[key]); err != nil { return err }
	}
	for _, key := range []string{"source_inventory", "toolchain_inventory", "module_inventory", "go_mod", "go_sum", "executable"} {
		if err := schemaFilePin(object[key]); err != nil { return err }
	}
	modules, err := schemaArray(object["modules"], 1, 16)
	if err != nil { return err }
	for _, value := range modules { if err := schemaModule(value); err != nil { return err } }
	if err := schemaStringArray(object["build_argv"]); err != nil { return err }
	if err := schemaStringArray(object["runtime_argv"]); err != nil { return err }
	oracle, err := schemaOracle(object["oracle"])
	if err != nil { return err }
	buildRequired := []string{"PATH", "GOROOT", "GOPATH", "GOMODCACHE", "GOCACHE", "GOTMPDIR", "TMPDIR", "CGO_ENABLED", "GOTOOLCHAIN", "GOENV", "GOWORK", "GOFLAGS", "GOPROXY", "GOSUMDB", "GOOS", "GOARCH", "GOMAXPROCS"}
	buildAllowed := append(append([]string(nil), buildRequired...), "GOAMD64")
	build, err := schemaEnvironment(object["build_env"], buildAllowed, buildRequired)
	if err != nil { return err }
	var architecture string
	if err := json.Unmarshal(build["GOARCH"], &architecture); err != nil { return err }
	if architecture == "amd64" {
		if _, ok := build["GOAMD64"]; !ok { return fmt.Errorf("schema missing GOAMD64 for amd64") }
	}
	runtimeKeys := []string{ackKey, rootKey, "GOTMPDIR", "TMPDIR", "LC_ALL", "LANG", "TZ", "GOMAXPROCS"}
	if oracle { runtimeKeys = append(runtimeKeys, "SQLITE_ORACLE") }
	if _, err := schemaEnvironment(object["runtime_env"], runtimeKeys, runtimeKeys); err != nil { return err }
	return nil
}

// decodeManifest has no environment, filesystem, runtime, native or process
// dependency. All size/duplicate/depth/token checks run before the exact-schema
// pass, which completes before case-folding typed decoding can observe a key.
// Values still undergo the unchanged native caller's provenance/runtime checks.
func decodeManifest(raw []byte) (manifest, error) {
	if len(raw) > manifestLimit { return manifest{}, fmt.Errorf("manifest byte limit") }
	if err := uniqueJSON(raw); err != nil { return manifest{}, err }
	if err := validateManifestSchema(raw); err != nil { return manifest{}, err }
	var result manifest
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&result); err != nil { return manifest{}, err }
	return result, nil
}
