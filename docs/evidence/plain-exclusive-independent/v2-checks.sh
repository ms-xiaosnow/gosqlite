#!/usr/bin/env bash
set -euo pipefail
root=/workspace/scratch/5e06a74e5f65/durable-restart
cd "$root/plain-exclusive-independent-v2"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$root/go-cache" GOPATH="$root/go-path" SQLITE_ORACLE="$root/toolchains/sqlite-build-3540000/sqlite3"
go="$root/toolchains/go1.27.2/bin/go"
selection='^(TestExclusiveIndependent|TestPlainSessionCleanupMemory|TestPlainTxCleanupMemory|TestPlainCleanupMemory|TestPlaintextCleanup|TestPathAbsenceMemory|TestCloseCleanupMemoryDBInvalidatesOnceAndRetries$|TestSessionDescriptorLeaseAndOrdinaryRetainedClose$|TestSessionCommitProtectsSectorNeighbors$|TestSessionReopenedWALNormalizationPreservesSnapshot$|TestWALLogicalHeaderExtentInMemory$|TestWALSectorsBoundedBeforeReadInMemory$|TestNativeWALConnectionLifecycle$|TestWALOptionsAndCanceledOpen$)'
regression='^(TestSharedWALSnapshotsPromotionAndCheckpoint|TestEncryptionOrdinaryLifecycleAndCopies|TestDriverSavepointsPoolResetAndOuterRelease|TestDriverFailedOuterReleaseRemainsActive|TestDriverDeferredBeginSeesFirstAccessState|TestWritableCommitBusyRollsBackForPool|TestWALConnectorSingleConnection|TestEncryptedConnectorOrdinaryLifecycle)$'
set -x
date -u +%FT%TZ
"$go" version
for cgo in 0 1; do
 export CGO_ENABLED="$cgo"
 args=(); if [ "$cgo" = 1 ]; then args=(-race); fi
 "$go" run ./internal/testscope
 "$go" test "${args[@]}" ./internal/pager ./internal/vfs ./sqlite ./driver -run "$selection" -count=1
 "$go" run ./internal/testscope
 "$go" test "${args[@]}" ./internal/pager ./sqlite ./driver -run "$regression" -count=1
done
export CGO_ENABLED=0
"$go" run ./internal/testscope
"$go" test -tags oracle ./internal/testkit -run '^(TestOracleExclusiveWALNormalInterop|TestOracleImportedWALCheckpointWithoutNativeWrite)$' -count=1
"$go" build ./...
"$go" vet ./internal/pager ./internal/vfs ./sqlite ./driver
"$go" list -deps -f '{{if .CgoFiles}}{{.ImportPath}}: {{.CgoFiles}}{{end}}' ./... > "$root/gosqlite/docs/evidence/plain-exclusive-independent/v2-production-cgo.txt"
for target in darwin windows; do
 "$go" run ./internal/testscope
 for pkg in internal/pager sqlite driver; do
  GOOS="$target" GOARCH=amd64 "$go" test -c -o "/tmp/plain-exclusive-independent-v2-$target-${pkg##*/}.test" "./$pkg"
 done
done
date -u +%FT%TZ
