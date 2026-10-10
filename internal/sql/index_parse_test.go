package sql

import "testing"

func TestIndexBinaryCollation(t *testing.T) {
	for _, query := range []string{"CREATE INDEX idx ON t(a COLLATE BINARY ASC,b)", "CREATE INDEX idx ON t(a COLLATE binary)"} {
		stmt, err := Parse(query)
		if err != nil {
			t.Fatal(err)
		}
		c := stmt.(*CreateIndex)
		if c.Columns[0] != "a" || c.Desc[0] {
			t.Fatal(c)
		}
	}
	for _, query := range []string{"CREATE INDEX idx ON t(a COLLATE NOCASE)", "CREATE INDEX idx ON t(a COLLATE RTRIM)", "CREATE INDEX idx ON t(a COLLATE)", "SELECT a COLLATE BINARY FROM t"} {
		if _, err := Parse(query); err == nil {
			t.Fatalf("accepted unsupported %s", query)
		}
	}
}
