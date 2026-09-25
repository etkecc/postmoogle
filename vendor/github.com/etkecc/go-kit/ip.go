package kit

import "net"

// AnonymizeIP masks the host part of an IP for GDPR-friendlier logging: IPv4 keeps the /24
// (1.2.3.4 becomes 1.2.3.0), IPv6 keeps the /48 (2001:db8:85a3::1 becomes 2001:db8:85a3::).
// Enough to keep a coarse network and rough geo without writing down a whole person. Empty string
// stays empty; anything that isn't an IP comes back unchanged (not an error), so untrusted input
// passes through without blowing up.
//
// The /48 on IPv6 is real de-identification: it drops the bottom 80 bits, the interface identifier
// and the subnet a single machine sits in, so you can't walk a log back to one box.
//
// Feed it a bare IP, not a "host:port" or an "addr%zone": net.ParseIP doesn't parse those, so they
// fall through the not-an-IP path and come back verbatim, un-anonymized. Strip the port first.
func AnonymizeIP(ip string) string {
	if ip == "" {
		return ""
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ip // not an ip
	}

	if v4 := parsedIP.To4(); v4 != nil {
		return v4.Mask(net.CIDRMask(24, 32)).String()
	}
	return parsedIP.Mask(net.CIDRMask(48, 128)).String()
}

// IsValidIP reports whether ipStr is a routable public address: it has to parse as IPv4 or IPv6
// AND not be one of the categories you don't want a user quietly handing you:
//   - Unspecified addresses (0.0.0.0 for IPv4, :: for IPv6)
//   - Loopback addresses (127.x.x.x for IPv4, ::1 for IPv6)
//   - Private addresses (RFC 1918 for IPv4: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16; RFC 4193 for IPv6: fc00::/7)
//   - Multicast addresses (224.0.0.0/4 for IPv4, ff00::/8 for IPv6)
//   - Link-local unicast addresses (169.254.0.0/16 for IPv4, fe80::/10 for IPv6)
//   - Link-local multicast addresses
//
// Returns false for empty strings or strings that are not valid IP addresses.
func IsValidIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	return !ip.IsUnspecified() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsMulticast() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}
