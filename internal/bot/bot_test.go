package bot

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/etkecc/go-kit"
	"github.com/etkecc/go-linkpearl"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite" // database of the Matrix client in tests

	"github.com/etkecc/postmoogle/internal/bot/config"
	"github.com/etkecc/postmoogle/internal/utils"
)

const testRoomID = "!mailbox:example.org"

type (
	// fakeHomeserver answers the Matrix API calls of the bot and records them
	fakeHomeserver struct {
		t        *testing.T
		server   *httptest.Server
		mu       sync.Mutex
		requests []fakeRequest
		data     map[string]string // account data by request path
		fail     []string          // requests with these path parts fail
	}

	fakeRequest struct {
		method string
		path   string
		body   []byte
	}
)

func newFakeHomeserver(t *testing.T) *fakeHomeserver {
	t.Helper()
	hs := &fakeHomeserver{t: t, data: map[string]string{}}
	hs.server = httptest.NewServer(http.HandlerFunc(hs.handle))
	t.Cleanup(hs.server.Close)
	return hs
}

// newTestBot creates a bot with a real Matrix client that talks to the fake homeserver
func newTestBot(t *testing.T, hs *fakeHomeserver) *Bot {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	log := zerolog.Nop()
	lp, err := linkpearl.New(&linkpearl.Config{Homeserver: hs.server.URL, Token: "token", DB: db, Dialect: "sqlite", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	hs.reset()
	return &Bot{
		prefix:  "!pm",
		domains: []string{"example.org", "example.com"},
		cfg:     config.New(lp, &log, "", ""),
		log:     &log,
		lp:      lp,
		mu:      kit.NewMutex(),
		images:  utils.NewImageFetcher(maxImageSize),
	}
}

func (hs *fakeHomeserver) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		hs.t.Errorf("cannot read the request: %v", err)
	}
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.requests = append(hs.requests, fakeRequest{method: r.Method, path: r.URL.Path, body: body})

	path := r.URL.Path
	w.Header().Set("Content-Type", "application/json")
	for _, part := range hs.fail {
		if strings.Contains(path, part) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"errcode": "M_FORBIDDEN", "error": "failing on purpose"}`)
			return
		}
	}
	switch {
	case strings.HasSuffix(path, "/account/whoami"):
		fmt.Fprint(w, `{"user_id": "@bot:example.org", "device_id": "BOTDEVICE"}`)
	case strings.HasSuffix(path, "/keys/query"):
		fmt.Fprint(w, `{"device_keys": {}}`)
	case strings.HasSuffix(path, "/keys/upload"):
		fmt.Fprint(w, `{"one_time_key_counts": {"signed_curve25519": 50}}`)
	case strings.HasSuffix(path, "/media/v3/upload"):
		fmt.Fprintf(w, `{"content_uri": "mxc://example.org/upload%d"}`, len(hs.requests))
	case strings.HasSuffix(path, "/createRoom"):
		fmt.Fprint(w, `{"room_id": "`+testRoomID+`"}`)
	case strings.Contains(path, "/state/m.room.power_levels") && r.Method == http.MethodGet:
		fmt.Fprint(w, `{"users": {"@bot:example.org": 100}, "users_default": 0}`)
	case strings.Contains(path, "/state/") || strings.Contains(path, "/send/"):
		fmt.Fprint(w, `{"event_id": "$event"}`)
	case strings.Contains(path, "/account_data/"):
		hs.accountData(w, r, body)
	default:
		hs.t.Logf("the fake homeserver does not know %s %s", r.Method, path)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errcode": "M_UNRECOGNIZED", "error": "unknown request"}`)
	}
}

func (hs *fakeHomeserver) accountData(w http.ResponseWriter, r *http.Request, body []byte) {
	if r.Method == http.MethodPut {
		hs.data[r.URL.Path] = string(body)
		fmt.Fprint(w, `{}`)
		return
	}
	data, ok := hs.data[r.URL.Path]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errcode": "M_NOT_FOUND", "error": "no account data"}`)
		return
	}
	fmt.Fprint(w, data)
}

// reset forgets the requests made so far, e.g. while the client started
func (hs *fakeHomeserver) reset() {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.requests = nil
}

// find returns the requests with the method whose path contains the text
func (hs *fakeHomeserver) find(method, pathPart string) []fakeRequest {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	var found []fakeRequest
	for _, req := range hs.requests {
		if req.method == method && strings.Contains(req.path, pathPart) {
			found = append(found, req)
		}
	}
	return found
}

// decode reads the JSON body of the request
func (req fakeRequest) decode(t *testing.T) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(req.body, &body); err != nil {
		t.Fatalf("the body of %s %s is not JSON: %v", req.method, req.path, err)
	}
	return body
}
