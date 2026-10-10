#!/bin/bash
set -euo pipefail
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOMAXPROCS=2
select='^(TestIndependentSegment.*|TestReplay.*InMemory|TestSQLiteRollbackFormatInMemory|TestRejectMalformedJournalBytesInMemory|FuzzReplay.*BytesOnly|FuzzIndependentSegmentStructured)$'
echo 'Pure-memory journal tests only. Independent source/scope audit recorded separately.'
go version
go run ./internal/testscope
go test ./internal/journal -run "$select" -count=1 -timeout=2m -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./internal/journal -run "$select" -count=1 -timeout=3m
go run ./internal/testscope
go vet ./internal/journal
go build ./internal/journal ./internal/pager
GOOS=linux GOARCH=386 go build ./internal/journal ./internal/pager
for target in linux/arm64 darwin/arm64 windows/amd64; do GOOS=${target%/*} GOARCH=${target#*/} go build ./internal/journal ./internal/pager; done
echo 'SELECTED NORMAL / RACE / VET / BUILDS PASS'
