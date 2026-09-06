package crypto

import (
	"bytes"
	"errors"
	"testing"
)

func k32(b byte) []byte {
	s := make([]byte, 32)
	for i := range s {
		s[i] = b
	}
	return s
}

func TestNewKeyringDuplicateSecretIsBenign(t *testing.T) {
	s := k32(7)
	kr, err := NewKeyring([][]byte{s, s})
	if err != nil {
		t.Fatalf("identical duplicate secret should be accepted, got %v", err)
	}
	blob, err := kr.Seal([]byte("hi"), nil)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if pt, err := kr.Open(blob, nil); err != nil || !bytes.Equal(pt, []byte("hi")) {
		t.Fatalf("round trip failed: pt=%q err=%v", pt, err)
	}
}

func TestKeyringSealOpenRoundTrip(t *testing.T) {
	kr, err := NewKeyring([][]byte{k32(1)})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	blob, err := kr.Seal([]byte("hi"), []byte("aad"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if id, _ := kr.MkidOf(blob); id != kr.PrimaryMkid() {
		t.Fatalf("mkid prefix mismatch")
	}
	pt, err := kr.Open(blob, []byte("aad"))
	if err != nil || !bytes.Equal(pt, []byte("hi")) {
		t.Fatalf("open: pt=%q err=%v", pt, err)
	}
	if _, err := kr.Open(blob, []byte("wrong")); err == nil {
		t.Fatal("open with wrong aad should fail")
	}
}

func TestKeyringOpensPreviousSecret(t *testing.T) {
	old, _ := NewKeyring([][]byte{k32(9)})
	blob, _ := old.Seal([]byte("secret"), nil)

	ring, _ := NewKeyring([][]byte{k32(1), k32(9)})
	pt, err := ring.Open(blob, nil)
	if err != nil || string(pt) != "secret" {
		t.Fatalf("should open old blob: pt=%q err=%v", pt, err)
	}
	if id, _ := ring.MkidOf(blob); id == ring.PrimaryMkid() {
		t.Fatal("old blob should not carry the new ring's primary mkid")
	}
	stray, _ := NewKeyring([][]byte{k32(7)})
	sblob, _ := stray.Seal([]byte("x"), nil)
	if _, err := ring.Open(sblob, nil); !errors.Is(err, ErrUnknownMkid) {
		t.Fatalf("want ErrUnknownMkid, got %v", err)
	}
}

func TestDeriveEndpointKey(t *testing.T) {
	kr, _ := NewKeyring([][]byte{k32(1)})
	mk := kr.PrimaryMkid()
	a, ok := kr.DeriveEndpointKey(mk, "app1", 1)
	if !ok || len(a) != 32 {
		t.Fatalf("derive: ok=%v len=%d", ok, len(a))
	}
	a2, _ := kr.DeriveEndpointKey(mk, "app1", 1)
	if !bytes.Equal(a, a2) {
		t.Fatal("derivation not deterministic")
	}
	b, _ := kr.DeriveEndpointKey(mk, "app2", 1)
	c, _ := kr.DeriveEndpointKey(mk, "app1", 2)
	if bytes.Equal(a, b) || bytes.Equal(a, c) {
		t.Fatal("derivation not separated by app/version")
	}
	if _, ok := kr.DeriveEndpointKey("deadbeef", "app1", 1); ok {
		t.Fatal("unknown mkid should return ok=false")
	}
}

func TestNewKeyringValidation(t *testing.T) {
	if _, err := NewKeyring(nil); err == nil {
		t.Fatal("empty should error")
	}
	if _, err := NewKeyring([][]byte{make([]byte, 16)}); err == nil {
		t.Fatal("non-32-byte should error")
	}
}
