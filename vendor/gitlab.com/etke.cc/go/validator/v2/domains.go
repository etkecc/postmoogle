package validator

import (
	"regexp"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// based on W3C email regex, ref: https://www.w3.org/TR/2016/REC-html51-20161101/sec-forms.html#email-state-typeemail
var domainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]$`)

// Domain checks if domain is valid
func (v *V) Domain(domain string) bool {
	if domain == "" {
		return !v.cfg.Domain.Enforce
	}

	if !v.DomainString(domain) {
		return false
	}

	return true
}

// DomainString checks if domain string / value is valid using string checks like length and regexp
func (v *V) DomainString(domain string) bool {
	if len(domain) < 4 || len(domain) > 77 {
		v.cfg.Log("domain %s invalid, reason: length", domain)
		return false
	}

	if !domainRegex.MatchString(domain) {
		v.cfg.Log("domain %s invalid, reason: regexp", domain)
		return false
	}

	return true
}

// GetBase returns base domain/host of the provided domain
func (v *V) GetBase(domain string) string {
	// domain without subdomain "example.com" has parts: example com
	minSize := 2
	if v.hasSuffix(domain) {
		// domain with a certain TLDs contains 3 parts: example.co.uk -> example co uk
		minSize = 3
	}

	parts := strings.Split(domain, ".")
	size := len(parts)
	// If domain contains only 2 parts (or less) - consider it without subdomains
	if size <= minSize {
		return domain
	}

	// return domain without subdomain (sub.example.com -> example.com; sub.example.co.uk -> example.co.uk)
	return strings.Join(parts[size-minSize:], ".")
}

// hasSuffix checks if domain has a suffix from public suffix list or from predefined suffix list
func (v *V) hasSuffix(domain string) bool {
	for _, suffix := range v.cfg.Domain.PrivateSuffixes {
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}

	eTLD, _ := publicsuffix.PublicSuffix(domain)
	return strings.IndexByte(eTLD, '.') >= 0
}
