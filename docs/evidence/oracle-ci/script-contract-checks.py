#!/usr/bin/env python3
"""Pure shell-contract checks with synthetic tools; never database acceptance."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

root = Path(__file__).resolve().parents[3]
identity = '3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3'
for shell, path in [('bash', 'scripts/build-oracle.sh'), ('sh', 'scripts/test-oracle.sh')]:
    subprocess.run([shell, '-n', str(root / path)], check=True)
    print(f'PASS syntax: {path}')
with tempfile.TemporaryDirectory(prefix='oracle-script-contract-') as directory:
    temporary = Path(directory)
    binary = temporary / 'bin'
    binary.mkdir()
    log = temporary / 'calls.jsonl'
    go = binary / 'go'
    go.write_text('''#!/usr/bin/env python3
import json, os, sys
with open(os.environ['MOCK_LOG'], 'a') as out:
    out.write(json.dumps(['go', sys.argv[1:], os.environ.get('CGO_ENABLED'), os.environ.get('GOTOOLCHAIN')]) + '\\n')
if sys.argv[1:] == ['env', 'GOVERSION']:
    print(os.environ.get('MOCK_GO_VERSION', 'go1.27.2'))
elif sys.argv[1:] == ['run', './internal/testscope']:
    sys.exit(int(os.environ.get('MOCK_GUARD_STATUS', '0')))
elif sys.argv[1:] != ['test', '-count=1', '-timeout=30m', '-tags', 'oracle', './...']:
    sys.exit('Unexpected go command')
''')
    go.chmod(0o700)
    oracle = binary / 'oracle'
    oracle.write_text('''#!/usr/bin/env python3
import json, os, sys
with open(os.environ['MOCK_LOG'], 'a') as out:
    out.write(json.dumps(['oracle', sys.argv[1:]]) + '\\n')
print(os.environ['MOCK_ORACLE_ID'])
''')
    oracle.chmod(0o700)
    common = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ['PATH'],
                  SQLITE_ORACLE=str(oracle), MOCK_LOG=str(log), MOCK_ORACLE_ID=identity)
    def run_case(name, changes, expected_status, expected_calls):
        log.write_text('')
        env = dict(common)
        for key, value in changes.items():
            if value is None:
                env.pop(key, None)
            else:
                env[key] = value
        proc = subprocess.run([str(root / 'scripts/test-oracle.sh')], cwd=root, env=env,
                              text=True, capture_output=True)
        assert (proc.returncode == 0) == expected_status, (name, proc.returncode, proc.stderr)
        calls = [json.loads(line) for line in log.read_text().splitlines()]
        assert [entry[0] for entry in calls] == expected_calls, (name, calls)
        if expected_status:
            assert calls[1][1] == ['run', './internal/testscope']
            assert calls[2][1] == ['-batch', '-bail', '-noheader', '-list', ':memory:', 'SELECT sqlite_version(),sqlite_source_id();']
            assert calls[-1] == ['go', ['test', '-count=1', '-timeout=30m', '-tags', 'oracle', './...'], '0', 'local']
        print(f'PASS synthetic contract: {name}; calls={calls}')
    run_case('mandatory oracle environment', {'SQLITE_ORACLE': None}, False, [])
    run_case('wrong Go blocks execution', {'MOCK_GO_VERSION': 'go1.26.0'}, False, ['go'])
    run_case('relative oracle path rejected', {'SQLITE_ORACLE': 'oracle'}, False, ['go'])
    run_case('missing executable rejected', {'SQLITE_ORACLE': str(binary / 'absent')}, False, ['go'])
    run_case('guard failure prevents oracle and tests', {'MOCK_GUARD_STATUS': '7'}, False, ['go', 'go'])
    run_case('wrong SQLite ID prevents tests', {'MOCK_ORACLE_ID': '3.54.0|wrong'}, False, ['go', 'go', 'oracle'])
    run_case('success invokes exactly complete CGO=0 ordinary oracle', {}, True, ['go', 'go', 'oracle', 'go'])
    # Non-database text inputs exercise source installer refusal only.
    wrong_archive = temporary / 'not-sqlite-source.zip'
    wrong_archive.write_text('ordinary text; not an archive')
    dest = temporary / 'builder-size-refusal'
    proc = subprocess.run([str(root / 'scripts/build-oracle.sh'), str(dest), str(wrong_archive)],
                          text=True, capture_output=True)
    assert proc.returncode != 0 and 'archive size mismatch' in proc.stderr, proc
    assert not (dest / 'sqlite-src-3540000').exists()
    print('PASS builder refuses incorrect archive byte count before extraction')
    proc = subprocess.run([str(root / 'scripts/build-oracle.sh'), str(dest), str(wrong_archive)],
                          text=True, capture_output=True)
    assert proc.returncode != 0 and 'File exists' in proc.stderr, proc
    print('PASS builder refuses existing destination')
workflow = (root / '.github/workflows/check.yml').read_text()
assert [line for line in workflow.splitlines() if line.startswith('  ') and not line.startswith('   ') and line.endswith(':')] == ['  ordinary:', '  oracle:']
assert workflow.count("go-version: '1.27.2'") == 2
assert 'sudo' not in workflow and 'apt-get' not in workflow
assert 'run: ./scripts/check.sh' in workflow
assert 'run: ./scripts/build-oracle.sh "$RUNNER_TEMP/sqlite-oracle"' in workflow
assert 'run: ./scripts/test-oracle.sh' in workflow
assert 'SQLITE_ORACLE: ${{ runner.temp }}/sqlite-oracle/sqlite-build-3540000/sqlite3' in workflow
assert 'manual_' not in workflow
print('PASS workflow source contract: exactly two jobs, pinned Go/oracle, no system installs or manual tags')
print('Synthetic tool checks validate shell routing only; they are not Go test or hosted CI results.')
