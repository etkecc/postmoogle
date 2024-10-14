package psd

import "strings"

// Item is a struct that represents a single set of targets in Prometheus Service Discovery
type Item struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

// GetDomain returns the domain of the target
func (i *Item) GetDomain() string {
	return i.Labels["domain"]
}

// Items is a slice of Prometheus Service Discovery items
type Items []*Item

// GetDomains - get domains from service discovery labels
func (items Items) GetDomains() []string {
	if len(items) == 0 {
		return []string{}
	}

	domainsMap := map[string]struct{}{}
	for _, item := range items {
		domain := item.GetDomain()
		if domain == "" {
			continue
		}

		if _, ok := domainsMap[domain]; !ok {
			domainsMap[domain] = struct{}{}
		}
	}

	domains := make([]string, 0, len(domainsMap))
	for domain := range domainsMap {
		domains = append(domains, domain)
	}

	return domains
}

// Contains - check if target exists in service discovery items
func (items Items) Contains(needle string) bool {
	for _, item := range items {
		for _, target := range item.Targets {
			if target == needle {
				return true
			}
		}
	}
	return false
}

// ContainsFederation - check if service discovery items contain matrix federation endpoint
func (items Items) ContainsFederation() bool {
	for _, item := range items {
		for _, target := range item.Targets {
			if strings.HasSuffix(target, "_matrix/federation/v1/version") {
				return true
			}
		}
	}
	return false
}

// ContainsDelegation - check if service discovery items contain matrix server delegation endpoint
func (items Items) ContainsDelegation() bool {
	for _, item := range items {
		for _, target := range item.Targets {
			if strings.HasSuffix(target, "/.well-known/matrix/server") {
				return true
			}
		}
	}
	return false
}

// ContainsMSC1929 - check if service discovery items contain matrix MSC1929 endpoint
func (items Items) ContainsMSC1929() bool {
	for _, item := range items {
		for _, target := range item.Targets {
			if strings.HasSuffix(target, "/.well-known/matrix/support") {
				return true
			}
		}
	}
	return false
}
