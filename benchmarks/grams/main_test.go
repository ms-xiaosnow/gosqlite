package main

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func testConfig() config {
	return config{Rows: 64, Queries: 2, Repeats: 2, Seed: 2026101052, PageSize: 4096, MaxPages: 4096, SourceID: "unit-test-source", Environment: "ordinary native unit test; timings not benchmark evidence"}
}
func TestCorpusDeterministicNFCAndMarkers(t *testing.T) {
	c := testConfig()
	a, hash, bytes := makeCorpus(c)
	b, again, againBytes := makeCorpus(c)
	if !reflect.DeepEqual(a, b) || hash != again || bytes != againBytes {
		t.Fatal("nondeterministic corpus")
	}
	const want = "849da59e3ab52ca6411b1df4d7141a5877d85bee3bdabca91edc9266dab395a6"
	if hash != want {
		t.Fatalf("corpus identity changed: %s", hash)
	}
	total := int64(0)
	for _, s := range a.Texts {
		if !norm.NFC.IsNormalString(s) {
			t.Fatalf("not NFC: %q", s)
		}
		total += int64(len(s))
	}
	if total != bytes || len(a.Queries) != 3*c.Queries+2 {
		t.Fatal("corpus accounting")
	}
	for n := 1; n <= 3; n++ {
		for g := 0; g < 32; g++ {
			if utf8.RuneCountInString(marker(n, g)) != n {
				t.Fatal("marker is not rune-counted")
			}
		}
	}
	c.Seed++
	_, changed, _ := makeCorpus(c)
	if changed == hash {
		t.Fatal("seed ignored")
	}
	c = testConfig()
	c.Queries++
	_, changed, _ = makeCorpus(c)
	if changed == hash {
		t.Fatal("query list omitted from identity")
	}
}
func TestEmpiricalDistributionAndUncertainty(t *testing.T) {
	input := []float64{5, 1, 4, 2, 3}
	got := summarize(input)
	if got.N != 5 || got.P50 != 3 || got.P95 != 5 || got.Min != 1 || got.Max != 5 || got.Mean != 3 || math.Abs(got.SampleStddev-math.Sqrt(2.5)) > 1e-12 || input[0] != 5 || got.CILow != nil || got.CIHigh != nil || got.ConfidenceMethod == "" {
		t.Fatal(got, input)
	}
	if one := summarize([]float64{7}); one.N != 1 || one.P50 != 7 || one.P95 != 7 || one.SampleStddev != 0 {
		t.Fatal(one)
	}
	if empty := summarize(nil); empty.N != 0 {
		t.Fatal(empty)
	}
}
func TestBoundedConfiguration(t *testing.T) {
	if e := testConfig().validate(); e != nil {
		t.Fatal(e)
	}
	cases := []func(*config){func(c *config) { c.Rows = 63 }, func(c *config) { c.Rows = 20001 }, func(c *config) { c.Queries = 0 }, func(c *config) { c.Queries = 129 }, func(c *config) { c.Repeats = 1 }, func(c *config) { c.Repeats = 11 }, func(c *config) { c.PageSize = 1024 }, func(c *config) { c.MaxPages = 4095 }, func(c *config) { c.MaxPages = 65537 }, func(c *config) { c.SourceID = "" }, func(c *config) { c.Environment = "" }}
	for i, edit := range cases {
		c := testConfig()
		edit(&c)
		if e := c.validate(); e == nil {
			t.Fatalf("accepted invalid case %d", i)
		}
	}
	c := testConfig()
	c.CorpusOnly = true
	c.SourceID = ""
	c.Environment = ""
	if e := c.validate(); e != nil {
		t.Fatal(e)
	}
}
func TestNativePersistedGramsBenchmarkSmoke(t *testing.T) {
	r, err := run(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "pass" || r.SchemaVersion != 1 || len(r.Trials) != 2 || r.BuildDistribution.N != 2 || r.DatasetSHA256 == "" || r.SourceTextBytes == 0 || r.Finished == "" || r.ElapsedSeconds <= 0 || len(r.Errors) != 0 {
		t.Fatalf("incomplete report: %+v", r)
	}
	for _, trial := range r.Trials {
		if trial.BuildMS <= 0 || trial.BeforeBytes <= 0 || trial.AfterBytes <= trial.BeforeBytes || trial.IndexBytes != trial.AfterBytes-trial.BeforeBytes || !trial.IntegrityBeforeClose || !trial.IntegrityAfterReopen || len(trial.Samples) != 8 || len(trial.DatabaseSHA256) != 64 {
			t.Fatalf("invalid persisted-build accounting: %+v", trial)
		}
		for _, s := range trial.Samples {
			if s.Automatic.N != 2 || s.Scan.N != 2 || s.MembershipComparisons != 128 {
				t.Fatal(s)
			}
			if s.Length == 0 {
				if s.SearchCount != 0 || s.FallbackCount != 2 {
					t.Fatal(s)
				}
			} else if s.SearchCount != 2 || s.FallbackCount != 0 {
				t.Fatal(s)
			}
			for _, o := range s.Observations {
				if !o.Equal || o.ScanWork.BaseRowFetches != 64 || o.ResultSHA256 == "" {
					t.Fatal(o)
				}
			}
		}
	}
	if _, err = json.Marshal(r); err != nil {
		t.Fatal(err)
	}
}
