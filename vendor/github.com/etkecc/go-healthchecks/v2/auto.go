package healthchecks

import "time"

// Auto is intended to start as separate goroutine (go c.Auto(5*time.Second))
// it will automatically send Success (ping) requests, leaving the client itself fully usable
// to stop the Auto(), call Shutdown() and destroy the client
func (c *Client) Auto(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.Success()
		case <-c.done:
			return
		}
	}
}
