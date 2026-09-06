package transport

import (
	"context"
	"errors"
	"time"
)

var ErrNotConfigured = errors.New("transport: not configured for this transport")
var ErrCredentialsInvalid = errors.New("transport: credentials present but invalid")

type Message struct {
	Ciphertext []byte
	Encoding   string
	TTL        int
	Urgency    string
	Topic      string
}

type Result struct {
	Delivered  bool
	Permanent  bool // endpoint is dead (410 / UNREGISTERED); backend should drop it
	StatusCode int
	RetryAfter time.Duration
	Reason     string
	Retryable  bool // transport-specific transient failure (e.g. APNs expired token)
}

type Transport interface {
	Send(ctx context.Context, transportRef string, msg Message) (Result, error)
}

func APNsPriority(urgency string) string {
	switch urgency {
	case "low", "very-low":
		return "5"
	default:
		return "10"
	}
}

func FCMPriority(urgency string) string {
	switch urgency {
	case "low", "very-low":
		return "normal"
	default:
		return "high"
	}
}

func NormalizeEncoding(enc string) string {
	if enc == "" {
		return "aes128gcm"
	}
	return enc
}
