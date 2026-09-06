package db

import (
	"path/filepath"
	"testing"
)

func TestResolvePath(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{":memory:", ":memory:", false},                                  // bare, back-compat
		{"./pushport.db", "./pushport.db", false},                        // bare relative path
		{"sqlite://./pushport.db", "./pushport.db", false},               // relative URI
		{"sqlite:///var/lib/pushport.db", "/var/lib/pushport.db", false}, // absolute URI
		{"sqlite://./x.db?_foo=1", "./x.db?_foo=1", false},               // query preserved
		{"postgres://localhost/db", "", true},                            // unsupported scheme
		{"sqlite://host/x.db", "", true},                                 // invalid host (not "" or ".")
		{"sqlite://", "", true},                                          // no path
	}
	for _, c := range cases {
		got, err := resolvePath(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolvePath(%q): want error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolvePath(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolvePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestOpenWithSQLiteURI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pushport.db")
	d, err := Open("sqlite://" + path)
	if err != nil {
		t.Fatalf("open via sqlite uri: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	var n int
	if err := d.sql.QueryRow("SELECT count(*) FROM app").Scan(&n); err != nil {
		t.Fatalf("query after open: %v", err)
	}
}
