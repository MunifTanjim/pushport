package transport

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// extraBlockedPrefixes are non-routable ranges netip's classifiers don't cover:
//   - 100.64.0.0/10: RFC 6598 CGNAT/shared space (IsPrivate is RFC 1918 + ULA only).
//   - 0.0.0.0/8: RFC 1122 "this host"; IsUnspecified matches only 0.0.0.0, but
//     Linux routes 0.x.y.z to loopback.
//   - 240.0.0.0/4: RFC 1112 reserved, incl. the 255.255.255.255 broadcast.
var extraBlockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

// blockedIP reports whether addr must not be a WebPush target (SSRF guard).
// The 169.254.169.254 cloud-metadata address falls under link-local unicast.
func blockedIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsMulticast() {
		return true
	}
	for _, p := range extraBlockedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	if v4, ok := embeddedV4(addr); ok {
		return blockedIP(v4)
	}
	return false
}

// embeddedV4 returns the IPv4 an IPv6 address embeds (see cases), ok=false if
// none. netip reports these as global unicast, so without re-checking the
// extracted v4 an internal target could hide in an IPv6 form. v4-mapped
// ::ffff:0:0/96 is already handled by Unmap upstream.
func embeddedV4(addr netip.Addr) (netip.Addr, bool) {
	if !addr.Is6() {
		return netip.Addr{}, false
	}
	b := addr.As16()
	switch {
	case b[0] == 0x20 && b[1] == 0x02: // 6to4 2002::/16, RFC 3056
		return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}), true
	case b[0] == 0x00 && b[1] == 0x64 && b[2] == 0xff && b[3] == 0x9b &&
		allZero(b[4:12]): // NAT64 64:ff9b::/96, RFC 6052
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}), true
	case allZero(b[0:12]): // IPv4-compatible ::/96, RFC 4291 (deprecated)
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}), true
	}
	return netip.Addr{}, false
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

// EndpointPolicy is the WebPush SSRF allowlist; its zero value is the strict
// default (https + public hosts only). It gates both registration (Validate) and
// connection (GuardClient), so the two can't disagree.
type EndpointPolicy struct {
	hosts map[string]bool // lowercased
	cidrs []netip.Prefix
}

// NewEndpointPolicy builds a policy from allowlist entries, each a CIDR, a bare
// IP, or a hostname. A malformed CIDR/IP entry errors, to fail fast at startup.
func NewEndpointPolicy(entries []string) (EndpointPolicy, error) {
	var p EndpointPolicy
	for _, raw := range entries {
		e := strings.TrimSpace(raw)
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			pfx, err := netip.ParsePrefix(e)
			if err != nil {
				return EndpointPolicy{}, fmt.Errorf("invalid CIDR %q: %w", e, err)
			}
			p.cidrs = append(p.cidrs, pfx.Masked())
			continue
		}
		if addr, err := netip.ParseAddr(e); err == nil {
			addr = addr.Unmap()
			p.cidrs = append(p.cidrs, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		if p.hosts == nil {
			p.hosts = make(map[string]bool)
		}
		p.hosts[strings.ToLower(e)] = true
	}
	return p, nil
}

func (p EndpointPolicy) hostAllowed(host string) bool {
	if p.hosts[strings.ToLower(host)] {
		return true
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return p.ipAllowed(addr)
	}
	return false
}

func (p EndpointPolicy) ipAllowed(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, c := range p.cidrs {
		if c.Contains(addr) {
			return true
		}
	}
	return false
}

// Validate rejects a WebPush endpoint at registration: non-https or a blocked IP
// literal, unless the host is allow-listed. Hostnames resolving to blocked
// addresses are caught at dial time by GuardClient, not here.
func (p EndpointPolicy) Validate(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid endpoint url: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("endpoint url must include a host")
	}
	if !p.hostAllowed(host) && u.Scheme != "https" {
		return fmt.Errorf("endpoint url must use https")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if !p.ipAllowed(addr) && blockedIP(addr) {
			return fmt.Errorf("endpoint host %s is not an allowed address", host)
		}
	}
	return nil
}

// GuardClient returns a copy of base whose transport refuses to dial blocked
// addresses. Allow-listed hosts dial directly; every other host is resolved once
// and the checked IP is pinned, so the address validated is the address dialed
// (no DNS-rebinding window).
//
// It errors if base.Transport is not *http.Transport: silently returning an
// unguarded client would disable a security control without a signal.
func (p EndpointPolicy) GuardClient(base *http.Client) (*http.Client, error) {
	rt := base.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	t, ok := rt.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("cannot install SSRF guard: transport is %T, not *http.Transport", rt)
	}
	c := *base
	tc := t.Clone()
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	tc.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if p.hostAllowed(host) {
			return dialer.DialContext(ctx, network, addr)
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !blockedIP(ip) {
				return dialer.DialContext(ctx, network, net.JoinHostPort(ip.Unmap().String(), port))
			}
		}
		return nil, fmt.Errorf("refusing to connect to blocked address for host %q", host)
	}
	c.Transport = tc
	return &c, nil
}
