package validator

import "net"

// A checks if host has at least one A record
func (v *V) A(host string) bool {
	if host == "" {
		return false
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		v.log.Error("cannot get A records of %s: %v", host, err)
		return false
	}

	return len(ips) > 0
}

// CNAME checks if host has at least one CNAME record
func (v *V) CNAME(host string) bool {
	if host == "" {
		return false
	}
	cname, err := net.LookupCNAME(host)
	if err != nil {
		v.log.Error("cannot get CNAME records of %s: %v", host, err)
		return false
	}

	return cname != ""
}

// MX checks if host has at least one MX record
func (v *V) MX(host string) bool {
	if host == "" {
		return false
	}
	mxs, err := net.LookupMX(host)
	if err != nil {
		v.log.Error("cannot get MX records of %s: %v", host, err)
		return false
	}

	return len(mxs) > 0
}
