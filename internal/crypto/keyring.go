package crypto

import (
	"bytes"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
)

// ErrUnknownMkid is returned by Open when a blob's key id is not in the ring.
var ErrUnknownMkid = errors.New("crypto: unknown mkid")

const mkidLen = 4 // raw fingerprint bytes prefixed on each sealed blob

// Keyring is a set of versioned root secrets, one designated primary (used for
// all new writes/derivations). Older secrets stay for reads during a rotation.
type Keyring struct {
	keys    map[string][]byte // mkid(hex) -> 32-byte secret
	primary string            // mkid(hex) of the primary secret
}

// NewKeyring builds a keyring; secrets[0] is the primary. Each must be 32 bytes.
func NewKeyring(secrets [][]byte) (*Keyring, error) {
	if len(secrets) == 0 {
		return nil, errors.New("crypto: no secrets")
	}
	k := &Keyring{keys: make(map[string][]byte, len(secrets))}
	for i, s := range secrets {
		if len(s) != 32 {
			return nil, errors.New("crypto: secret must be 32 bytes")
		}
		id, err := mkidOf(s)
		if err != nil {
			return nil, err
		}
		if existing, ok := k.keys[id]; ok {
			// The same secret listed twice is a benign duplicate; two *distinct*
			// secrets sharing a 4-byte mkid would otherwise be silently dropped and
			// make anything sealed under the loser undecryptable — fail loudly.
			if !bytes.Equal(existing, s) {
				return nil, errors.New("crypto: mkid collision between distinct secrets")
			}
		} else {
			k.keys[id] = s
		}
		if i == 0 {
			k.primary = id
		}
	}
	return k, nil
}

func mkidOf(secret []byte) (string, error) {
	fp, err := hkdf.Key(sha256.New, secret, nil, "pushport/mkid", mkidLen)
	if err != nil {
		return "", errors.New("crypto: mkid derivation: " + err.Error())
	}
	return hex.EncodeToString(fp), nil
}

func (k *Keyring) PrimaryMkid() string { return k.primary }

func (k *Keyring) Seal(pt, aad []byte) ([]byte, error) {
	prefix, err := hex.DecodeString(k.primary)
	if err != nil {
		return nil, err
	}
	ct, err := AEADSeal(k.keys[k.primary], pt, aad)
	if err != nil {
		return nil, err
	}
	return append(prefix, ct...), nil
}

func (k *Keyring) Open(blob, aad []byte) ([]byte, error) {
	id, ok := k.MkidOf(blob)
	if !ok {
		return nil, errors.New("crypto: sealed blob too short")
	}
	key, ok := k.keys[id]
	if !ok {
		return nil, ErrUnknownMkid
	}
	return AEADOpen(key, blob[mkidLen:], aad)
}

func (k *Keyring) MkidOf(blob []byte) (string, bool) {
	if len(blob) < mkidLen {
		return "", false
	}
	return hex.EncodeToString(blob[:mkidLen]), true
}

// DeriveEndpointKey derives a per-app, per-version endpoint sealing key from the
// secret identified by mkid. ok is false if the mkid is not in the ring.
func (k *Keyring) DeriveEndpointKey(mkid, appID string, keyVersion int) ([]byte, bool) {
	secret, ok := k.keys[mkid]
	if !ok {
		return nil, false
	}
	key, err := hkdf.Key(sha256.New, secret, nil, "pushport/endpoint-key/v1:"+appID+":"+strconv.Itoa(keyVersion), 32)
	if err != nil {
		return nil, false
	}
	return key, true
}
