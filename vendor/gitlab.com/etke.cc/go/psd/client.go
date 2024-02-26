package psd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"time"
)

var version = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return "0.0.0-unknown"
}()

type Client struct {
	url      *url.URL
	login    string
	password string
}

// NewClient returns a new PSD client
func NewClient(baseURL, login, password string) *Client {
	uri, err := url.Parse(baseURL)
	if err != nil || login == "" || password == "" {
		return &Client{}
	}
	return &Client{url: uri, login: login, password: password}
}

// GetWithContext returns the list of targets for the given identifier using the given context
func (p *Client) GetWithContext(ctx context.Context, identifier string) ([]*Target, error) {
	if p.url == nil {
		return nil, nil
	}
	cloned := *p.url
	uri := cloned.JoinPath("/node/" + identifier)

	childCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(childCtx, http.MethodGet, uri.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(p.login, p.password)
	req.Header.Set("User-Agent", "Go-psd-client/"+version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("%s", resp.Status) //nolint:goerr113 // that's ok
		return nil, err
	}
	datab, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var psd []*Target
	err = json.Unmarshal(datab, &psd)
	if err != nil {
		return nil, err
	}

	return psd, nil
}

// Get returns the list of targets for the given identifier
func (p *Client) Get(identifier string) ([]*Target, error) {
	return p.GetWithContext(context.Background(), identifier)
}
