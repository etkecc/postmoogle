package psd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

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

// Get returns the list of targets for the given identifier
func (p *Client) Get(identifier string) ([]*Target, error) {
	if p.url == nil {
		return nil, nil
	}
	cloned := *p.url
	uri := cloned.JoinPath("/node/" + identifier)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(p.login, p.password)
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
