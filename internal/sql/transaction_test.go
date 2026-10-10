package sql

import "testing"

func TestTransactionControls(t *testing.T) {
	for _, tc := range []struct{ query, action, mode, name string }{
		{"BEGIN", "BEGIN", "", ""}, {"BEGIN DEFERRED TRANSACTION", "BEGIN", "DEFERRED", ""},
		{"BEGIN EXCLUSIVE", "BEGIN", "EXCLUSIVE", ""}, {"END TRANSACTION", "COMMIT", "", ""},
		{"SAVEPOINT inner", "SAVEPOINT", "", "inner"}, {"SAVEPOINT begin", "SAVEPOINT", "", "begin"}, {"SAVEPOINT window", "SAVEPOINT", "", "window"}, {"SAVEPOINT repeat", "SAVEPOINT", "", "repeat"}, {`SAVEPOINT "a b"`, "SAVEPOINT", "", "a b"},
		{"SAVEPOINT 'a''b'", "SAVEPOINT", "", "a'b"}, {"RELEASE repeat", "RELEASE", "", "repeat"},
		{"RELEASE SAVEPOINT [a b]", "RELEASE", "", "a b"},
		{"ROLLBACK TRANSACTION", "ROLLBACK", "", ""}, {"ROLLBACK TO repeat", "ROLLBACK TO", "", "repeat"},
		{"ROLLBACK TRANSACTION TO SAVEPOINT `a b`;", "ROLLBACK TO", "", "a b"},
	} {
		stmt, err := Parse(tc.query)
		if err != nil {
			t.Fatal(tc.query, err)
		}
		got, ok := stmt.(*Transaction)
		if !ok || got.Action != tc.action || got.Mode != tc.mode || got.Name != tc.name || got.ParameterCount() != 0 {
			t.Fatalf("%s => %#v", tc.query, stmt)
		}
	}
	for _, query := range []string{"SAVEPOINT", "SAVEPOINT ?", "RELEASE", "RELEASE SAVEPOINT", "ROLLBACK TO", "ROLLBACK TRANSACTION TO SAVEPOINT", "ROLLBACK TO x extra", "RELEASE x; COMMIT", "BEGIN EXCLUSIVE IMMEDIATE", "SAVEPOINT SELECT", "SAVEPOINT TO", "SAVEPOINT JOIN"} {
		if _, err := Parse(query); err == nil {
			t.Fatal("accepted", query)
		}
	}
}
