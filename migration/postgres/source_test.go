package postgres

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func TestPinnedLegacySourceFiles(t *testing.T) {
	files := map[string]string{
		"embedding--0.3.6.sql": "e167cfc2ffe6ac29aa8a6984441725f02a6351adc2c823b3bc0ffe6b470c7e22",
		"embedding.control":    "68ec6fb5703a1fa5bdad19d3a65d4d43a9779364c974cb410c533d41dc6f2c7b",
		"LICENSE":              "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30",
		"NOTICE":               "0cb583690f00b0486d8d807a455a80401b90780a769e1458125252adf7b2cfc3",
	}
	for name, want := range files {
		b, e := os.ReadFile("../../testdata/pgembedding-0.3.6/" + name)
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(b)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Fatalf("pinned source %s hash changed: %s", name, got)
		}
	}
}
