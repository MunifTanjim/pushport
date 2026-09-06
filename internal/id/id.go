// Package id generates unique, k-sortable identifiers (xid) for domain
// entities: apps, instances, usage plans, endpoint jti, and request ids.
//
// These are IDENTIFIERS, not secrets: xid embeds a timestamp, machine id, and
// counter, so values are ordered and guessable — never use id.New for anything
// that must be unpredictable (token secrets, keys); those stay crypto/rand.
package id

import "github.com/rs/xid"

func New() string { return xid.New().String() }
