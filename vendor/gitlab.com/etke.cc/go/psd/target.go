package psd

// Target is a struct that represents a single set of targets in Prometheus Service Discovery
type Target struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

// GetDomain returns the domain of the target
func (t *Target) GetDomain() string {
	return t.Labels["domain"]
}
