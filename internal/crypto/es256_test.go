package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
)

func TestSignES256VerifiesAndHasThreeParts(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	tok, err := SignES256(key, map[string]any{"alg": "ES256", "kid": "K1"}, map[string]any{"iss": "T1"})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("want 3 parts, got %d", len(parts))
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		t.Fatalf("sig decode: len=%d err=%v", len(sig), err)
	}
	signingInput := parts[0] + "." + parts[1]
	sum := sha256.Sum256([]byte(signingInput))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&key.PublicKey, sum[:], r, s) {
		t.Fatal("signature did not verify")
	}
}

func TestSignES256PinsAlgHeader(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	tok, err := SignES256(key, map[string]any{"alg": "HS256", "kid": "K9"}, map[string]any{"iss": "T1"})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("want 3 parts, got %d", len(parts))
	}
	hdrJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("header decode: %v", err)
	}
	var hdr map[string]any
	if err := json.Unmarshal(hdrJSON, &hdr); err != nil {
		t.Fatalf("header json: %v", err)
	}
	if hdr["alg"] != "ES256" {
		t.Fatalf("alg not pinned to ES256, got %v", hdr["alg"])
	}
	if hdr["kid"] != "K9" {
		t.Fatalf("caller kid not preserved, got %v", hdr["kid"])
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		t.Fatalf("sig decode: len=%d err=%v", len(sig), err)
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&key.PublicKey, sum[:], r, s) {
		t.Fatal("signature did not verify")
	}
}

func TestKeyFromDMatchesGeneratedKey(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	raw, err := key.Bytes()
	if err != nil {
		t.Fatalf("key bytes: %v", err)
	}
	rebuilt, err := KeyFromD(raw)
	if err != nil {
		t.Fatalf("KeyFromD: %v", err)
	}
	rebuiltPub, err := rebuilt.PublicKey.Bytes()
	if err != nil {
		t.Fatalf("rebuilt pub bytes: %v", err)
	}
	origPub, err := key.PublicKey.Bytes()
	if err != nil {
		t.Fatalf("orig pub bytes: %v", err)
	}
	if !bytes.Equal(rebuiltPub, origPub) {
		t.Fatal("rebuilt public key does not match")
	}
}
