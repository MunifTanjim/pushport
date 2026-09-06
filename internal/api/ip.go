package api

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// forwardedForHeader is honored automatically when no explicit clientIPHeader is
// configured — but only from a private/loopback peer (see clientIP).
const forwardedForHeader = "X-Forwarded-For"

// clientIPHeader is an admin-configured request header the real caller IP is
// read from (e.g. "CF-Connecting-IP"). When set it is trusted from any peer, so
// it must only be configured when the origin is reached exclusively through a
// proxy that sets it. Empty (the default) falls back to the private-peer
// X-Forwarded-For behavior. Set once at startup via SetClientIPHeader; not
// mutated while serving.
var clientIPHeader string

// SetClientIPHeader configures the header clientIP reads the caller IP from. An
// empty name leaves the private-peer X-Forwarded-For fallback in effect. Call
// once before serving.
func SetClientIPHeader(name string) {
	clientIPHeader = strings.TrimSpace(name)
}

// clientIP resolves the caller's IP for per-IP rate limiting and throttling.
//
// Resolution order:
//  1. If an explicit header is configured (SetClientIPHeader), its right-most
//     comma-separated entry is used, trusted from any peer — the admin owns
//     the guarantee that the origin is only reached through that proxy.
//  2. Otherwise, X-Forwarded-For is honored only when the direct peer is a
//     private/loopback address, i.e. an in-network proxy a remote client cannot
//     impersonate (RemoteAddr is the real TCP peer, unforgeable over TCP).
//  3. Otherwise the transport-level RemoteAddr host is used.
//
// The right-most entry is the value recorded by the closest proxy, which a client
// cannot forge by prepending its own entries. A missing/malformed value falls
// back to RemoteAddr.
func clientIP(r *http.Request) string {
	host := remoteHost(r)
	if clientIPHeader != "" {
		if ip, ok := rightmostIP(r.Header.Get(clientIPHeader)); ok {
			return ip
		}
		return host
	}
	if isPrivatePeer(host) {
		if ip, ok := rightmostIP(r.Header.Get(forwardedForHeader)); ok {
			return ip
		}
	}
	return host
}

// rightmostIP returns the right-most valid IP in a comma-separated header value.
// It returns ok=false when the value is empty or its right-most entry is not a
// valid IP (an untrustworthy proxy record — caller should fall back).
func rightmostIP(v string) (string, bool) {
	parts := strings.Split(v, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		s := strings.TrimSpace(parts[i])
		if s == "" {
			continue
		}
		if addr, err := netip.ParseAddr(s); err == nil {
			return addr.String(), true
		}
		return "", false
	}
	return "", false
}

// isPrivatePeer reports whether host is a private, loopback, or link-local
// address — a peer that can only be an in-network proxy, never a remote client.
func isPrivatePeer(host string) bool {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
