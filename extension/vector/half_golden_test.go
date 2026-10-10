package vector_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// The files retain published expected outputs. No PostgreSQL process is used.
func TestExtendedPublishedPGVectorGolden(t *testing.T) {
	root := "../../testdata/pgvector-v0.8.7"
	data, err := os.ReadFile(filepath.Join(root, "extended/golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	type source struct{ Source, Statement string }
	var fixtures struct {
		Parse []struct {
			source
			Kind, Input   string
			Dimensions    *int
			Output, Error *string
		}
		Metrics []struct {
			source
			Kind, Operation, A string
			B, Output, Error   *string
			Policy             string
		}
		Quantize []struct {
			source
			Kind, Input   string
			Output, Error *string
		}
		Subvector []struct {
			source
			Kind, Input   string
			Start, Count  int32
			Output, Error *string
		}
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Parse) != 88 || len(fixtures.Metrics) != 93 || len(fixtures.Quantize) != 6 || len(fixtures.Subvector) != 20 {
		t.Fatal("fixture coverage changed without review")
	}
	cache := map[string]string{}
	checkSource := func(s source) {
		t.Helper()
		for _, name := range []string{s.Source, strings.TrimSuffix(s.Source, ".out") + ".sql"} {
			text, ok := cache[name]
			if !ok {
				b, err := os.ReadFile(filepath.Join(root, name))
				if err != nil {
					t.Fatal(err)
				}
				text = string(b)
				cache[name] = text
			}
			if !strings.Contains(text, s.Statement) {
				t.Fatal("statement missing from retained source", s.Statement, name)
			}
		}
	}
	for _, f := range fixtures.Parse {
		t.Run(f.Statement, func(t *testing.T) {
			checkSource(f.source)
			var got string
			var err error
			if f.Kind == "halfvec" {
				var v vector.HalfVector
				if f.Dimensions == nil {
					v, err = vector.ParseHalf(f.Input)
				} else {
					v, err = vector.ParseHalfDims(f.Input, *f.Dimensions)
				}
				got = v.String()
			} else {
				var v vector.SparseVector
				if f.Dimensions == nil {
					v, err = vector.ParseSparse(f.Input)
				} else {
					v, err = vector.ParseSparseDims(f.Input, *f.Dimensions)
				}
				got = v.String()
			}
			if f.Error != nil {
				if err == nil {
					t.Fatal("accepted invalid published case", got)
				}
				return
			}
			if err != nil || f.Output == nil {
				t.Fatal(err)
			}
			// Compare normalized round-tripping text; PostgreSQL exponent/fixed
			// formatting thresholds can differ without a representation change.
			var want string
			if f.Kind == "halfvec" {
				want = mustHalfM9(t, *f.Output).String()
			} else {
				want = mustSparseM9(t, *f.Output).String()
			}
			if got != want {
				t.Fatal(got, want)
			}
		})
	}
	for _, f := range fixtures.Metrics {
		t.Run(f.Statement, func(t *testing.T) {
			checkSource(f.source)
			var got float64
			var err error
			if f.Kind == "halfvec" {
				a := mustHalfM9(t, f.A)
				b := a
				if f.B != nil {
					b = mustHalfM9(t, *f.B)
				}
				switch f.Operation {
				case "l2_distance":
					got, err = vector.HalfL2Distance(a, b)
				case "l1_distance":
					got, err = vector.HalfL1Distance(a, b)
				case "inner_product":
					got, err = vector.HalfInnerProduct(a, b)
				case "negative_inner_product":
					got, err = vector.HalfNegativeInnerProduct(a, b)
				case "cosine_distance":
					got, err = vector.HalfCosineDistance(a, b)
				case "norm":
					got, err = vector.HalfNorm(a)
				default:
					t.Fatal(f.Operation)
				}
			} else if f.Kind == "sparsevec" {
				a := mustSparseM9(t, f.A)
				b := a
				if f.B != nil {
					b = mustSparseM9(t, *f.B)
				}
				switch f.Operation {
				case "l2_distance":
					got, err = vector.SparseL2Distance(a, b)
				case "l1_distance":
					got, err = vector.SparseL1Distance(a, b)
				case "inner_product":
					got, err = vector.SparseInnerProduct(a, b)
				case "negative_inner_product":
					got, err = vector.SparseNegativeInnerProduct(a, b)
				case "cosine_distance":
					got, err = vector.SparseCosineDistance(a, b)
				case "norm":
					got, err = vector.SparseNorm(a)
				default:
					t.Fatal(f.Operation)
				}
			} else {
				a, e := vector.ParseBit(f.A)
				if e != nil {
					t.Fatal(e)
				}
				b, e := vector.ParseBit(*f.B)
				if e != nil {
					t.Fatal(e)
				}
				if f.Operation == "hamming_distance" {
					got, err = vector.HammingDistance(a, b)
				} else if f.Operation == "jaccard_distance" {
					got, err = vector.JaccardDistance(a, b)
				} else {
					t.Fatal(f.Operation)
				}
			}
			if f.Error != nil {
				if !errors.Is(err, vector.ErrDimensions) {
					t.Fatal("published dimension error", got, err)
				}
				return
			}
			if f.Policy == "zero_norm_error" {
				if !errors.Is(err, vector.ErrZeroNorm) {
					t.Fatal(got, err)
				}
				return
			}
			if err != nil || math.IsInf(got, 0) || math.IsNaN(got) || f.Output == nil {
				t.Fatal(got, err)
			}
			if f.Policy == "finite64_cosine" {
				if f.Kind != "sparsevec" || f.Operation != "cosine_distance" || f.A != "{1:3e38}/1" || f.B == nil || *f.B != f.A || got != 0 {
					t.Fatal("changed nonzero overflow-cosine fixture", got)
				}
				return
			}
			if f.Policy == "finite64_l1" {
				if f.Kind != "sparsevec" || f.Operation != "l1_distance" {
					t.Fatal("unreviewed L1 overflow policy")
				}
				a := mustSparseM9(t, f.A).Entries()
				b := mustSparseM9(t, *f.B).Entries()
				if len(a) != 1 || len(b) != 1 || a[0].Index != b[0].Index {
					t.Fatal("changed L1 overflow fixture")
				}
				want := math.Abs(float64(a[0].Value) - float64(b[0].Value))
				if got != want {
					t.Fatal(got, want)
				}
				return
			}
			if f.Policy == "finite64" {
				// Retained upstream float32 inner-product overflow is an explicit
				// difference, never a loose-tolerance pass. This one-component
				// product is exact in binary64 (two 24-bit significands).
				if f.Kind != "sparsevec" || f.Operation != "inner_product" {
					t.Fatal("unreviewed overflow policy")
				}
				a := mustSparseM9(t, f.A).Entries()
				b := mustSparseM9(t, *f.B).Entries()
				if len(a) != 1 || len(b) != 1 || a[0].Index != b[0].Index {
					t.Fatal("changed overflow fixture")
				}
				want := float64(a[0].Value) * float64(b[0].Value)
				if got != want {
					t.Fatal(got, want)
				}
				return
			}
			want, e := strconv.ParseFloat(*f.Output, 64)
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(got-want) > 1e-6+1e-5*math.Abs(want) {
				t.Fatal(got, want)
			}
		})
	}
	for _, f := range fixtures.Quantize {
		t.Run(f.Statement, func(t *testing.T) {
			checkSource(f.source)
			var v vector.BitVector
			var err error
			if f.Kind == "halfvec" {
				v, err = vector.HalfBinaryQuantize(mustHalfM9(t, f.Input))
			} else {
				v, err = vector.BinaryQuantize(must(t, f.Input))
			}
			if err != nil || f.Output == nil || v.String() != *f.Output {
				t.Fatal(v, err)
			}
		})
	}
	for _, f := range fixtures.Subvector {
		t.Run(f.Statement, func(t *testing.T) {
			checkSource(f.source)
			var got string
			var err error
			if f.Kind == "halfvec" {
				var v vector.HalfVector
				v, err = vector.HalfSubvector(mustHalfM9(t, f.Input), f.Start, f.Count)
				got = v.String()
			} else {
				var v vector.Vector
				v, err = vector.Subvector(must(t, f.Input), f.Start, f.Count)
				got = v.String()
			}
			if f.Error != nil {
				if !errors.Is(err, vector.ErrDimensions) {
					t.Fatal(got, err)
				}
				return
			}
			if err != nil || f.Output == nil || got != *f.Output {
				t.Fatal(got, err)
			}
		})
	}
}

func TestExtendedPublishedFixtureHashes(t *testing.T) {
	root := "../../testdata/pgvector-v0.8.7/extended"
	b, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Commit string
		Files  []struct {
			File, SHA256 string
			Bytes        int
		}
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Commit != "f37c13f68b57d2c3472b2214fbcff699d6d34876" || len(m.Files) != 6 {
		t.Fatal("unreviewed pinned source")
	}
	for _, f := range m.Files {
		b, err := os.ReadFile(filepath.Join(root, f.File))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		if len(b) != f.Bytes || hex.EncodeToString(sum[:]) != f.SHA256 {
			t.Fatal("fixture source changed", f.File)
		}
	}
}
