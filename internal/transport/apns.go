package transport

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/crypto"
)

type APNs struct {
	key     *ecdsa.PrivateKey
	keyID   string
	teamID  string
	topic   string
	baseURL string
	client  *http.Client

	mu    sync.Mutex
	jwt   string
	jwtAt time.Time
}

func NewAPNs(creds app.APNsCreds, baseURL string, client *http.Client) (*APNs, error) {
	key, err := crypto.ParseP8(creds.KeyP8)
	if err != nil {
		return nil, err
	}
	if baseURL == "" {
		if creds.Production {
			baseURL = "https://api.push.apple.com"
		} else {
			baseURL = "https://api.sandbox.push.apple.com"
		}
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &APNs{key: key, keyID: creds.KeyID, teamID: creds.TeamID, topic: creds.Topic, baseURL: baseURL, client: client}, nil
}

func (a *APNs) providerToken() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.jwt != "" && time.Since(a.jwtAt) < 40*time.Minute {
		return a.jwt, nil
	}
	tok, err := crypto.SignES256(
		a.key,
		map[string]any{"alg": "ES256", "kid": a.keyID},
		map[string]any{"iss": a.teamID, "iat": time.Now().Unix()},
	)
	if err != nil {
		return "", err
	}
	a.jwt, a.jwtAt = tok, time.Now()
	return tok, nil
}

func (a *APNs) Send(ctx context.Context, transportRef string, msg Message) (Result, error) {
	tok, err := a.providerToken()
	if err != nil {
		return Result{}, err
	}
	payload := map[string]any{
		"aps": map[string]any{
			"mutable-content": 1,
			"alert":           map[string]any{"title": "New notification", "body": "You have a new message"},
		},
		"e":  base64.RawURLEncoding.EncodeToString(msg.Ciphertext),
		"ce": NormalizeEncoding(msg.Encoding),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/3/device/"+transportRef, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("authorization", "bearer "+tok)
	req.Header.Set("apns-topic", a.topic)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", APNsPriority(msg.Urgency))
	if msg.TTL > 0 {
		req.Header.Set("apns-expiration", strconv.FormatInt(time.Now().Add(time.Duration(msg.TTL)*time.Second).Unix(), 10))
	} else {
		req.Header.Set("apns-expiration", "0")
	}
	if msg.Topic != "" {
		req.Header.Set("apns-collapse-id", msg.Topic)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	res := mapAPNsResult(resp, respBody)
	// Drop the cached JWT so the next send regenerates it instead of reusing the
	// rejected token until the 40-minute refresh timer elapses.
	if resp.StatusCode == http.StatusForbidden &&
		(res.Reason == "ExpiredProviderToken" || res.Reason == "InvalidProviderToken") {
		a.invalidateToken()
	}
	return res, nil
}

func (a *APNs) invalidateToken() {
	a.mu.Lock()
	a.jwt = ""
	a.mu.Unlock()
}

func mapAPNsResult(resp *http.Response, body []byte) Result {
	switch {
	case resp.StatusCode == http.StatusOK:
		return Result{Delivered: true, StatusCode: resp.StatusCode}
	case resp.StatusCode == http.StatusGone:
		return Result{Permanent: true, StatusCode: resp.StatusCode, Reason: apnsReason(body)}
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
		return Result{StatusCode: resp.StatusCode, RetryAfter: retryAfter(resp), Reason: apnsReason(body)}
	case resp.StatusCode == http.StatusBadRequest:
		reason := apnsReason(body)
		if reason == "BadDeviceToken" || reason == "DeviceTokenNotForTopic" {
			return Result{Permanent: true, StatusCode: resp.StatusCode, Reason: reason}
		}
		return Result{StatusCode: resp.StatusCode, Reason: reason}
	case resp.StatusCode == http.StatusForbidden:
		// ExpiredProviderToken is transient (usually clock skew) so a fresh token
		// likely succeeds; InvalidProviderToken is usually a real key/team/topic
		// misconfig, so don't retry on it.
		reason := apnsReason(body)
		return Result{StatusCode: resp.StatusCode, Reason: reason, Retryable: reason == "ExpiredProviderToken"}
	default:
		return Result{StatusCode: resp.StatusCode, Reason: apnsReason(body)}
	}
}

func apnsReason(body []byte) string {
	var r struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(body, &r)
	return r.Reason
}

// retryAfter parses a Retry-After header, which RFC 7231 permits as either
// delta-seconds or an HTTP-date. Returns 0 when absent, malformed, or in the past.
func retryAfter(resp *http.Response) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
