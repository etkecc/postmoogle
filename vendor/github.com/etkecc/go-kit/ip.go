package kit

import (
	"net"
	"strings"
)

// AnonymizeIP drops the last octet of the IPv4 and IPv6 address to anonymize it
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
	if len(ipParts) > 0 {
		ipParts[len(ipParts)-1] = "0"
		return strings.Join(ipParts, ":")
	}
	return ip // not an ip
}
