// Package client is a thin HTTP client for pushport's management API. It speaks
// the uniform {request_id, data|error} envelope.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIError is a decoded error envelope (or a synthesized transport error).
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s (%d %s)", e.Message, e.StatusCode, e.Code)
	}
	return fmt.Sprintf("%s (%d)", e.Message, e.StatusCode)
}

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string, timeout time.Duration) *Client {
	return &Client{BaseURL: baseURL, Token: token, HTTP: &http.Client{Timeout: timeout}}
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *APIError       `json:"error"`
}

// Do sends method+path with an optional JSON body and returns the envelope's
// data (nil for 204). A non-2xx response or an error envelope yields *APIError.
func (c *Client) Do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var env envelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, &APIError{Message: "invalid response body", StatusCode: resp.StatusCode}
		}
	}
	// An error envelope is authoritative regardless of status code.
	if env.Error != nil {
		return nil, env.Error
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Message: http.StatusText(resp.StatusCode), StatusCode: resp.StatusCode}
	}
	return env.Data, nil
}
