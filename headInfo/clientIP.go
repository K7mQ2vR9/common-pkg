package headInfo

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIPResolver 从连接对端和可信代理转发链解析客户端 IP。零值仅使用连接对端。
type ClientIPResolver struct {
	trustedProxies []netip.Prefix
}

// NewClientIPResolver 校验可信代理 CIDR。代理须在 X-Forwarded-For 末尾追加其连接对端的 IP。
func NewClientIPResolver(trustedProxyCIDRs []string) (ClientIPResolver, error) {
	resolver := ClientIPResolver{trustedProxies: make([]netip.Prefix, 0, len(trustedProxyCIDRs))}
	for i, cidr := range trustedProxyCIDRs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err != nil {
			return ClientIPResolver{}, fmt.Errorf("invalid trusted proxy CIDR at index %d: %w", i, err)
		}
		if prefix.Addr().Is4In6() {
			if prefix.Bits() < 96 {
				return ClientIPResolver{}, fmt.Errorf("invalid IPv4-mapped trusted proxy CIDR at index %d", i)
			}
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		resolver.trustedProxies = append(resolver.trustedProxies, prefix.Masked())
	}
	return resolver, nil
}

func (resolver ClientIPResolver) clientIP(r *http.Request) string {
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	peer, err := netip.ParseAddrPort(remoteAddr)
	peerIP := peer.Addr()
	if err != nil {
		peerIP, err = netip.ParseAddr(remoteAddr)
		if err != nil {
			return ""
		}
	}
	peerIP = peerIP.WithZone("").Unmap()
	if !resolver.isTrustedProxy(peerIP) {
		return peerIP.String()
	}

	// 多行同名请求头按出现顺序组成一条链，从离服务端最近的一跳开始检查。
	clientIP := peerIP
	forwarded := r.Header.Values("X-Forwarded-For")
	for i := len(forwarded) - 1; i >= 0; i-- {
		entries := strings.Split(forwarded[i], ",")
		for j := len(entries) - 1; j >= 0; j-- {
			ip, err := netip.ParseAddr(strings.TrimSpace(entries[j]))
			if err != nil || ip.Zone() != "" {
				return peerIP.String()
			}
			clientIP = ip.Unmap()
			// 到达第一个不可信对端。它左侧的转发信息由该对端控制。
			if !resolver.isTrustedProxy(clientIP) {
				return clientIP.String()
			}
		}
	}
	return clientIP.String()
}

func (resolver ClientIPResolver) isTrustedProxy(ip netip.Addr) bool {
	for _, prefix := range resolver.trustedProxies {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}
