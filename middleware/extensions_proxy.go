package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type proxyContextKey struct{}

// ClientInfo is derived only from a trusted direct peer. Scheme otherwise
// reflects TLS on the direct connection, never an untrusted forwarding header.
type ClientInfo struct {
	Address netip.Addr
	Scheme  string
}

func ClientFromContext(ctx context.Context) (ClientInfo, bool) {
	client, ok := ctx.Value(proxyContextKey{}).(ClientInfo)
	return client, ok
}

// TrustedProxies requires an explicit nonempty prefix allowlist. X-Forwarded-For
// is traversed from the direct peer toward the client, stopping at the first
// untrusted hop. Malformed chains are ignored entirely; Forwarded is not used.
func TrustedProxies(prefixes []netip.Prefix) (Extension, error) {
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("trusted proxies: explicit prefixes required")
	}
	allow := append([]netip.Prefix(nil), prefixes...)
	for _, prefix := range allow {
		if !prefix.IsValid() || prefix.Bits() == 0 {
			return nil, fmt.Errorf("trusted proxies: invalid or universal prefix")
		}
	}
	trusted := func(address netip.Addr) bool {
		for _, prefix := range allow {
			if prefix.Contains(address) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			peer, _ := netip.ParseAddr(host)
			peer = peer.Unmap()
			info := ClientInfo{Address: peer, Scheme: "http"}
			if r.TLS != nil {
				info.Scheme = "https"
			}
			if trusted(peer) {
				raw := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
				parts := strings.Split(raw, ",")
				hops := make([]netip.Addr, 0, len(parts))
				valid := raw != "" && len(raw) <= 4096 && len(parts) <= 64
				for _, part := range parts {
					address, err := netip.ParseAddr(strings.TrimSpace(part))
					if err != nil || address.Zone() != "" {
						valid = false
						break
					}
					hops = append(hops, address.Unmap())
				}
				if valid {
					for i := len(hops) - 1; i >= 0 && trusted(info.Address); i-- {
						info.Address = hops[i]
					}
				}
				// A single protocol is required: ambiguous multi-proxy protocol
				// lists must be normalized by the trusted edge.
				proto := r.Header.Values("X-Forwarded-Proto")
				if len(proto) == 1 && (proto[0] == "http" || proto[0] == "https") {
					info.Scheme = proto[0]
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), proxyContextKey{}, info)))
		})
	}, nil
}
