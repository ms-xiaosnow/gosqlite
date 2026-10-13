package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// A mechanical, explicitly SYNTHETIC schema3 example, not a measured run.
// Embedding performs no file access in the test's application call closure.
//
//go:embed testdata/synthetic-schema3-hnsw.json
var syntheticSchema3 []byte

func TestANNReportCheckSyntheticStdinRoundTrip(t *testing.T) {
	var out, errout bytes.Buffer
	if code := run(nil, bytes.NewReader(syntheticSchema3), &out, &errout); code != 0 {
		t.Fatal("synthetic schema3 rejected", code, out.String(), errout.String())
	}
	if out.String() != "{\"status\":\"VALID\",\"self_consistency_valid\":true,\"performance_accepted\":false}\n" || errout.Len() != 0 {
		t.Fatal("self-consistency only", out.String(), errout.String())
	}
	// The same complete wire input must fail after a material status change.
	changed := bytes.Replace(syntheticSchema3, []byte(`"status":"passed"`), []byte(`"status":"incomplete"`), 1)
	if bytes.Equal(changed, syntheticSchema3) {
		t.Fatal("status mutation absent")
	}
	out.Reset()
	if code := run(nil, bytes.NewReader(changed), &out, &errout); code != 1 || !bytes.Contains(out.Bytes(), []byte(`"status":"INVALID"`)) {
		t.Fatal("incomplete synthetic schema3 admitted", code, out.String())
	}
}

func TestANNReportCheckInvalidStdin(t *testing.T) {
	for _, input := range []string{"", "{}", `{"schema_version":2}`, `{"schema_version":3,"status":"incomplete"}`} {
		var out, errout bytes.Buffer
		if code := run(nil, strings.NewReader(input), &out, &errout); code != 1 {
			t.Fatal("invalid report exit", code)
		}
		var r result
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		if r.Status != "INVALID" || r.SelfConsistencyValid || r.PerformanceAccepted || r.Error == "" || errout.Len() != 0 {
			t.Fatal("invalid result contract", r, errout.String())
		}
	}
}

func TestANNReportCheckUsageDoesNotRead(t *testing.T) {
	var out, errout bytes.Buffer
	if code := run([]string{"report.json"}, neverReader{t}, &out, &errout); code != 2 || out.Len() != 0 || !strings.Contains(errout.String(), "usage:") {
		t.Fatal("usage contract", code, out.String(), errout.String())
	}
}

type neverReader struct{ t *testing.T }

func (r neverReader) Read([]byte) (int, error) {
	r.t.Fatal("unexpected stdin read")
	return 0, io.EOF
}

func TestANNReportCheckBoundedRead(t *testing.T) {
	input := strings.NewReader("123456789")
	if _, err := readReport(input, 4); err == nil || input.Len() != 4 {
		t.Fatal("must stop after limit+1 bytes", err, input.Len())
	}
	b, err := readReport(strings.NewReader("1234"), 4)
	if err != nil || string(b) != "1234" {
		t.Fatal("exact bound rejected", string(b), err)
	}
}

type brokenIO struct{}

func (brokenIO) Read([]byte) (int, error)  { return 0, errors.New("synthetic read error") }
func (brokenIO) Write([]byte) (int, error) { return 0, errors.New("synthetic write error") }

func TestANNReportCheckIOFailures(t *testing.T) {
	var out, errout bytes.Buffer
	if code := run(nil, brokenIO{}, &out, &errout); code != 1 || !strings.Contains(out.String(), "synthetic read error") {
		t.Fatal("read error contract", code, out.String())
	}
	if code := run(nil, strings.NewReader("{}"), brokenIO{}, &errout); code != 1 || !strings.Contains(errout.String(), "cannot write validation result") {
		t.Fatal("write error contract", code, errout.String())
	}
}

func TestANNReportCheckValidResultIsNotPerformanceAcceptance(t *testing.T) {
	var out, errout bytes.Buffer
	if code := writeResult(&out, &errout, result{Status: "VALID", SelfConsistencyValid: true}, 0); code != 0 {
		t.Fatal(code)
	}
	if out.String() != "{\"status\":\"VALID\",\"self_consistency_valid\":true,\"performance_accepted\":false}\n" || errout.Len() != 0 {
		t.Fatal(out.String(), errout.String())
	}
}
