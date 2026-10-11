package main

import (
	"strings"
	"testing"
)

// A synthetic source-contract fixture, never a shell input or subprocess. The
// incomplete shell scaffold is intentional: these tests inspect text only.
func oracleShellFixture(path string) string {
	executable := `oracle_exe="$oracle_work_dir/sqlite3"`
	if path == "scripts/test-oracle.sh" {
		executable = `oracle_exe="$SQLITE_ORACLE"`
	}
	return strings.Join(oracleShellSetup, "\n") + "\n" + executable + "\n" + strings.Join(oracleShellSites[path], "\n") + "\n"
}

func TestOracleShellFiveSiteContract(t *testing.T) {
	total := 0
	for path, sites := range oracleShellSites {
		total += len(sites)
		source := oracleShellFixture(path)
		if got := inspectOracleShellStructure(path, []byte(source)); len(got) != 0 {
			t.Fatalf("exact text fixture rejected: %v", got)
		}
		if got := inspectOracleShellSource(path, []byte(source)); len(got) == 0 {
			t.Fatal("shape-only fixture passed frozen complete-script source contract")
		}
		for _, site := range sites {
			changed := strings.Replace(source, site, "# removed direct site", 1)
			if got := inspectOracleShellStructure(path, []byte(changed)); len(got) == 0 {
				t.Fatal("omitted site accepted", path, site)
			}
			changed = strings.Replace(source, site, site+"\n"+site, 1)
			if got := inspectOracleShellStructure(path, []byte(changed)); len(got) == 0 {
				t.Fatal("duplicate site accepted", path, site)
			}
		}
	}
	if len(oracleShellSites) != 3 || total != 5 {
		t.Fatalf("shell inventory = %d scripts/%d sites, want 3/5", len(oracleShellSites), total)
	}
	if got := inspectOracleShellSource("docs/evidence/old.sh", []byte(oracleShellPrefix)); len(got) == 0 {
		t.Fatal("historical script admitted as a current contract")
	}
}

func TestOracleShellStartupMutationGuard(t *testing.T) {
	for path := range oracleShellSites {
		source := oracleShellFixture(path)
		for _, tc := range []struct {
			name, old, replacement string
		}{
			{"inherit-env", "/usr/bin/env -i ", "/usr/bin/env "},
			{"lookup-env", "/usr/bin/env -i ", "env -i "},
			{"no-noinit", " -noinit", ""},
			{"debug-empty", " LC_ALL=C ", " SQLITE_DEBUG_BREAK= LC_ALL=C "},
			{"ambient-home", " LANG=C ", " HOME=$HOME LANG=C "},
			{"missing-locale", " LC_ALL=C ", " "},
			{"missing-lang", " LANG=C ", " "},
			{"missing-tz", " TZ=UTC ", " "},
			{"missing-sqlite-temp", ` "SQLITE_TMPDIR=$oracle_temp_root"`, ""},
			{"missing-tmp", ` "TMPDIR=$oracle_temp_root"`, ""},
			{"different-temp", `"TMPDIR=$oracle_temp_root"`, `"TMPDIR=/var/tmp"`},
			{"bare-executable", `"$oracle_exe" -noinit`, `sqlite3 -noinit`},
			{"relative-executable", `"$oracle_exe" -noinit`, `./sqlite3 -noinit`},
			{"unquoted-executable", `"$oracle_exe" -noinit`, `$oracle_exe -noinit`},
			{"no-linux-check", `if [ "$(/usr/bin/uname -s)" != Linux ]; then`, `# no platform check`},
			{"wrong-temp-precedence", `oracle_temp_root=${GOTMPDIR:-${TMPDIR:-/tmp}}`, `oracle_temp_root=${TMPDIR:-/tmp}`},
			{"cwd-inherited", `cd "$oracle_work_dir"`, `# inherited CWD`},
			{"cwd-not-physical", `oracle_work_dir=$(pwd -P)`, `oracle_work_dir=$(pwd)`},
			{"no-clean-path-check", `  if ! oracle_absolute_path "$oracle_path"; then`, `# no path check`},
			{"broader-temp-root", `  /tmp|/tmp/*) ;;`, `  /*) ;;`},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				if !strings.Contains(source, tc.old) {
					t.Fatalf("mutation does not match source fixture: %s", tc.name)
				}
				changed := strings.Replace(source, tc.old, tc.replacement, 1)
				if got := inspectOracleShellStructure(path, []byte(changed)); len(got) == 0 {
					t.Fatal("unsafe source mutation accepted", tc.name)
				}
			})
		}
		for _, setup := range oracleShellSetup {
			changed := strings.Replace(source, setup+"\n", "", 1) + setup + "\n"
			if got := inspectOracleShellStructure(path, []byte(changed)); len(got) == 0 {
				t.Fatal("preflight moved after launch accepted", path, setup)
			}
		}
		// The text model cannot interpret a new alias or branch. The frozen
		// complete source is the fail-closed boundary for such source changes.
		if got := inspectOracleShellSource(path, []byte(source+"launch=./sqlite3\n$launch --version\n")); len(got) == 0 {
			t.Fatal("indirect extra invocation accepted by source hash")
		}
	}
}

func TestOracleFrozenSourceContract(t *testing.T) {
	if got := oracleSourceDigest([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal("unexpected SHA256 source digest", got)
	}
	for path := range oracleFrozenSources {
		if got := inspectOracleFrozenSource(path, []byte("changed source")); len(got) != 1 {
			t.Fatal("changed reviewed Go source accepted", path, got)
		}
	}
	if got := inspectOracleFrozenSource("sqlite/ordinary.go", []byte("not a frozen exception")); len(got) != 0 {
		t.Fatal("ordinary source treated as a frozen exception", got)
	}
}
