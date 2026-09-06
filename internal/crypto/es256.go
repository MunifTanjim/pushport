package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

var ErrNotECKey = errors.New("crypto: not an EC P-256 private key")

// SignES256 signs an ES256 JWT (APNs provider and WebPush VAPID tokens). The
// "alg" header is always pinned to ES256 to match the actual signature — a
// caller cannot advertise a different or missing algorithm.
func SignES256(key *ecdsa.PrivateKey, header, claims map[string]any) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims(claims))
	h := make(map[string]any, len(header)+1)
	for k, v := range header {
		h[k] = v
	}
	h["alg"] = jwt.SigningMethodES256.Alg()
	token.Header = h
	return token.SignedString(key)
}

// ParseP8 parses an APNs auth key (PKCS#8 PEM, ".p8") into an EC private key.
func ParseP8(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("crypto: no PEM block found")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ec, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		return nil, ErrNotECKey
	}
	return ec, nil
}

// KeyFromD builds a P-256 private key from a raw scalar d (e.g. a decoded
// VAPID private key), left-padded to the 32-byte P-256 width.
func KeyFromD(d []byte) (*ecdsa.PrivateKey, error) {
	const size = 32
	if len(d) == 0 || len(d) > size {
		return nil, errors.New("crypto: invalid scalar length")
	}
	buf := make([]byte, size)
	copy(buf[size-len(d):], d)
	return ecdsa.ParseRawPrivateKey(elliptic.P256(), buf)
}

// VAPIDPublicKey returns the base64url (raw) VAPID public key (the uncompressed
// P-256 point) for key.
func VAPIDPublicKey(key *ecdsa.PrivateKey) (string, error) {
	pub, err := key.PublicKey.ECDH()
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(pub.Bytes()), nil
}
