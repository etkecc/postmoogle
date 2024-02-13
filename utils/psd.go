package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog"
)

type PSD struct {
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

func (p *PSD) Status(email string) string {
	psd, err := p.get(email)
	if err != nil {
		p.log.Error().Err(err).Str("email", email).Msg("error checking PSD")
		return ""
	}
	if len(psd) == 0 {
		return ""
	}
	return "👥" + psd[0].Labels["domain"] + " 👤"
}

func (p *PSD) get(identifier string) ([]*PSDTarget, error) {
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
	var psd []*PSDTarget
	err = json.Unmarshal(datab, &psd)
	if err != nil {
		return nil, err
	}

	return psd, nil
}
