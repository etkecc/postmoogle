package utils

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strings"
	"syscall"
	"time"
)

const (
	fetchTimeout      = 15 * time.Second
	fetchMaxRedirects = 5
	fetchUserAgent    = "Mozilla/5.0 (compatible; Postmoogle; +https://github.com/etkecc/postmoogle)"
)

var (
	// ErrForbiddenAddress is returned when a download would connect to a non-public network address
	ErrForbiddenAddress = errors.New("refusing to connect to a non-public address")
	// ErrUnsupportedURL is returned for URLs that cannot be downloaded
	ErrUnsupportedURL = errors.New("unsupported URL")
	// ErrDownloadFailed is returned when the server does not respond with the file
	ErrDownloadFailed = errors.New("download failed")
	// ErrTooLarge is returned when a file exceeds the size limit
	ErrTooLarge = errors.New("file is too large")
	// ErrNotImage is returned when a file is not an image that Matrix clients can display
	ErrNotImage = errors.New("file is not a supported image")

	// nonPublicPrefixes are special-purpose networks not covered by netip.Addr methods
	nonPublicPrefixes = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),       // "this" network
		netip.MustParsePrefix("100.64.0.0/10"),   // carrier-grade NAT
		netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
		netip.MustParsePrefix("192.0.2.0/24"),    // documentation
		netip.MustParsePrefix("192.88.99.0/24"),  // 6to4 relay anycast
		netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
		netip.MustParsePrefix("198.51.100.0/24"), // documentation
		netip.MustParsePrefix("203.0.113.0/24"),  // documentation
		netip.MustParsePrefix("240.0.0.0/4"),     // reserved and broadcast
		netip.MustParsePrefix("::/96"),           // IPv4-compatible IPv6
		netip.MustParsePrefix("::ffff:0:0:0/96"), // IPv4-translated, e.g. ::ffff:0:127.0.0.1
		netip.MustParsePrefix("64:ff9b::/96"),    // NAT64, may point to private IPv4
		netip.MustParsePrefix("64:ff9b:1::/48"),  // local NAT64
		netip.MustParsePrefix("100::/64"),        // discard-only
		netip.MustParsePrefix("2001::/32"),       // Teredo, may tunnel to private IPv4
		netip.MustParsePrefix("2001:2::/48"),     // benchmarking
		netip.MustParsePrefix("2001:10::/28"),    // ORCHID
		netip.MustParsePrefix("2001:20::/28"),    // ORCHIDv2
		netip.MustParsePrefix("2001:db8::/32"),   // documentation
		netip.MustParsePrefix("2002::/16"),       // 6to4, may point to private IPv4
		netip.MustParsePrefix("3fff::/20"),       // documentation
		netip.MustParsePrefix("5f00::/16"),       // segment routing (SRv6)
		netip.MustParsePrefix("fec0::/10"),       // site-local, still routed in some internal networks
	}
)

// ImageFetcher downloads images from public http(s) URLs, refusing to connect to private networks
type ImageFetcher struct {
	client  *http.Client
	maxSize int64
}

// NewImageFetcher creates an image fetcher with a size limit per image, in bytes
func NewImageFetcher(maxSize int64) *ImageFetcher {
	return newImageFetcher(maxSize, PublicAddr)
}

// newImageFetcher creates an image fetcher that only connects to addresses allowed by the allow func
func newImageFetcher(maxSize int64, allow func(netip.Addr) bool) *ImageFetcher {
	fetcher := &ImageFetcher{maxSize: maxSize}
	// the resolved address is checked right before connecting, so DNS tricks and redirects cannot bypass it
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			addrport, err := netip.ParseAddrPort(address)
			if err != nil || !allow(addrport.Addr()) {
				return ErrForbiddenAddress
			}
			return nil
		},
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	fetcher.client = &http.Client{
		Transport: transport,
		Timeout:   fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= fetchMaxRedirects || !fetcher.isHTTP(req.URL) {
				return ErrUnsupportedURL
			}
			return nil
		},
	}
	return fetcher
}

// Fetch downloads an image, failing on non-public hosts, non-image content, and files over the size limit
func (f *ImageFetcher) Fetch(ctx context.Context, rawURL string) (*File, error) {
	target, err := url.Parse(rawURL)
	if err != nil || !f.isHTTP(target) {
		return nil, ErrUnsupportedURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fetchUserAgent)
	req.Header.Set("Accept", "image/*")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s", ErrDownloadFailed, resp.Status)
	}
	if resp.ContentLength > f.maxSize {
		return nil, ErrTooLarge
	}
	content, err := io.ReadAll(io.LimitReader(resp.Body, f.maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > f.maxSize {
		return nil, ErrTooLarge
	}

	file := NewFile(f.fileName(resp.Request.URL), content)
	if !file.IsWebImage() {
		return nil, ErrNotImage
	}
	return file, nil
}

// PublicAddr reports whether the address belongs to the public internet
func PublicAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() || addr.IsPrivate() || addr.IsMulticast() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsInterfaceLocalMulticast() {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	// ISATAP addresses (any prefix, then 0:5efe or 200:5efe and an IPv4 address) may tunnel to private IPv4
	raw := addr.As16()
	return !addr.Is6() || raw[8]&^0x03 != 0 || raw[9] != 0 || raw[10] != 0x5e || raw[11] != 0xfe
}

// FileFromDataURI decodes an image from a data: URI, like data:image/png;base64,iVBORw0KG...
func FileFromDataURI(uri string, maxSize int) (*File, error) {
	meta, data, ok := strings.Cut(uri, ",")
	if !ok || len(meta) < len("data:") || !strings.EqualFold(meta[:len("data:")], "data:") {
		return nil, ErrUnsupportedURL
	}
	if len(data) > maxSize/3*4+4 {
		return nil, ErrTooLarge
	}
	data, err := url.PathUnescape(data)
	if err != nil {
		return nil, err
	}

	content := []byte(data)
	if strings.HasSuffix(strings.ToLower(meta), ";base64") {
		cleaned := strings.TrimRight(strings.Join(strings.Fields(data), ""), "=")
		content, err = base64.RawStdEncoding.DecodeString(cleaned)
		if err != nil {
			return nil, err
		}
	}
	if len(content) > maxSize {
		return nil, ErrTooLarge
	}

	file := NewFile("", content)
	if !file.IsWebImage() {
		return nil, ErrNotImage
	}
	return file, nil
}

func (f *ImageFetcher) isHTTP(target *url.URL) bool {
	return (target.Scheme == "http" || target.Scheme == "https") && target.Host != ""
}

// fileName returns the file name from the URL path, or an empty string if there is none
func (f *ImageFetcher) fileName(target *url.URL) string {
	name := path.Base(target.Path)
	if name == "." || name == "/" {
		return ""
	}
	return name
}
