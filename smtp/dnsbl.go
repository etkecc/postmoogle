package smtp

import (
	"context"
	"errors"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/etke.cc/postmoogle/utils"
)

const (
	// DNSBLTimeout is the timeout for DNSBL requests
	DNSBLTimeout = 5 * time.Second
	// DNSBLDefaultSignal is the default signal for DNSBLs
	DNSBLDefaultSignal = "127.0.0.2"
)

// DNSBLs is a list of Domain Name System Blacklists with list of signals they use
var DNSBLs = map[string][]string{
	"b.barracudacentral.org":  {DNSBLDefaultSignal},
	"bl.spamcop.net":          {DNSBLDefaultSignal, "127.0.0.3"},
	"ix.dnsbl.manitu.net":     {DNSBLDefaultSignal},
	"psbl.surriel.com":        {DNSBLDefaultSignal},
	"rbl.interserver.net":     {DNSBLDefaultSignal},
	"spam.dnsbl.anonmails.de": {DNSBLDefaultSignal},
	"zen.spamhaus.org":        {DNSBLDefaultSignal, "127.0.0.3", "127.0.0.4", "127.0.0.5", "127.0.0.6", "127.0.0.7", "127.0.0.9"},
	"rbl.your-server.de":      {DNSBLDefaultSignal},
}

// DNSBLRequest is a request to check if an IP address is listed in any of the DNSBLs
type DNSBLRequest struct {
	ctx     context.Context //nolint:containedctx // this is a request struct
	log     *zerolog.Logger
	addr    net.Addr
	ttl     time.Duration
	results []*DNSBLResult
	wg      *sync.WaitGroup
}

type DNSBLResult struct {
	RBL     string
	Reasons string
	Listed  bool
	Error   bool
}

// CheckDNSBLs checks if the given IP address is listed in any of the DNSBLs, and returns a decision, based on the results
func CheckDNSBLs(ctx context.Context, log *zerolog.Logger, addr net.Addr, optionalTimeout ...time.Duration) bool {
	ttl := DNSBLTimeout
	if len(optionalTimeout) > 0 {
		ttl = optionalTimeout[0]
	}
	logger := log.With().Str("addr", addr.String()).Logger()

	req := &DNSBLRequest{
		ctx:     ctx,
		log:     &logger,
		addr:    addr,
		ttl:     ttl,
		results: make([]*DNSBLResult, 0, len(DNSBLs)),
		wg:      &sync.WaitGroup{},
	}
	req.check()
	total := len(req.results)
	var listed, unlisted int
	var listedRBLs, unlistedRBLs, failedRBLs []string
	for _, r := range req.results {
		if r.Error {
			failedRBLs = append(failedRBLs, r.RBL)
			continue
		}
		if r.Listed {
			listed++
			listedRBLs = append(listedRBLs, r.RBL)
		} else {
			unlisted++
			unlistedRBLs = append(unlistedRBLs, r.RBL)
		}
	}

	decision := req.decision(total, listed, unlisted)
	logger.Info().
		Int("listed_in", listed).
		Strs("listed_rbls", listedRBLs).
		Int("unlisted_in", unlisted).
		Strs("unlisted_rbls", unlistedRBLs).
		Strs("failed_rbls", failedRBLs).
		Int("total", total).
		Bool("blocked", decision).
		Msg("DNSBL results")

	return decision
}

// check checks if the given IP address is listed in any of the DNSBLs
func (req *DNSBLRequest) check() {
	req.wg.Add(len(DNSBLs))
	defer req.wg.Wait()

	for rbl, signals := range DNSBLs {
		go req.checkRBL(rbl, signals)
	}
}

func (req *DNSBLRequest) checkRBL(rbl string, signals []string) {
	defer req.wg.Done()

	ctx, cancel := context.WithTimeout(req.ctx, req.ttl)
	defer cancel()

	host := req.getHost(rbl)
	log := req.log.With().Str("rbl", rbl).Str("host", host).Logger()
	log.Debug().Msg("checking")

	// first, check if the host is mentioned in the RBL ("listed" status can be set _only_ if the signal will match)
	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	var dnsErr *net.DNSError
	if err != nil {
		// not found = not listed
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			req.results = append(req.results, &DNSBLResult{RBL: rbl, Listed: false})
			return
		}
		// other errors = unknown status
		req.results = append(req.results, &DNSBLResult{RBL: rbl, Error: true})
		return
	}
	// if the host is resolved, check if there is any signal in the response
	// if not = not listed
	if len(ips) == 0 {
		req.results = append(req.results, &DNSBLResult{RBL: rbl, Listed: false})
		return
	}

	// if there is a DNS entry, check if it matches any of the signals
	for _, ip := range ips {
		// if the signal is found = listed
		if slices.Contains(signals, ip) {
			// get TXT records for the host, just additional information
			txts, _ := net.DefaultResolver.LookupTXT(ctx, host) //nolint:errcheck // the host is listed for sure, just no information about it
			req.results = append(req.results, &DNSBLResult{RBL: rbl, Listed: true, Reasons: strings.Join(txts, "; ")})
			log.Debug().Str("signal", ip).Msg("listed")
			return
		}
	}
	// if no signal is found = not listed, despite the host being resolved
	req.results = append(req.results, &DNSBLResult{RBL: rbl, Listed: false})
}

// getHost returns the host name for the given DNSBL list
func (req *DNSBLRequest) getHost(list string) string {
	ip := net.ParseIP(utils.AddrIP(req.addr))
	if ip == nil {
		return ""
	}
	var b strings.Builder
	v4 := ip.To4()
	if v4 != nil {
		s := len(v4) - 1
		for i := s; i >= 0; i-- {
			if i < s {
				b.WriteByte('.')
			}
			b.WriteString(strconv.Itoa(int(v4[i])))
		}
	} else {
		s := len(ip) - 1
		const chars = "0123456789abcdef"
		for i := s; i >= 0; i-- {
			if i < s {
				b.WriteByte('.')
			}
			v := ip[i]
			b.WriteByte(chars[v>>0&0xf])
			b.WriteByte('.')
			b.WriteByte(chars[v>>4&0xf])
		}
	}
	b.WriteString(".")
	b.WriteString(list)
	b.WriteString(".") // trailing dot is required

	return b.String()
}

// decision returns a decision based on the DNSBL results
func (req *DNSBLRequest) decision(total, listed, unlisted int) bool {
	if total == 0 || listed == 0 {
		return false
	}

	if listed < unlisted {
		return false
	}
	return true
}
