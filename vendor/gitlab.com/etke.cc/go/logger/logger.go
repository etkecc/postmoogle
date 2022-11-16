package logger

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/getsentry/sentry-go"
)

// Logger struct
type Logger struct {
	log   *log.Logger
	hub   *sentry.Hub
	level int
}

const (
	// TRACE level
	TRACE int = iota
	// DEBUG level
	DEBUG
	// INFO level
	INFO
	// WARNING level
	WARNING
	// ERROR level
	ERROR
	// FATAL level
	FATAL
)

var (
	txtLevelMap = map[string]int{
		"TRACE":   TRACE,
		"DEBUG":   DEBUG,
		"INFO":    INFO,
		"WARNING": WARNING,
		"ERROR":   ERROR,
		"FATAL":   FATAL,
	}
	levelMap = map[int]string{
		TRACE:   "TRACE",
		DEBUG:   "DEBUG",
		INFO:    "INFO",
		WARNING: "WARNING",
		ERROR:   "ERROR",
		FATAL:   "FATAL",
	}
	sentryLevelMap = map[int]sentry.Level{
		TRACE:   sentry.LevelDebug,
		DEBUG:   sentry.LevelDebug,
		INFO:    sentry.LevelInfo,
		WARNING: sentry.LevelWarning,
		ERROR:   sentry.LevelError,
		FATAL:   sentry.LevelFatal,
	}
)

// New creates new Logger object
func New(prefix string, level string, sentryHub ...*sentry.Hub) *Logger {
	levelID, ok := txtLevelMap[strings.ToUpper(level)]
	if !ok {
		levelID = INFO
	}
	var hub *sentry.Hub
	if len(sentryHub) > 0 {
		hub = sentryHub[0]
	}

	return &Logger{log: log.New(os.Stdout, prefix, 0), level: levelID, hub: hub}
}

// GetHub returns sentry hub (either attached to the logger when called New() or current sentry hub)
func (l *Logger) GetHub() *sentry.Hub {
	if l.hub == nil {
		return sentry.CurrentHub()
	}

	return l.hub
}

// GetLog returns underlying Logger object, useful in cases where log.Logger required
func (l *Logger) GetLog() *log.Logger {
	return l.log
}

// GetLevel (current)
func (l *Logger) GetLevel() string {
	return levelMap[l.level]
}

// Fatal log and exit
func (l *Logger) Fatal(message string, args ...interface{}) {
	l.log.Panicln("FATAL", fmt.Sprintf(message, args...))
}

// Error log
func (l *Logger) Error(message string, args ...interface{}) {
	// do not recover
	if strings.HasPrefix(message, "recovery()") {
		return
	}

	message = fmt.Sprintf(message, args...)
	l.GetHub().AddBreadcrumb(&sentry.Breadcrumb{
		Category: l.log.Prefix(),
		Message:  message,
		Level:    sentryLevelMap[ERROR],
	}, nil)

	if l.level > ERROR {
		return
	}

	l.log.Println("ERROR", message)
}

// Warn log
func (l *Logger) Warn(message string, args ...interface{}) {
	message = fmt.Sprintf(message, args...)
	l.GetHub().AddBreadcrumb(&sentry.Breadcrumb{
		Category: l.log.Prefix(),
		Message:  message,
		Level:    sentryLevelMap[WARNING],
	}, nil)
	if l.level > WARNING {
		return
	}

	l.log.Println("WARNING", message)
}

// Warnfln for mautrix.Logger
func (l *Logger) Warnfln(message string, args ...interface{}) {
	l.Warn(message, args...)
}

// Info log
func (l *Logger) Info(message string, args ...interface{}) {
	message = fmt.Sprintf(message, args...)
	l.GetHub().AddBreadcrumb(&sentry.Breadcrumb{
		Category: l.log.Prefix(),
		Message:  message,
		Level:    sentryLevelMap[INFO],
	}, nil)
	if l.level > INFO {
		return
	}

	l.log.Println("INFO", message)
}

// Debug log
func (l *Logger) Debug(message string, args ...interface{}) {
	message = fmt.Sprintf(message, args...)
	l.GetHub().AddBreadcrumb(&sentry.Breadcrumb{
		Category: l.log.Prefix(),
		Message:  message,
		Level:    sentryLevelMap[DEBUG],
	}, nil)
	if l.level > DEBUG {
		return
	}

	l.log.Println("DEBUG", message)
}

// Debugfln for mautrix.Logger
func (l *Logger) Debugfln(message string, args ...interface{}) {
	l.Debug(message, args...)
}

// Trace log
func (l *Logger) Trace(message string, args ...interface{}) {
	message = fmt.Sprintf(message, args...)
	l.GetHub().AddBreadcrumb(&sentry.Breadcrumb{
		Category: l.log.Prefix(),
		Message:  message,
		Level:    sentryLevelMap[TRACE],
	}, nil)
	if l.level > TRACE {
		return
	}

	l.log.Println("TRACE", message)
}
