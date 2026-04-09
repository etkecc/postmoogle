package kit

import (
	"net"
	"strings"
)

// AnonymizeIP returns an anonymized form of the given IP address for GDPR-compliant logging.
//
// For IPv4 addresses, it replaces the last octet with 0 (e.g., 1.2.3.4 becomes 1.2.3.0).
// For IPv6 addresses, it replaces the last group with 0 (e.g., 2001:db8::1 becomes 2001:db8::0).
//
// The function handles three cases:
//   - Empty string: returns empty string unchanged.
//   - Non-IP string (invalid format): returns the input string unchanged (not an error).
//   - Valid IP (IPv4 or IPv6): returns the anonymized form with the last segment zeroed.
func AnonymizeIP(ip string) string {
	if ip == "" {
		return ""
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ip // not an ip
	}

	// IPv4
	if parsedIP.To4() != nil {
		ipParts := strings.Split(parsedIP.String(), ".")
		if len(ipParts) == 4 {
			ipParts[3] = "0"
			return strings.Join(ipParts, ".")
		}
	}

	// IPv6
	ipParts := strings.Split(parsedIP.String(), ":")
	ipParts[len(ipParts)-1] = "0"
	return strings.Join(ipParts, ":")
}

// IsValidIP reports whether ipStr is a valid public IPv4 or IPv6 address suitable for general use.
//
// It parses ipStr as either IPv4 or IPv6 and explicitly rejects the following address categories:
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
