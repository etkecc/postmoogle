// Fuzz testing for package spf.
//
// Run it with:
//
//   go-fuzz-build blitiri.com.ar/go/spf
//   go-fuzz -bin=./spf-fuzz.zip -workdir=testdata/fuzz
//

//go:build gofuzz
// +build gofuzz

package spf

import (
	"net"

	"blitiri.com.ar/go/spf/internal/dnstest"
)

// Parsed IP addresses, for convenience.
var (
	ip1110 = net.ParseIP("1.1.1.0")
	ip1111 = net.ParseIP("1.1.1.1")
	ip6666 = net.ParseIP("2001:db8::68")
	ip6660 = net.ParseIP("2001:db8::0")
)

// DNS resolver to use. Will be initialized once with the expected fixtures,
// and then reused on each fuzz run.
var dns = dnstest.NewResolver()

func init() {
	dns.Ip["d1111"] = []net.IP{ip1111}
	dns.Ip["d1110"] = []net.IP{ip1110}
	dns.Mx["d1110"] = []*net.MX{{"d1110", 5}, {"nothing", 10}}
	dns.Ip["d6666"] = []net.IP{ip6666}
	dns.Ip["d6660"] = []net.IP{ip6660}
	dns.Mx["d6660"] = []*net.MX{{"d6660", 5}, {"nothing", 10}}
	dns.Addr["2001:db8::68"] = []string{"sonlas6.", "domain.", "d6666."}
	dns.Addr["1.1.1.1"] = []string{"lalala.", "domain.", "d1111."}
}

func Fuzz(data []byte) int {
	// The domain's TXT record comes from the fuzzer.
	dns.Txt["domain"] = []string{string(data)}

	v4result, _ := CheckHostWithSender(
		ip1111, "helo", "domain", WithResolver(dns))
	v6result, _ := CheckHostWithSender(
		ip6666, "helo", "domain", WithResolver(dns))

	// Raise priority if any of the results was something other than
	// PermError, as it means the data was better formed.
	if v4result != PermError || v6result != PermError {
		return 1
	}
	return 0
}
