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

func TestVectorPublishedPGVectorGolden(t *testing.T) {
	root := "../../testdata/pgvector-v0.8.7"
	data, err := os.ReadFile(filepath.Join(root, "golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Parse []struct {
			Statement, Input string
			Upstream, Error  *string
		}
		Metrics []struct {
			Statement, Operation, A string
			B, Upstream, Error      *string
			LocalPolicy             string `json:"local_policy"`
		}
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Parse) != 30 || len(fixtures.Metrics) != 31 {
		t.Fatal("fixture coverage changed without review")
	}
	sql, err := os.ReadFile(filepath.Join(root, "vector_type.sql"))
	if err != nil {
		t.Fatal(err)
	}
	published, err := os.ReadFile(filepath.Join(root, "vector_type.out"))
	if err != nil {
		t.Fatal(err)
	}
	checkSource := func(statement string) {
		t.Helper()
		if !strings.Contains(string(sql), statement) || !strings.Contains(string(published), statement) {
			t.Fatal("statement missing in retained upstream source", statement)
		}
	}
	for _, f := range fixtures.Parse {
		t.Run(f.Statement, func(t *testing.T) {
			checkSource(f.Statement)
			v, err := vector.Parse(f.Input)
			if f.Error != nil {
				if err == nil || v.Valid() {
					t.Fatal("accepted invalid upstream case")
				}
				return
			}
			if err != nil || f.Upstream == nil {
				t.Fatal(err)
			}
			// Formatting can differ; the published decimal must yield identical bits.
			want := must(t, *f.Upstream)
			if !sameBits(v, want) {
				t.Fatal(v, want)
			}
		})
	}
	for _, f := range fixtures.Metrics {
		t.Run(f.Statement, func(t *testing.T) {
			checkSource(f.Statement)
			a := must(t, f.A)
			b := a
			if f.B != nil {
				b = must(t, *f.B)
			}
			var got float64
			var err error
			refName := ""
			switch f.Operation {
			case "l2_distance":
				got, err = vector.L2Distance(a, b)
				refName = "l2"
			case "l1_distance":
				got, err = vector.L1Distance(a, b)
				refName = "l1"
			case "cosine_distance":
				got, err = vector.CosineDistance(a, b)
				refName = "cosine"
			case "inner_product":
				got, err = vector.InnerProduct(a, b)
				refName = "inner_product"
			case "negative_inner_product":
				got, err = vector.NegativeInnerProduct(a, b)
				refName = "negative_inner_product"
			case "vector_norm":
				got, err = vector.Norm(a)
				refName = "norm"
			default:
				t.Fatal("unknown operation")
			}
			if f.Error != nil {
				if !errors.Is(err, vector.ErrDimensions) {
					t.Fatal(err)
				}
				return
			}
			if f.LocalPolicy == "ErrZeroNorm" {
				if !errors.Is(err, vector.ErrZeroNorm) {
					t.Fatal(err)
				}
				return
			}
			if err != nil || math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatal(got, err)
			}
			var want float64
			if f.LocalPolicy == "finite_float64" {
				want = referenceMetrics(a, b)[refName]
			} else {
				if f.Upstream == nil {
					t.Fatal("missing expected value")
				}
				want, err = strconv.ParseFloat(*f.Upstream, 64)
				if err != nil {
					t.Fatal(err)
				}
			}
			if !closeFloat(got, want, 1e-7, 1e-7) {
				t.Fatalf("got %.17g, want %.17g", got, want)
			}
		})
	}
}

func TestVectorPinnedFixtureHashes(t *testing.T) {
	root := "../../testdata/pgvector-v0.8.7"
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tag, Commit string
		Files       []struct {
			File, SHA256 string
			Bytes        int
		}
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Tag != "v0.8.7" || m.Commit != "f37c13f68b57d2c3472b2214fbcff699d6d34876" {
		t.Fatal("baseline changed without review")
	}
	for _, f := range m.Files {
		b, err := os.ReadFile(filepath.Join(root, f.File))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(b)
		if len(b) != f.Bytes || hex.EncodeToString(h[:]) != f.SHA256 {
			t.Fatal("changed fixture", f.File)
		}
	}
	license, err := os.ReadFile("../../third_party/pgvector-LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(license)
	if hex.EncodeToString(h[:]) != "6bba9ebeb73e27477463b05e5ef1bf303bccbddb3db9bbc95905d351604d6a87" {
		t.Fatal("upstream license changed")
	}
}
