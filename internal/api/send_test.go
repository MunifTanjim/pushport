package api

import (
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MunifTanjim/pushport/internal/app"
	"github.com/MunifTanjim/pushport/internal/crypto"
	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/instance"
	"github.com/MunifTanjim/pushport/internal/seal"
	"github.com/MunifTanjim/pushport/internal/transport"
)

type fakeSender struct {
	res                  transport.Result
	err                  error
	called               bool
	gotTransport, gotRef string
	gotSandbox           bool
}

func (f *fakeSender) Send(_ context.Context, _, transportName, transportRef string, opts transport.SendOptions, _ transport.Message) (transport.Result, error) {
	f.called = true
	f.gotTransport, f.gotRef, f.gotSandbox = transportName, transportRef, opts.Sandbox
	return f.res, f.err
}

func newSendFixture(t *testing.T, sender Sender) (*httptest.Server, string, string) {
	t.Helper()
	return newSendFixtureWithPayload(t, sender, seal.Payload{})
}

func newSendFixtureWithPayload(t *testing.T, sender Sender, extra seal.Payload) (*httptest.Server, string, string) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	tsvc := app.NewService(d, kr)
	tn, _ := tsvc.Create(context.Background(), "app")
	instances := instance.NewService(d.Queries, kr)
	rawKey, _, _ := instances.Issue(context.Background(), tn.ID, "srv")
	sealSvc := seal.NewService(tsvc)
	p := seal.Payload{
		Transport: "apns", TransportRef: "devtoken", Exp: time.Now().Add(time.Hour).Unix(), JTI: "j1",
	}
	p.Transport = cmpOr(p.Transport, extra.Transport)
	p.TransportRef = cmpOr(p.TransportRef, extra.TransportRef)
	p.Sandbox = p.Sandbox || extra.Sandbox
	endpoint, _ := sealSvc.Seal(context.Background(), tn.ID, p)
	h := NewSendHandler(instances, sealSvc, sender, 4096)
	mux := http.NewServeMux()
	h.Routes(mux)
	srv := httptest.NewServer(WithRequestID(mux))
	t.Cleanup(srv.Close)
	return srv, rawKey, endpoint
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func TestSendPassesSandboxOption(t *testing.T) {
	sender := &fakeSender{res: transport.Result{Delivered: true, StatusCode: 200}}
	srv, key, endpoint := newSendFixtureWithPayload(t, sender, seal.Payload{Sandbox: true})
	resp := post(t, srv, endpoint, key, "ciphertext")
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", resp.StatusCode)
	}
	if !sender.called || !sender.gotSandbox {
		t.Fatalf("sandbox option not passed through: %+v", sender)
	}
}

func post(t *testing.T, srv *httptest.Server, token, authKey, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.URL+"/push/"+token, strings.NewReader(body))
	if authKey != "" {
		req.Header.Set("Authorization", "Bearer "+authKey)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

func TestSendDeliveredReturns202(t *testing.T) {
	sender := &fakeSender{res: transport.Result{Delivered: true, StatusCode: 200}}
	srv, key, endpoint := newSendFixture(t, sender)
	resp := post(t, srv, endpoint, key, "ciphertext")
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", resp.StatusCode)
	}
	if !sender.called || sender.gotTransport != "apns" || sender.gotRef != "devtoken" {
		t.Fatalf("dispatch wrong: %+v", sender)
	}
}

func TestSendRejectsUnsupportedEncoding(t *testing.T) {
	sender := &fakeSender{res: transport.Result{Delivered: true, StatusCode: 200}}
	srv, key, endpoint := newSendFixture(t, sender)
	req, _ := http.NewRequest("POST", srv.URL+"/push/"+endpoint, strings.NewReader("ciphertext"))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Encoding", "garbage")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for unsupported content-encoding, got %d", resp.StatusCode)
	}
	if sender.called {
		t.Fatal("sender should not be called when encoding is rejected")
	}
}

func TestSendUnauthorizedWithoutKey(t *testing.T) {
	srv, _, endpoint := newSendFixture(t, &fakeSender{})
	resp := post(t, srv, endpoint, "", "x")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestSendAuthDBErrorReturns500(t *testing.T) {
	// A storage failure during instance auth must be a 500, not a 401 — a 401
	// would masquerade as bad credentials and feed the per-IP auth throttle.
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	tsvc := app.NewService(d, kr)
	tn, _ := tsvc.Create(context.Background(), "app")
	instances := instance.NewService(d.Queries, kr)
	rawKey, _, _ := instances.Issue(context.Background(), tn.ID, "srv")
	sealSvc := seal.NewService(tsvc)
	endpoint, _ := sealSvc.Seal(context.Background(), tn.ID, seal.Payload{
		Transport: "apns", TransportRef: "devtoken", Exp: time.Now().Add(time.Hour).Unix(), JTI: "j1",
	})
	h := NewSendHandler(instances, sealSvc, &fakeSender{}, 4096)
	mux := http.NewServeMux()
	h.Routes(mux)
	srv := httptest.NewServer(WithRequestID(mux))
	t.Cleanup(srv.Close)

	// Break the DB so GetInstance fails with a non-ErrNoRows error.
	_ = d.Close()

	resp := post(t, srv, endpoint, rawKey, "x")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500 on DB error during auth, got %d", resp.StatusCode)
	}
}

