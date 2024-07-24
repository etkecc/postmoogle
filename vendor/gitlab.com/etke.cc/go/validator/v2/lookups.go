package validator

import (
	"net"
	"strings"
)

// A checks if host has at least one A record
func (v *V) A(host string) bool {
	if host == "" {
		return false
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		v.cfg.Log("cannot get A records of %s: %v", host, err)
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
		v.cfg.Log("cannot get CNAME records of %s: %v", host, err)
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
		v.cfg.Log("cannot get MX records of %s: %v", host, err)
		return false
	}

	return len(mxs) > 0
}

// NS checks if host has at least one NS record,
// and optionally checks if the NS record contains the given string
func (v *V) NS(host string, contains ...string) bool {
	if host == "" {
		return false
	}
	nss, err := net.LookupNS(host)
	if err != nil {
		v.cfg.Log("cannot get NS records of %s: %v", host, err)
		return false
	}
	if len(nss) == 0 {
		v.cfg.Log("%s doesn't have NS records", host)
		return false
	}

	if len(contains) == 0 {
		return true
	}

	if len(contains) > 0 {
		for _, ns := range nss {
			for _, c := range contains {
				if strings.Contains(ns.Host, c) {
					return true
				}
			}
		}
	}
	return false
}
