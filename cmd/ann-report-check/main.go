// Command ann-report-check checks one schema3 ANN report from standard input.
// It neither runs benchmarks nor opens paths named inside a report.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/benchmarks/ann/reportcheck"
)

type result struct {
	Status               string `json:"status"`
	SelfConsistencyValid bool   `json:"self_consistency_valid"`
	PerformanceAccepted  bool   `json:"performance_accepted"`
	Error                string `json:"error,omitempty"`
}

// readReport consumes at most limit+1 bytes. The extra byte distinguishes an
// exactly bounded report from an oversized stream without reading to its end.
func readReport(in io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(in, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("report exceeds 256 MiB input limit")
	}
	return b, nil
}

func writeResult(out io.Writer, errout io.Writer, r result, code int) int {
	if err := json.NewEncoder(out).Encode(r); err != nil {
		fmt.Fprintln(errout, "cannot write validation result:", err)
		return 1
	}
	return code
}

// run is reader/writer-only so command-contract tests need no files/processes.
func run(args []string, in io.Reader, out io.Writer, errout io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(errout, "usage: ann-report-check < report.json")
		return 2
	}
	b, err := readReport(in, reportcheck.MaxReportBytes)
	if err != nil {
		return writeResult(out, errout, result{Status: "INVALID", Error: "read report: " + err.Error()}, 1)
	}
	if _, err = reportcheck.DecodeReport(b); err != nil {
		return writeResult(out, errout, result{Status: "INVALID", Error: err.Error()}, 1)
	}
	// VALID is self-consistency only. Never infer performance acceptance from
	// caller-supplied JSON, even when all digests and counters are consistent.
	return writeResult(out, errout, result{Status: "VALID", SelfConsistencyValid: true}, 0)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
