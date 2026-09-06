package types

import (
	"database/sql"
	"testing"
)

func TestNewNullString(t *testing.T) {
	fromStr := NewNullString("x")
	if !fromStr.Valid || fromStr.String != "x" {
		t.Errorf("NewNullString: want {x true}, got %+v", fromStr)
	}
	fromNil := NewNullString[*string](nil)
	if fromNil.Valid {
		t.Errorf("NewNullString(nil): want NULL, got %+v", fromNil)
	}
	p := "x"
	fromPtr := NewNullString(&p)
	if !fromPtr.Valid || fromPtr.String != "x" {
		t.Errorf("NewNullString: want {x true}, got %+v", fromPtr)
	}
}

func TestNullStringToStrPtr(t *testing.T) {
	zero := NullString{}
	if got := zero.ToStrPtr(); got != nil {
		t.Errorf("zero value: want nil, got %q", *got)
	}
	valid := NewNullString("x")
	if got := valid.ToStrPtr(); got == nil || *got != "x" {
		t.Errorf("valid value: want x, got %v", got)
	}
}

func TestNullStringDriverRoundTrip(t *testing.T) {
	v, err := NewNullString("x").Value()
	if err != nil || v != "x" {
		t.Fatalf("Value: want x, got %v err=%v", v, err)
	}
	empty := NullString{}
	if v, err := empty.Value(); err != nil || v != nil {
		t.Fatalf("Value(NULL): want nil, got %v err=%v", v, err)
	}
	var scanned NullString
	if err := scanned.Scan("x"); err != nil || !scanned.Valid || scanned.String != "x" {
		t.Fatalf("Scan: want {x true}, got %+v err=%v", scanned, err)
	}
	var nulled NullString
	if err := nulled.Scan(nil); err != nil || nulled.Valid {
		t.Fatalf("Scan(nil): want NULL, got %+v err=%v", nulled, err)
	}
	want := sql.NullString{String: "x", Valid: true}
	built := NewNullString("x")
	if got := built.NullString; got != want {
		t.Errorf("embedded field: got %+v", got)
	}
}
