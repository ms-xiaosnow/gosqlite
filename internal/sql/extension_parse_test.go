package sql

import "testing"

func TestExtensionGrammar(t *testing.T) {
	s, err := Parse(`CREATE INDEX IF NOT EXISTS words ON docs(title,body) USING grams13 WITH (case_fold='unicode-full-default', max_input_bytes=123, nfc=TRUE)`)
	if err != nil {
		t.Fatal(err)
	}
	c := s.(*CreateIndex)
	if c.Method != "grams13" || !c.IfNotExists || c.Options["max_input_bytes"] != int64(123) || c.Options["nfc"] != int64(1) {
		t.Fatal(c)
	}
	r, err := Parse(`REINDEX words`)
	if err != nil || r.(*Reindex).Name != "words" {
		t.Fatal(r, err)
	}
	q, err := Parse(`EXPLAIN QUERY PLAN SELECT id FROM docs AS d NOT INDEXED WHERE d.title LIKE ? ESCAPE ?`)
	if err != nil {
		t.Fatal(err)
	}
	sel := q.(*Select)
	if !sel.Explain || !sel.NotIndexed || sel.ParameterCount() != 2 {
		t.Fatal(sel)
	}
	for _, bad := range []string{
		`CREATE INDEX words ON docs(title) USING grams13 WITH (x=1,X=2)`,
		`CREATE INDEX words ON docs(title) USING grams13 WITH (x=?)`,
		`CREATE INDEX words ON docs(title) USING grams13 WITH (x=1+2)`,
		`CREATE INDEX words ON docs(title) USING grams13 WITH (x=lower('ABC'))`,
		`CREATE INDEX words ON docs(title) WITH (x=1)`,
		`EXPLAIN SELECT id FROM docs`, `EXPLAIN QUERY PLAN DELETE FROM docs`, `REINDEX`,
		`SELECT 1 NOT INDEXED`, `SELECT id FROM docs INDEXED BY words`,
	} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
