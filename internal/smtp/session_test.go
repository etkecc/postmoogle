package smtp

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/email"
)

// fakebot is a test stub implementing matrixbot. Every method panics by
// default so that any unexpected interaction fails the test loudly.
// Override individual methods via the function fields where a specific
// test needs non-default behavior.
type fakebot struct {
	getMapping func(context.Context, string) (id.RoomID, bool)
}

func (f *fakebot) AllowAuth(context.Context, string, string) (id.RoomID, bool) {
	panic("AllowAuth: unexpected call")
}

func (f *fakebot) IsGreylisted(context.Context, net.Addr) bool {
	panic("IsGreylisted: unexpected call")
}

func (f *fakebot) IsBanned(context.Context, net.Addr) bool {
	panic("IsBanned: unexpected call")
}

func (f *fakebot) IsTrusted(net.Addr) bool { panic("IsTrusted: unexpected call") }

func (f *fakebot) BanAuto(context.Context, net.Addr) {
	panic("BanAuto: unexpected call")
}

func (f *fakebot) BanAuth(context.Context, net.Addr) {
	panic("BanAuth: unexpected call")
}

func (f *fakebot) GetMapping(ctx context.Context, mailbox string) (id.RoomID, bool) {
	if f.getMapping != nil {
		return f.getMapping(ctx, mailbox)
	}
	panic("GetMapping: unexpected call")
}

func (f *fakebot) GetIFOptions(context.Context, id.RoomID) email.IncomingFilteringOptions {
	panic("GetIFOptions: unexpected call")
}

func (f *fakebot) IncomingEmail(context.Context, *email.Email) error {
	panic("IncomingEmail: unexpected call")
}

func (f *fakebot) GetDKIMprivkey(context.Context) string {
	panic("GetDKIMprivkey: unexpected call")
}

func (f *fakebot) GetRelayConfig(context.Context, id.RoomID) *url.URL {
	panic("GetRelayConfig: unexpected call")
}

// newTestSession builds a session without a live *smtp.Conn. Callers
// must avoid Mail() code paths that dereference s.conn (invalid-format
// branch).
func newTestSession(bot matrixbot, domains []string, dir string) *session {
	log := zerolog.Nop()
	return &session{
		log:     &log,
		bot:     bot,
		ctx:     context.Background(),
		domains: domains,
		dir:     dir,
	}
}

func TestMailAuthenticatedLocalDomainDelegatesToOutgoingValidation(t *testing.T) {
	room := id.RoomID("!room:example.com")
	bot := &fakebot{
		getMapping: func(_ context.Context, mb string) (id.RoomID, bool) {
			if mb != "alice" {
				t.Fatalf("unexpected mailbox: %q", mb)
			}
			return room, true
		},
	}
	s := newTestSession(bot, []string{"example.com"}, Outoing)
	s.fromRoom = room

	if err := s.Mail("alice@example.com", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMailUnauthenticatedLocalDomainRejected(t *testing.T) {
	s := newTestSession(&fakebot{}, []string{"example.com", "other.net"}, "")

	err := s.Mail("admin@example.com", nil)
	if !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("expected ErrAuthRequired, got %v", err)
	}
	if s.from != "" {
		t.Fatalf("s.from must not be set on reject, got %q", s.from)
	}
}

func TestMailUnauthenticatedRemoteDomainAccepted(t *testing.T) {
	s := newTestSession(&fakebot{}, []string{"example.com"}, "")

	if err := s.Mail("someone@external.org", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.from != "someone@external.org" {
		t.Fatalf("expected s.from to be populated, got %q", s.from)
	}
}
