package main

import (
	"fmt"
	"strings"
)

const oracleShellPrefix = `/usr/bin/env -i LC_ALL=C LANG=C TZ=UTC "SQLITE_TMPDIR=$oracle_temp_root" "TMPDIR=$oracle_temp_root" "$oracle_exe" -noinit`

// Exactly the five current direct shell sites, in source order. This is a
// text/source contract, not a shell evaluator; it never invokes even -help or
// --version. Whole-file hashes also require review of added indirect sites,
// reassignment, control-flow changes, or a change to the retained shell input.
// Historical evidence and any future new script need their own source review.
var oracleShellSites = map[string][]string{
	"scripts/build-oracle.sh": {
		`actual=$(` + oracleShellPrefix + ` -batch -bail -noheader -list :memory: 'SELECT sqlite_version(),sqlite_source_id();')`,
		oracleShellPrefix + ` --version`,
	},
	"scripts/install-baselines.sh": {
		oracleShellPrefix + ` --version`,
		oracleShellPrefix + ` :memory: 'SELECT sqlite_version(), sqlite_source_id(); PRAGMA integrity_check;'`,
	},
	"scripts/test-oracle.sh": {
		`actual=$(` + oracleShellPrefix + ` -batch -bail -noheader -list :memory: 'SELECT sqlite_version(),sqlite_source_id();')`,
	},
}

var oracleShellHashes = map[string]string{
	"scripts/build-oracle.sh": "0826180e18c7e67bd3ab35b0b534409578932adba13041b8e5b1e32fb43cca66",
	"scripts/install-baselines.sh": "0943fdec085c8cbd2ed53951c563ded9e6107e6358a86213005c5a6eb25d3792",
	"scripts/test-oracle.sh": "b8e3e443a3fbe17b96526cfdb22b6a297dedf0eac2d9c3063cd7f9da05a64f79",
}

var oracleShellSetup = []string{
	`if [ "$(/usr/bin/uname -s)" != Linux ]; then`,
	`oracle_absolute_path() {`,
	`    *//*|*/./*|*/../*|*/.|*/..|*\\*) return 1 ;;`,
	`    */) return 1 ;;`,
	`oracle_temp_root=${GOTMPDIR:-${TMPDIR:-/tmp}}`,
	`oracle_work_dir=$(pwd -P)`,
	`for oracle_path in "$oracle_exe" "$oracle_temp_root" "$oracle_work_dir"; do`,
	`  if ! oracle_absolute_path "$oracle_path"; then`,
	`case "$oracle_temp_root" in`,
	`  /tmp|/tmp/*) ;;`,
	`if [ ! -d "$oracle_temp_root" ] || [ ! -w "$oracle_temp_root" ] || [ ! -x "$oracle_temp_root" ]; then`,
	`if [ ! -f "$oracle_exe" ] || [ ! -x "$oracle_exe" ]; then`,
	`cd "$oracle_work_dir"`,
}

func inspectOracleShellStructure(path string, raw []byte) []string {
	sites, reviewed := oracleShellSites[path]
	if !reviewed {
		return []string{path + ": unreviewed oracle shell path"}
	}
	text := string(raw)
	var findings []string
	report := func(message string) {
		findings = append(findings, path+": oracle shell startup contract: "+message)
	}
	if strings.Count(text, oracleShellPrefix) != len(sites) {
		report(fmt.Sprintf("clean-child prefix count differs from %d reviewed sites", len(sites)))
	}
	lines := strings.Split(text, "\n")
	previous := -1
	for _, site := range sites {
		count, lineNumber := 0, -1
		for i, line := range lines {
			if line == site {
				count++
				lineNumber = i
			}
		}
		if count != 1 || lineNumber <= previous {
			report("missing, duplicated, changed, or reordered direct site: "+site)
		}
		previous = lineNumber
	}
	firstSite := strings.Index(text, oracleShellPrefix)
	for _, setup := range oracleShellSetup {
		position := strings.Index(text, setup)
		if strings.Count(text, setup) != 1 || position >= firstSite {
			report("required unique preflight/CWD source before child: "+setup)
		}
	}
	executable := `oracle_exe="$oracle_work_dir/sqlite3"`
	if path == "scripts/test-oracle.sh" {
		executable = `oracle_exe="$SQLITE_ORACLE"`
	}
	if strings.Count(text, executable) != 1 || strings.Index(text, executable) >= firstSite {
		report("missing exact absolute executable selection before child")
	}
	return findings
}

func inspectOracleShellSource(path string, raw []byte) []string {
	findings := inspectOracleShellStructure(path, raw)
	if want, ok := oracleShellHashes[path]; !ok || oracleSourceDigest(raw) != want {
		findings = append(findings, path+": reviewed full shell source changed; inspect before updating its source hash")
	}
	return findings
}
