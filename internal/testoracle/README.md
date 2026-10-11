# Test-only official SQLite startup contract

This package only constructs launch specifications and unstarted commands. It
does not run SQLite, discover an executable, create fixtures, or read startup
configuration. Production SQL packages must not import it.

`NewSpec` takes an explicit target platform, executable, temporary root, package
working directory, and original argument list. It accepts the reviewed Linux
contract only. Paths must be nonempty, clean absolute Linux paths, without NUL,
backslash, UNC-style leading double slash, or `.`/`..` segments. The executable
cannot be `/`. The temporary root must be `/tmp` or a lexical descendant.
No PATH search, filesystem canonicalization, or executable identity claim is
made by these checks.

Each command has:

- `Path`: the supplied absolute executable
- `Args`: that executable, mandatory `-noinit`, then every original argument in
  its original order, including any already-present `-noinit`
- `Env`: a fresh nonnil slice containing exactly `LC_ALL=C`, `LANG=C`, `TZ=UTC`,
  `SQLITE_TMPDIR=<temporary root>`, and `TMPDIR=<the same temporary root>`
- `Dir`: the captured absolute package working directory

The direct `exec.Cmd` literal avoids `exec.Command` construction-time PATH and
GODEBUG reads. Stdio and process-lifetime controls remain zero and caller-owned.
There is no context, Cancel, WaitDelay, kill policy, extra descriptor, init, or
TestMain hook. Accessors and commands copy argument/environment slices.

`ForTest` first rejects unsupported actual runtime platforms and invalid
executable paths, captures `os.Getwd`, then explicitly reads only GOTMPDIR and
TMPDIR for temporary-root selection. Its pure selector follows the Go 1.27.2
Linux testing.TempDir precedence: nonempty GOTMPDIR, then nonempty TMPDIR, then
`/tmp`. An invalid selected root is an error, not permission to fall back. It
does not call TempDir or open a hidden file. Go's own test fixtures still use
their existing TempDir behavior; approved launches must pin those same inputs.

Six existing relative readonly fixture origins retain their meaning because
`Dir` stays the absolute current package directory. It is never silently moved
to the temporary root. The reviewed shell sites similarly capture physical CWD
and use an absolute executable. They clean only the SQLite child through the
trusted absolute `/usr/bin/env -i`, retaining compiler/bootstrap environment.

## Limits and future preflight

Actual oracle execution is a separate step. Establish the exact source tree,
executable identity/hash, Linux build, package/selector, arguments, stdio,
working directory, fixture paths, retained private temporary directory and
normal-completion policy before running it. Directory existence, permissions,
symlink/mount targets, stability and cleanup lifetime are not string-model
properties. SQLite can fall back to another directory if its selected temp
directory becomes unusable. `/tmp` as fallback is a broader shared namespace
than an explicitly approved private `/tmp/...` directory.

Windows execution is unsupported by this reviewed adapter. An explicit
Windows contract would need separately reviewed path semantics and required
environment keys; Go can otherwise add ambient SYSTEMROOT when starting a
Windows command. Foreign compilation or pure Linux-spec modeling on another
OS does not constitute native acceptance there.

This controls the declared child startup inputs. It does not isolate the OS,
loader/system configuration, parent shell, compiler, or already-running Go
runtime, and it does not prove binary provenance. It does not cap output or
alter normal live-writer retirement. Historical runs without this contract
keep their recorded results, but do not establish startup isolation.

## Pure checks

The tests in `spec_test.go` inspect inert strings and unstarted command fields.
They never invoke an oracle, read real configuration, or evaluate Cmd.Environ.
The source guards in `internal/testscope` parse raw source and read the five
current shell call sites as text. Their scope and syntactic/dataflow limits are
documented in their source. Source hashes freeze reviewed factory/launcher
contracts; edits require a deliberate inventory/review refresh.

Compiler and test execution need separate authorization. The intended focused
selectors are `TestSpecArguments`, `TestSpecRejectsInvalidPathsAndPlatforms`,
`TestSpecEnvironmentAndCopies`, `TestTempRootPrecedence`, and
`TestSpecPreservesRelativeFixtureCWD`. No oracle-tagged suite, shell launcher,
CI job, recovery or physical-failure campaign follows from a pure-check pass.
