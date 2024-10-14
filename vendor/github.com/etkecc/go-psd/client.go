package psd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// DefaultUserAgent is the default user agent used by the client
const DefaultUserAgent = "Go-PSD-client/v0.0.0"

// Client is the client to interact with the Prometheus Service Discovery HTTP API
type Client struct {
	url       *url.URL
	cache     *expirable.LRU[string, cacheValue]
	login     string
	password  string
	userAgent string
}

type cacheValue struct {
	data     []byte
	etag     string
	cachedAt string
}

// NewClient returns a new PSD client
func NewClient(baseURL, login, password string, userAgentOverride ...string) *Client {
	uri, err := url.Parse(baseURL)
	if err != nil || login == "" || password == "" {
		return &Client{}
	}

	cache := expirable.NewLRU[string, cacheValue](1000, nil, 5*time.Minute)
	userAgent := DefaultUserAgent
	if len(userAgentOverride) > 0 {
		userAgent = userAgentOverride[0]
	}

	return &Client{
		url:       uri,
		cache:     cache,
		login:     login,
		password:  password,
		userAgent: userAgent,
	}
}

// GetWithContext returns the list of targets for the given identifier using the given context
func (p *Client) GetWithContext(ctx context.Context, identifier string, jobOverride ...string) (Items, error) {
	if p.url == nil {
		return nil, nil
	}
	cloned := *p.url
	job := "node"
	if len(jobOverride) > 0 {
		job = jobOverride[0]
	}
	uri := cloned.JoinPath("/" + job + "/" + identifier)
	urlTarget := uri.String()
	cachedData, cached := p.cache.Get(urlTarget)

	childCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(childCtx, http.MethodGet, urlTarget, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(p.login, p.password)
	if cached {
		req.Header.Set("If-Modified-Since", cachedData.cachedAt)
		req.Header.Set("If-None-Match", cachedData.etag)
	}

	req.Header.Set("User-Agent", p.userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var datab []byte
	if resp.StatusCode == http.StatusNotModified && cached {
		datab = cachedData.data
	} else if resp.StatusCode == http.StatusGone { // not found, to distinguish from reverse proxy 404 error
		return nil, nil
	} else if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP error: %s", resp.Status) //nolint:goerr113 // that's ok
		return nil, err
	}

	if datab == nil {
		datab, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		lastModified := resp.Header.Get("Last-Modified")
		if lastModified == "" {
			lastModified = time.Now().Format(http.TimeFormat)
		}
		p.cache.Add(urlTarget, cacheValue{
			data:     datab,
			etag:     resp.Header.Get("ETag"),
			cachedAt: lastModified,
		})
	}
	var psd []*Item
	err = json.Unmarshal(datab, &psd)
	if err != nil {
		return nil, err
	}

	return psd, nil
}

// Get returns the list of targets for the given identifier
func (p *Client) Get(identifier string) (Items, error) {
	return p.GetWithContext(context.Background(), identifier)
}
