// Package reportcheck validates schema3 ANN report self-consistency offline.
// It does not run measurements or attest source, host, files, recall or speed.
package reportcheck

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// generateFixture is only deterministic in-memory seed/GSV hashing. It performs
// no native build/search, database operation, measurement, or file access.
func generateFixture(c Config, retain bool) (data, queries []vector.Vector, dh, qh, combined string, err error) {
	rng := rand.New(rand.NewSource(c.Seed))
	all := sha256.New()
	makeSet := func(n int) ([]vector.Vector, string, error) {
		var out []vector.Vector
		if retain {
			out = make([]vector.Vector, n)
		}
		h := sha256.New()
		for i := 0; i < n; i++ {
			v := make([]float32, c.Dimensions)
			for j := range v {
				v[j] = float32(rng.NormFloat64())
			}
			x, e := vector.New(v)
			if e != nil {
				return nil, "", e
			}
			b, e := x.MarshalBinary()
			if e != nil {
				return nil, "", e
			}
			_, _ = h.Write(b)
			_, _ = all.Write(b)
			if retain {
				out[i] = x
			}
		}
		return out, hex.EncodeToString(h.Sum(nil)), nil
	}
	data, dh, err = makeSet(c.Rows)
	if err != nil {
		return
	}
	queries, qh, err = makeSet(c.Queries)
	combined = hex.EncodeToString(all.Sum(nil))
	return
}
