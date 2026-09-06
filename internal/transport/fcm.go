package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

type FCM struct {
	projectID string
	ts        oauth2.TokenSource
	baseURL   string
	client    *http.Client
}

func NewFCM(projectID string, ts oauth2.TokenSource, baseURL string, client *http.Client) *FCM {
	if baseURL == "" {
		baseURL = "https://fcm.googleapis.com"
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &FCM{projectID: projectID, ts: ts, baseURL: baseURL, client: client}
}

// FCMTokenSourceFromJSON builds a cached OAuth token source for the service
// account. The token exchange with Google runs on client (via oauth2.HTTPClient)
// so it inherits that client's timeout — a nil client falls back to the untimed
// http.DefaultClient. A background context is used deliberately: the source is
// long-lived and cached, so it must not capture a per-request context.
func FCMTokenSourceFromJSON(serviceAccountJSON string, client *http.Client) (oauth2.TokenSource, error) {
	cfg, err := google.JWTConfigFromJSON([]byte(serviceAccountJSON), fcmScope)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if client != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	}
	return cfg.TokenSource(ctx), nil
}

func (f *FCM) Send(ctx context.Context, transportRef string, msg Message) (Result, error) {
	tok, err := f.ts.Token()
	if err != nil {
		return Result{}, err
	}
	data := map[string]string{
		"e":  base64.RawURLEncoding.EncodeToString(msg.Ciphertext),
		"ce": NormalizeEncoding(msg.Encoding),
	}
	android := map[string]any{"priority": FCMPriority(msg.Urgency)}
	if msg.TTL > 0 {
		android["ttl"] = strconv.Itoa(msg.TTL) + "s"
	}
	if msg.Topic != "" {
		android["collapse_key"] = msg.Topic
	}
	body, err := json.Marshal(map[string]any{
		"message": map[string]any{"token": transportRef, "data": data, "android": android},
	})
	if err != nil {
		return Result{}, err
	}
	url := f.baseURL + "/v1/projects/" + f.projectID + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	return mapFCMResult(resp, respBody), nil
}

func mapFCMResult(resp *http.Response, body []byte) Result {
	var e struct {
		Error struct {
			Status string `json:"status"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	status := e.Error.Status
	switch {
	case resp.StatusCode == http.StatusOK:
		return Result{Delivered: true, StatusCode: resp.StatusCode}
	case resp.StatusCode == http.StatusNotFound || status == "UNREGISTERED" || status == "NOT_FOUND":
		return Result{Permanent: true, StatusCode: resp.StatusCode, Reason: status}
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
		return Result{StatusCode: resp.StatusCode, RetryAfter: retryAfter(resp), Reason: status}
	default:
		return Result{StatusCode: resp.StatusCode, Reason: status}
	}
}
