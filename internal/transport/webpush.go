package transport

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/crypto"
)

type WebPush struct {
	key     *ecdsa.PrivateKey
	pubB64  string
	subject string
	client  *http.Client
}

func NewWebPush(creds app.WebPushCreds, client *http.Client) (*WebPush, error) {
	d, err := base64.RawURLEncoding.DecodeString(creds.VAPIDPrivateKey)
	if err != nil {
		return nil, err
	}
	key, err := crypto.KeyFromD(d)
	if err != nil {
		return nil, err
	}
	pub, err := crypto.VAPIDPublicKey(key)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &WebPush{key: key, pubB64: pub, subject: creds.Subject, client: client}, nil
}

func (wp *WebPush) vapidHeader(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	aud := u.Scheme + "://" + u.Host
	jwt, err := crypto.SignES256(
		wp.key,
		map[string]any{"typ": "JWT", "alg": "ES256"},
		map[string]any{"aud": aud, "exp": time.Now().Add(12 * time.Hour).Unix(), "sub": wp.subject},
	)
	if err != nil {
		return "", err
	}
	return "vapid t=" + jwt + ", k=" + wp.pubB64, nil
}

func (wp *WebPush) Send(ctx context.Context, transportRef string, msg Message) (Result, error) {
	auth, err := wp.vapidHeader(transportRef)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, transportRef, bytes.NewReader(msg.Ciphertext))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Encoding", NormalizeEncoding(msg.Encoding))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", strconv.Itoa(msg.TTL))
	if msg.Urgency != "" {
		req.Header.Set("Urgency", msg.Urgency)
	}
	if msg.Topic != "" {
		req.Header.Set("Topic", msg.Topic)
	}
	resp, err := wp.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	return mapWebPushResult(resp), nil
}

func mapWebPushResult(resp *http.Response) Result {
	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusAccepted:
		return Result{Delivered: true, StatusCode: resp.StatusCode}
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return Result{Permanent: true, StatusCode: resp.StatusCode}
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
		return Result{StatusCode: resp.StatusCode, RetryAfter: retryAfter(resp)}
	default:
		return Result{StatusCode: resp.StatusCode}
	}
}
