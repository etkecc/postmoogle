package smtp

import (
	"strings"

	"github.com/rs/zerolog"
)

// validatorLoggerWrapper is a wrapper around zerolog.Logger to implement validator.Logger interface
type validatorLoggerWrapper struct {
	log *zerolog.Logger
}

func (l validatorLoggerWrapper) Info(msg string, args ...any) {
	l.log.Info().Msgf(msg, args...)
}

func (l validatorLoggerWrapper) Error(msg string, args ...any) {
	l.log.Error().Msgf(msg, args...)
}

// loggerWrapper is a wrapper around any logger to implement smtp.Logger interface
type loggerWrapper struct {
	log func(string, ...any)
}

func (l loggerWrapper) Printf(format string, v ...any) {
	l.log(format, v...)
}

func (l loggerWrapper) Println(v ...any) {
	msg := strings.Repeat("%v ", len(v))
	l.log(msg, v...)
}

// loggerWriter is a wrapper around io.Writer to implement io.Writer interface
type loggerWriter struct {
	log func(string)
}

func (l loggerWriter) Write(p []byte) (n int, err error) {
	l.log(string(p))
	return len(p), nil
}
