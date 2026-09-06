package types

import "testing"

func TestNewNullInt64(t *testing.T) {
	fromInt := NewNullInt64(42)
	if !fromInt.Valid || fromInt.Int64 != 42 {
		t.Errorf("int: want {42 true}, got %+v", fromInt)
	}
	fromInt64 := NewNullInt64(int64(42))
	if !fromInt64.Valid || fromInt64.Int64 != 42 {
		t.Errorf("int64: want {42 true}, got %+v", fromInt64)
	}
	if got := NewNullInt64((*int)(nil)); got.Valid {
		t.Errorf("*int nil: want NULL, got %+v", got)
	}
	if got := NewNullInt64((*int64)(nil)); got.Valid {
		t.Errorf("*int64 nil: want NULL, got %+v", got)
	}
	n := 7
	fromPtr := NewNullInt64(&n)
	if !fromPtr.Valid || fromPtr.Int64 != 7 {
		t.Errorf("*int: want {7 true}, got %+v", fromPtr)
	}
	n64 := int64(9)
	fromPtr64 := NewNullInt64(&n64)
	if !fromPtr64.Valid || fromPtr64.Int64 != 9 {
		t.Errorf("*int64: want {9 true}, got %+v", fromPtr64)
	}
}

func TestNullInt64DriverRoundTrip(t *testing.T) {
	v, err := NewNullInt64(42).Value()
	if err != nil || v != int64(42) {
		t.Fatalf("Value: want 42, got %v err=%v", v, err)
	}
	empty := NullInt64{}
	if v, err := empty.Value(); err != nil || v != nil {
		t.Fatalf("Value(NULL): want nil, got %v err=%v", v, err)
	}
	var scanned NullInt64
	if err := scanned.Scan(int64(7)); err != nil || !scanned.Valid || scanned.Int64 != 7 {
		t.Fatalf("Scan: want {7 true}, got %+v err=%v", scanned, err)
	}
	var nulled NullInt64
	if err := nulled.Scan(nil); err != nil || nulled.Valid {
		t.Fatalf("Scan(nil): want NULL, got %+v err=%v", nulled, err)
	}
}