func TestSendMalformedTokenReturns404(t *testing.T) {
	srv, key, _ := newSendFixture(t, &fakeSender{})
	resp := post(t, srv, "not-a-token", key, "x")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestSendPermanentReturns410(t *testing.T) {
	sender := &fakeSender{res: transport.Result{Permanent: true, StatusCode: 410}}
	srv, key, endpoint := newSendFixture(t, sender)
	resp := post(t, srv, endpoint, key, "x")
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("want 410, got %d", resp.StatusCode)
	}
}

func TestSendCrossTenantForbidden(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	tsvc := app.NewService(d, kr)
	tnA, _ := tsvc.Create(context.Background(), "appA")
	tnB, _ := tsvc.Create(context.Background(), "appB")
	instances := instance.NewService(d.Queries, kr)
	_, _, _ = instances.Issue(context.Background(), tnA.ID, "srvA")
	rawKeyB, _, _ := instances.Issue(context.Background(), tnB.ID, "srvB")
	sealSvc := seal.NewService(tsvc)
	endpointA, _ := sealSvc.Seal(context.Background(), tnA.ID, seal.Payload{
		Transport: "apns", TransportRef: "devtoken", Exp: time.Now().Add(time.Hour).Unix(), JTI: "j1",
	})
	h := NewSendHandler(instances, sealSvc, &fakeSender{}, 4096)
	mux := http.NewServeMux()
	h.Routes(mux)
	srv := httptest.NewServer(WithRequestID(mux))
	t.Cleanup(srv.Close)

	resp := post(t, srv, endpointA, rawKeyB, "x")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

type denyAdmitter struct{ retry time.Duration }

func (d denyAdmitter) Admit(_ context.Context, _, _, _ string) (bool, time.Duration, string) {
	return false, d.retry, "instance_rate"
}

func newSendFixtureWithAdmitter(t *testing.T, sender Sender, admitter Admitter) (*httptest.Server, string, string) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	tsvc := app.NewService(d, kr)
	tn, _ := tsvc.Create(context.Background(), "app")
	instances := instance.NewService(d.Queries, kr)
	rawKey, _, _ := instances.Issue(context.Background(), tn.ID, "srv")
	sealSvc := seal.NewService(tsvc)
	endpoint, _ := sealSvc.Seal(context.Background(), tn.ID, seal.Payload{
		Transport: "apns", TransportRef: "devtoken", Exp: time.Now().Add(time.Hour).Unix(), JTI: "j1",
	})
	h := NewSendHandler(instances, sealSvc, sender, 4096)
	h.SetAdmitter(admitter)
	mux := http.NewServeMux()
	h.Routes(mux)
	srv := httptest.NewServer(WithRequestID(mux))
	t.Cleanup(srv.Close)
	return srv, rawKey, endpoint
}

func TestSendRateLimited429(t *testing.T) {
	sender := &fakeSender{res: transport.Result{Delivered: true}}
	srv, key, endpoint := newSendFixtureWithAdmitter(t, sender, denyAdmitter{retry: 30 * time.Second})
	resp := post(t, srv, endpoint, key, "ciphertext")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") != "30" {
		t.Fatalf("want Retry-After 30, got %q", resp.Header.Get("Retry-After"))
	}
}

type credErrSender struct{}

func (credErrSender) Send(_ context.Context, _, _, _ string, _ transport.SendOptions, _ transport.Message) (transport.Result, error) {
	return transport.Result{}, transport.ErrCredentialsInvalid
}

func TestSendInvalidCredentials500(t *testing.T) {
	srv, key, endpoint := newSendFixture(t, credErrSender{})
	resp := post(t, srv, endpoint, key, "x")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500 for invalid creds, got %d", resp.StatusCode)
	}
}

func TestSendExpiredEndpointReturns410(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	mk := make([]byte, 32)
	_, _ = rand.Read(mk)
	kr, _ := crypto.NewKeyring([][]byte{mk})
	tsvc := app.NewService(d, kr)
	tn, _ := tsvc.Create(context.Background(), "app")
	instances := instance.NewService(d.Queries, kr)
	rawKey, _, _ := instances.Issue(context.Background(), tn.ID, "srv")
	sealSvc := seal.NewService(tsvc)
	endpoint, _ := sealSvc.Seal(context.Background(), tn.ID, seal.Payload{
		Transport: "apns", TransportRef: "devtoken", Exp: time.Now().Add(-time.Hour).Unix(), JTI: "j2",
	})
	h := NewSendHandler(instances, sealSvc, &fakeSender{}, 4096)
	mux := http.NewServeMux()
	h.Routes(mux)
	srv := httptest.NewServer(WithRequestID(mux))
	t.Cleanup(srv.Close)

	resp := post(t, srv, endpoint, rawKey, "x")
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("want 410, got %d", resp.StatusCode)
	}
}
