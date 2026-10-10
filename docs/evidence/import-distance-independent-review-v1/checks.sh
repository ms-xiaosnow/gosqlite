#!/bin/bash
set -euo pipefail
P=/workspace/scratch/5e06a74e5f65/durable-restart
R=$P/import-distance-independent-review-v1
cd "$R/work"
export PATH="$P/toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$P/go-cache" GOPATH="$P/go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false
exec 3>&1
step(){ printf '\nCOMMAND:';printf ' %q' "$@";printf '\n';"$@"; }
date -u +%FT%TZ
step go version
step go run ./internal/testscope
step go test ./migration/postgres -run '^(TestIndependentImportDistance|TestImportDataset|TestImportNativeCommittedResumeAndVerify$|TestImportExactDistancesAgainstIndependentHighPrecision$|TestImportReviewIndependentPKBitsAndCanonicalHash$)' -count=1 -v
step go test ./cmd/gosqlite -run '^(TestMigrateImportDatasetDistancesCLI|TestIndependentMigrateImportDistanceCLIReportsFailure)$' -count=1 -v
step go test ./extension/vector -run '^(TestVectorPublishedPGVectorGolden|TestVectorPinnedFixtureHashes)$' -count=1
step go test ./migration/postgres -run '^(TestPinnedLegacySourceFiles|TestRealArrayFiniteBinary32|TestRealArrayRejects|TestReviewRealArrayIndependentBinary32)$' -count=1
step go vet ./migration/postgres ./cmd/gosqlite
step go build -trimpath ./...
go run ./examples/pgembedding-import > "$R/evidence/example.json"
printf '%s\n' '{"column":"embedding","query_count":4,"top_k":10}' > "$R/evidence/options.json"
python3 - "$R/evidence" <<'PY'
import json,sys,subprocess,pathlib
p=pathlib.Path(sys.argv[1]);rows=json.loads((p/'example.json').read_text());assert len(rows)==2
for row in rows:
 assert row['Verified']['status']=='complete_verified'
 assert row['HNSWRecallAt4']==1 and row['IVFRecallAt4']==1
 assert 'exact=false' in row['HNSWPlan'] and 'exact=false' in row['IVFPlan']
 out=p/(row['Format']+'-distances.json')
 with out.open('w') as file:subprocess.run(['go','run','./cmd/gosqlite','migrate','pgembedding','verify','--input',row['Input'],'--manifest',row['Manifest'],'--output',row['Database'],'--distances',str(p/'options.json')],stdout=file,check=True)
 report=json.loads(out.read_text());r=report['distance_verification']
 assert report['import']['status']=='complete_verified' and r['status']=='passed'
 assert r['candidate_rows']==r['norms_checked']==r['native_rows_checked']==64
 assert r['distances_checked']==1024 and r['top_k_checked']==160 and len(r['queries'])==16
 assert r['query_coverage']=='sampled_queries_all_candidates'
 assert r['ann_recall'].startswith('not_run') and report['import']['ann_recall'].startswith('not_run')
 assert all(q['native_exact'] and len(q['all'])==64 and len(q['top_k'])==10 and q['top_k']==q['all'][:10] for q in r['queries'])
 print(row['Format']+' example + reusable CLI PASS: 64 norms, 1024 full distances, 160 separately executed Top-K; verifier ANN not_run')
PY
step go run ./internal/testscope
step env CGO_ENABLED=1 go test -race ./migration/postgres -run '^(TestIndependentImportDistance|TestImportDataset|TestImportNativeCommittedResumeAndVerify$|TestImportExactDistancesAgainstIndependentHighPrecision$|TestImportReviewIndependentPKBitsAndCanonicalHash$)' -count=1 -timeout=10m
step env CGO_ENABLED=1 go test -race ./cmd/gosqlite -run '^(TestMigrateImportDatasetDistancesCLI|TestIndependentMigrateImportDistanceCLIReportsFailure)$' -count=1 -timeout=10m
go list -deps -f '{{if .CgoFiles}}{{.ImportPath}}{{end}}' ./cmd/gosqlite ./migration/postgres > "$R/evidence/cgo-closure.txt"
test ! -s "$R/evidence/cgo-closure.txt"
echo 'INDEPENDENT SCOPED NORMAL/RACE/CLI/EXAMPLE CHECKS PASS'
date -u +%FT%TZ
