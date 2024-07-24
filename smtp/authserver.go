package smtp

import (
	"bytes"
	"context"
	"errors"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// ensure that PlainAuthServer implements sasl.Server
var _ sasl.Server = (*PlainAuthServer)(nil)

// PlainAuthServer is a server implementation of the PLAIN authentication mechanism.
// It's a modified version of the original plainServer from https://github.com/emersion/go-sasl package (MIT License)
// ref: https://github.com/emersion/go-sasl/blob/e73c9f7bad438a9bf3f5b28e661b74d752ecafdd/plain.go
// The reason for modification is to extend automatic banning mechanism of Postmoogle, as the original implementation
// doesn't provide a way to return an error to the caller before the actual authentication process.
type PlainAuthServer struct {
	done         bool
	ctx          context.Context //nolint:containedctx // that's per-request structure
	bot          matrixbot
	conn         *smtp.Conn
	authenticate sasl.PlainAuthenticator
}

// NewPlainAuthServer creates a new PLAIN authentication server.
func NewPlainAuthServer(ctx context.Context, bot matrixbot, conn *smtp.Conn, auth sasl.PlainAuthenticator) *PlainAuthServer {
	return &PlainAuthServer{
		ctx:          ctx,
		bot:          bot,
		conn:         conn,
		authenticate: auth,
	}
}

func (a *PlainAuthServer) Next(response []byte) (challenge []byte, done bool, err error) {
	if a.done {
		err = sasl.ErrUnexpectedClientResponse
		return
	}
	// No initial response, send an empty challenge
	if response == nil {
		return []byte{}, false, nil
	}

	a.done = true

	parts := bytes.Split(response, []byte("\x00"))
	if len(parts) != 3 {
		a.bot.BanAuth(a.ctx, a.conn.Conn().RemoteAddr())
		err = errors.New("sasl: invalid response. Don't bother me anymore, kupo")
		return
	}

	identity := string(parts[0])
	username := string(parts[1])
	password := string(parts[2])

	err = a.authenticate(identity, username, password)
	done = true
	return
}
