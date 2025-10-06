package smtp

import (
	"bytes"
	"context"
	"errors"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

const (
	loginStateNotStarted = iota
	loginStateUsername
	loginStatePassword
)

// ensure that PlainAuthServer implements sasl.Server
var (
	_ sasl.Server = (*PlainAuthServer)(nil)
	_ sasl.Server = (*LoginAuthServer)(nil)
)

// LoginAuthServer is a server implementation of the LOGIN authentication mechanism.
// It's a modified implementation of the go-sasl's original loginServer (MIT License),
// removed from go-sasl in https://github.com/emersion/go-sasl/commit/b788ff22d5a6b3970cde181998f52658a475bffc
// DO NOT USE IT, unless you have no other choice.
type LoginAuthServer struct {
	done         bool
	state        int
	ctx          context.Context //nolint:containedctx // that's per-request structure
	bot          matrixbot
	conn         *smtp.Conn
	authenticate sasl.PlainAuthenticator // we use the same interface as PLAIN auth for simplicity
	username     string
	password     string
}

// NewLoginAuthServer creates a new LOGIN authentication server.
func NewLoginAuthServer(ctx context.Context, bot matrixbot, conn *smtp.Conn, auth sasl.PlainAuthenticator) *LoginAuthServer {
	return &LoginAuthServer{
		ctx:          ctx,
		bot:          bot,
		conn:         conn,
		authenticate: auth,
		state:        loginStateNotStarted,
	}
}

// Next processes the next step of the authentication.
func (a *LoginAuthServer) Next(response []byte) (challenge []byte, done bool, err error) {
	if a.done {
		err = sasl.ErrUnexpectedClientResponse
		return challenge, done, err
	}

	switch a.state {
	case loginStateNotStarted:
		a.state = loginStateUsername
		return []byte("Username:"), false, nil

	case loginStateUsername:
		a.username = string(response)
		a.state = loginStatePassword
		return []byte("Password:"), false, nil

	case loginStatePassword:
		a.password = string(response)
		a.done = true
		err = a.authenticate("", a.username, a.password)
		done = true
		return challenge, done, err

	default:
		err = sasl.ErrUnexpectedClientResponse
		return challenge, done, err
	}
}

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

// Next processes the next step of the authentication.
func (a *PlainAuthServer) Next(response []byte) (challenge []byte, done bool, err error) {
	if a.done {
		err = sasl.ErrUnexpectedClientResponse
		return challenge, done, err
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
		return challenge, done, err
	}

	identity := string(parts[0])
	username := string(parts[1])
	password := string(parts[2])

	err = a.authenticate(identity, username, password)
	done = true
	return challenge, done, err
}
