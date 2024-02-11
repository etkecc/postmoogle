package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

//nolint:gocritic // sync.Mutex is intended
type PSD struct {
	sync.Mutex
	cachedAt time.Time
	cache    map[string]bool
	log      *zerolog.Logger
	url      *url.URL
	login    string
	password string
}

type PSDTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func NewPSD(baseURL, login, password string, log *zerolog.Logger) *PSD {
	uri, err := url.Parse(baseURL)
	if err != nil || login == "" || password == "" {
		return &PSD{}
	}
	return &PSD{url: uri, login: login, password: password, log: log}
}

func (p *PSD) Contains(email string) (bool, error) {
	if p.cachedAt.IsZero() || time.Since(p.cachedAt) > 10*time.Minute {
		err := p.updateCache()
		if err != nil {
			return false, err
		}
	}

	p.Lock()
	defer p.Unlock()
	return p.cache[email], nil
}

func (p *PSD) Status(email string) string {
	ok, err := p.Contains(email)
	if !ok || err != nil {
		return ""
	}
	return "👤"
}

func (p *PSD) updateCache() error {
	p.Lock()
	defer p.Unlock()
	defer func() {
		p.cachedAt = time.Now()
	}()

	if p.url == nil {
		return nil
	}
	cloned := *p.url
	uri := cloned.JoinPath("/emails")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri.String(), http.NoBody)
	if err != nil {
		p.log.Error().Err(err).Msg("failed to create request")
		return err
	}
	req.SetBasicAuth(p.login, p.password)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("%s", resp.Status) //nolint:goerr113 // no need to wrap
		p.log.Error().Err(err).Msg("failed to fetch PSD")
		return err
	}
	datab, err := io.ReadAll(resp.Body)
	if err != nil {
		p.log.Error().Err(err).Msg("failed to read response")
		return err
	}
	var psd []*PSDTarget
	err = json.Unmarshal(datab, &psd)
	if err != nil {
		p.log.Error().Err(err).Msg("failed to unmarshal response")
		return err
	}

	p.cache = make(map[string]bool)
	for _, t := range psd {
		for _, email := range t.Targets {
			p.cache[email] = true
		}
	}

	return nil
}
