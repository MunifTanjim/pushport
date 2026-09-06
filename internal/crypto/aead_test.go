package crypto

import (
	"bytes"
	"testing"
)

func key32() []byte { return make([]byte, 32) }

func TestAEADSealOpenRoundTrip(t *testing.T) {
	pt := []byte("secret payload")
	aad := []byte("tenant:kid")
	sealed, err := AEADSeal(key32(), pt, aad)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	got, err := AEADOpen(key32(), sealed, aad)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatalf("roundtrip mismatch: %q", got)
	}
}

func TestAEADOpenFailsOnWrongAAD(t *testing.T) {
	sealed, _ := AEADSeal(key32(), []byte("x"), []byte("aad-a"))
	if _, err := AEADOpen(key32(), sealed, []byte("aad-b")); err == nil {
		t.Fatal("expected open to fail with wrong aad")
	}
}
