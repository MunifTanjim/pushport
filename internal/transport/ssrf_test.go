package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"169.254.169.254", true}, // cloud metadata
		{"100.64.1.1", true},      // RFC 6598 CGNAT / shared space
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"::", true},
		{"fd00::1", true},
		{"fe80::1", true},
		{"::ffff:10.0.0.1", true},          // v4-mapped private (Unmap)
		{"::ffff:169.254.169.254", true},   // v4-mapped metadata (Unmap)
		{"0.1.2.3", true},                  // 0.0.0.0/8 "this host" (Linux -> loopback)
		{"255.255.255.255", true},          // limited broadcast (240.0.0.0/4)
		{"240.0.0.1", true},                // reserved (240.0.0.0/4)
		{"64:ff9b::a9fe:a9fe", true},       // NAT64 embedding 169.254.169.254
		{"64:ff9b::169.254.169.254", true}, // NAT64 embedding 169.254.169.254 (dotted)
		{"64:ff9b::a00:1", true},           // NAT64 embedding 10.0.0.1
		{"2002:a9fe:a9fe::", true},         // 6to4 embedding 169.254.169.254
		{"2002:a00:1::", true},             // 6to4 embedding 10.0.0.1
		{"::169.254.169.254", true},        // IPv4-compatible embedding metadata
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2606:4700:4700::1111", false},
		{"64:ff9b::808:808", false}, // NAT64 embedding 8.8.8.8 (public) stays allowed
		{"2002:808:808::", false},   // 6to4 embedding 8.8.8.8 (public) stays allowed
	}
	for _, c := range cases {
		addr := netip.MustParseAddr(c.ip)
		if got := blockedIP(addr); got != c.blocked {
			t.Errorf("blockedIP(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
}

func TestEndpointPolicyValidateStrict(t *testing.T) {
	p, err := NewEndpointPolicy(nil)
	if err != nil {
		t.Fatalf("NewEndpointPolicy: %v", err)
	}
	reject := []string{
		"http://push.example.com/ep", // non-https
		"https://169.254.169.254/ep",
		"https://127.0.0.1/ep",
		"https://[::1]/ep",
		"https://10.0.0.1/ep",
		"http://169.254.169.254/ep",
		"",
		"://nonsense",
		"https:///ep", // missing host
	}
	for _, raw := range reject {
		if err := p.Validate(raw); err == nil {
			t.Errorf("Validate(%q) = nil, want error", raw)
		}
	}
	if err := p.Validate("https://push.example.com/ep"); err != nil {
		t.Errorf("Validate(public https) = %v, want nil", err)
	}
}

func TestEndpointPolicyValidateAllowlist(t *testing.T) {
	p, err := NewEndpointPolicy([]string{"localhost", "push.internal", "10.0.0.0/8", "127.0.0.1"})
	if err != nil {
		t.Fatalf("NewEndpointPolicy: %v", err)
	}
	accept := []string{
		"http://localhost/ep", // allow-listed host: http tolerated
		"http://push.internal/ep",
		"http://127.0.0.1/ep",
		"http://10.1.2.3/ep", // inside 10.0.0.0/8
		"https://push.example.com/ep",
	}
	for _, raw := range accept {
		if err := p.Validate(raw); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", raw, err)
		}
	}
	// A private host not covered by the allowlist stays blocked.
	if err := p.Validate("https://192.168.0.1/ep"); err == nil {
		t.Errorf("Validate(non-listed private) = nil, want error")
	}
}

func TestNewEndpointPolicyRejectsMalformedCIDR(t *testing.T) {
	if _, err := NewEndpointPolicy([]string{"10.0.0.0/99"}); err == nil {
		t.Fatal("NewEndpointPolicy(bad cidr) = nil, want error")
	}
}

func TestGuardClientBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	strict, _ := NewEndpointPolicy(nil)
	client, err := strict.GuardClient(srv.Client())
	if err != nil {
		t.Fatalf("GuardClient: %v", err)
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if _, err := client.Do(req); err == nil {
		t.Fatal("guarded client reached loopback, want dial error")
	}
}

// TestGuardClientBlocksRebinding covers the resolve-then-block (anti-DNS-rebinding)
// branch: a hostname resolving to loopback must be refused under the strict policy.
func TestGuardClientBlocksRebinding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	strict, _ := NewEndpointPolicy(nil)
	client, err := strict.GuardClient(srv.Client())
	if err != nil {
		t.Fatalf("GuardClient: %v", err)
	}
	url := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if _, err := client.Do(req); err == nil {
		t.Fatal("guarded client reached a hostname resolving to loopback, want error")
	}
}

// roundTripperOnly is not an *http.Transport, so the guard cannot be installed.
type roundTripperOnly struct{}

func (roundTripperOnly) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

func TestGuardClientRejectsNonHTTPTransport(t *testing.T) {
	strict, _ := NewEndpointPolicy(nil)
	if _, err := strict.GuardClient(&http.Client{Transport: roundTripperOnly{}}); err == nil {
		t.Fatal("GuardClient(non-*http.Transport) = nil error, want error (guard cannot be installed)")
	}
}

func TestGuardClientHonorsAllowlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	// IP entry: 127.0.0.1 allow-listed lets the loopback request through.
	byIP, _ := NewEndpointPolicy([]string{"127.0.0.1", "::1"})
	client, err := byIP.GuardClient(srv.Client())
	if err != nil {
		t.Fatalf("GuardClient: %v", err)
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("allow-listed IP request failed: %v", err)
	}
	resp.Body.Close()

	// Hostname entry: reach the same loopback server by name.
	byHost, _ := NewEndpointPolicy([]string{"localhost"})
	client, err = byHost.GuardClient(srv.Client())
	if err != nil {
		t.Fatalf("GuardClient: %v", err)
	}
	url := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	req, _ = http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("allow-listed hostname request failed: %v", err)
	}
	resp.Body.Close()
}
