package reportcheck

import (
	"math"
	"strings"
	"testing"
	"time"
)

func modelConfig() Config {
	c := defaults()
	c.SourceID = strings.Repeat("a", 40)
	c.HardwareID = "descriptor-free-model"
	return c
}
func TestANNAdmissionCartesian(t *testing.T) {
	c := modelConfig()
	a, e := c.admission()
	if e != nil || a.TotalComponentWork == 0 {
		t.Fatalf("default admission: %+v %v", a, e)
	}
	tests := map[string]func(*Config){"rows": func(c *Config) { c.Rows = 99 }, "dimensions": func(c *Config) { c.Dimensions = 2001 }, "queries": func(c *Config) { c.Queries = 0 }, "method": func(c *Config) { c.Method = "unknown" }, "source": func(c *Config) { c.SourceID = "working-tree" }, "hardware": func(c *Config) { c.HardwareID = " " }, "ivf flags hnsw": func(c *Config) { c.Probes = 2 }, "time": func(c *Config) { c.MaxDuration = time.Nanosecond }, "memory scalar": func(c *Config) { c.MaxMemoryBytes = 1 }, "disk scalar": func(c *Config) { c.MaxDataBytes = 1 }, "work scalar": func(c *Config) { c.MaxWork = math.MaxUint64 }, "memory product": func(c *Config) { c.Queries = 10000; c.MaxWork = 1e12; c.MaxMemoryBytes = 64 << 20 }, "disk product": func(c *Config) { c.MaxDataBytes = 1 << 20 }, "work product": func(c *Config) { c.MaxWork = a.TotalComponentWork - 1 }, "all scalar maxima": func(c *Config) {
		c.Rows = 100000
		c.Dimensions = 2000
		c.Queries = 10000
		c.MaxWork = 1e12
		c.MaxMemoryBytes = 1 << 30
		c.MaxDataBytes = 1 << 30
	}}
	for name, f := range tests {
		t.Run(name, func(t *testing.T) {
			v := c
			f(&v)
			if _, e := v.admission(); e == nil {
				t.Fatal("invalid Cartesian config accepted")
			}
		})
	}
	c.Method = "ivfflat"
	c.Lists = 8
	c.Probes = 6
	c.Rows = 256
	c.Dimensions = 4
	c.Queries = 2
	if _, e := c.admission(); e != nil {
		t.Fatal(e)
	}
	for _, f := range []func(*Config){func(c *Config) { c.Lists = 0 }, func(c *Config) { c.Lists = c.Rows + 1 }, func(c *Config) { c.Probes = c.Lists + 1 }, func(c *Config) { c.MaxProbes = c.Probes - 1 }} {
		v := c
		f(&v)
		if _, e := v.admission(); e == nil {
			t.Fatal("invalid IVF admission accepted")
		}
	}
}
func TestANNCheckedArithmetic(t *testing.T) {
	if _, e := checkedProduct(math.MaxUint64, 2); e == nil {
		t.Fatal("overflow")
	}
	if _, e := checkedSum(math.MaxUint64, 1); e == nil {
		t.Fatal("overflow")
	}
	if n, e := checkedProduct(100, 2000, 10000); e != nil || n != 2e9 {
		t.Fatalf("product %d %v", n, e)
	}
	if n, e := checkedProduct(math.MaxUint64, 0); e != nil || n != 0 {
		t.Fatal(n, e)
	}
}
func TestANNDatasetCompatibility(t *testing.T) {
	c := modelConfig()
	_, _, dh, qh, all, e := generateFixture(c, true)
	if e != nil {
		t.Fatal(e)
	}
	if all != "81aec74dd6099591042ec104f4453cd0bda864aa57a06e5077c2142bedd280a0" {
		t.Fatalf("historical workload changed: %s", all)
	}
	_, _, dh2, qh2, all2, e := generateFixture(c, true)
	if e != nil || dh != dh2 || qh != qh2 || all != all2 || dh == qh {
		t.Fatal("nondeterministic")
	}
	c.Seed++
	_, _, _, _, changed, e := generateFixture(c, true)
	if e != nil || changed == all {
		t.Fatal("seed missing from identity")
	}
}
func TestANNAlgorithmDefaults(t *testing.T) {
	c := modelConfig()
	h, e := algorithm(c)
	if e != nil || h.M != 16 || h.EFConstruction != 64 || h.Seed != 0x243f6a8885a308d3 {
		t.Fatal(h, e)
	}
	c.Method = "ivfflat"
	v, e := algorithm(c)
	if e != nil || v.Lists != 100 || v.TrainingIterations != 20 || v.TrainingSamples != 10000 || v.M != 0 {
		t.Fatal(v, e)
	}
}

func TestANNIndependentCartesianBudgets(t *testing.T) {
	c := modelConfig()
	c.Queries = 1000
	c.MaxMemoryBytes = 64 << 20
	c.MaxDataBytes = 1 << 30
	c.MaxWork = 1e12
	a, e := c.admission()
	if e == nil || a.EstimatedMemoryBytes <= c.MaxMemoryBytes || a.EstimatedDataBytes > c.MaxDataBytes || a.TotalComponentWork > c.MaxWork || a.EstimatedReportNodes > maxReportNodes {
		t.Fatal("memory product isolation", a, e)
	}
	c = modelConfig()
	c.Rows = 256
	c.Dimensions = 4
	c.Queries = 3000
	c.MaxMemoryBytes = 1 << 30
	c.MaxDataBytes = 1 << 30
	c.MaxWork = 1e12
	a, e = c.admission()
	if e == nil || !strings.Contains(e.Error(), "report wire shape") || a.EstimatedReportNodes <= maxReportNodes {
		t.Fatal("wire product admission", a, e)
	}
	c = modelConfig()
	c.Method = "ivfflat"
	c.Probes = c.Lists
	if _, e = c.admission(); e == nil {
		t.Fatal("known all-exact request admitted")
	}
}
