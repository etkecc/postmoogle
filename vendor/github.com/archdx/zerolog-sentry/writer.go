package zlogsentry

import (
	"io"
	"time"
	"unsafe"

	"github.com/buger/jsonparser"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
)

var levelsMapping = map[zerolog.Level]sentry.Level{
	zerolog.DebugLevel: sentry.LevelDebug,
	zerolog.InfoLevel:  sentry.LevelInfo,
	zerolog.WarnLevel:  sentry.LevelWarning,
	zerolog.ErrorLevel: sentry.LevelError,
	zerolog.FatalLevel: sentry.LevelFatal,
	zerolog.PanicLevel: sentry.LevelFatal,
}

var _ = io.WriteCloser(new(Writer))

var now = time.Now

// Writer is a sentry events writer with std io.Writer iface.
type Writer struct {
	hub *sentry.Hub

	levels       map[zerolog.Level]struct{}
	flushTimeout time.Duration
}

// Write handles zerolog's json and sends events to sentry.
func (w *Writer) Write(data []byte) (int, error) {
	event, ok := w.parseLogEvent(data)
	if ok {
		w.hub.CaptureEvent(event)
		// should flush before os.Exit
		if event.Level == sentry.LevelFatal {
			w.hub.Flush(w.flushTimeout)
		}
	}

	return len(data), nil
}

// Close forces client to flush all pending events.
// Can be useful before application exits.
func (w *Writer) Close() error {
	w.hub.Flush(w.flushTimeout)
	return nil
}

func (w *Writer) parseLogEvent(data []byte) (*sentry.Event, bool) {
	const logger = "zerolog"

	lvlStr, err := jsonparser.GetUnsafeString(data, zerolog.LevelFieldName)
	if err != nil {
		return nil, false
	}

	lvl, err := zerolog.ParseLevel(lvlStr)
	if err != nil {
		return nil, false
	}

	_, enabled := w.levels[lvl]
	if !enabled {
		return nil, false
	}

	sentryLvl, ok := levelsMapping[lvl]
	if !ok {
		return nil, false
	}

	event := sentry.Event{
		Timestamp: now(),
		Level:     sentryLvl,
		Logger:    logger,
		Extra:     map[string]interface{}{},
	}

	err = jsonparser.ObjectEach(data, func(key, value []byte, vt jsonparser.ValueType, offset int) error {
		switch string(key) {
		case zerolog.MessageFieldName:
			event.Message = bytesToStrUnsafe(value)
		case zerolog.ErrorFieldName:
			event.Exception = append(event.Exception, sentry.Exception{
				Value:      bytesToStrUnsafe(value),
				Stacktrace: newStacktrace(),
			})
		case zerolog.LevelFieldName, zerolog.TimestampFieldName:
		default:
			event.Extra[string(key)] = bytesToStrUnsafe(value)
		}

		return nil
	})

	if err != nil {
		return nil, false
	}

	return &event, true
}

func newStacktrace() *sentry.Stacktrace {
	const (
		module       = "github.com/archdx/zerolog-sentry"
		loggerModule = "github.com/rs/zerolog"
	)

	st := sentry.NewStacktrace()

	threshold := len(st.Frames) - 1
	// drop current module frames
	for ; threshold > 0 && st.Frames[threshold].Module == module; threshold-- {
	}

outer:
	// try to drop zerolog module frames after logger call point
	for i := threshold; i > 0; i-- {
		if st.Frames[i].Module == loggerModule {
			for j := i - 1; j >= 0; j-- {
				if st.Frames[j].Module != loggerModule {
					threshold = j
					break outer
				}
			}

			break
		}
	}

	st.Frames = st.Frames[:threshold+1]

	return st
}

func bytesToStrUnsafe(data []byte) string {
	return *(*string)(unsafe.Pointer(&data))
}

// WriterOption configures sentry events writer.
type WriterOption interface {
	apply(*config)
}

type optionFunc func(*config)

func (fn optionFunc) apply(c *config) { fn(c) }

type config struct {
	levels       []zerolog.Level
	sampleRate   float64
	release      string
	environment  string
	serverName   string
	ignoreErrors []string
	debug        bool
	flushTimeout time.Duration
}

// WithLevels configures zerolog levels that have to be sent to Sentry.
// Default levels are: error, fatal, panic.
func WithLevels(levels ...zerolog.Level) WriterOption {
	return optionFunc(func(cfg *config) {
		cfg.levels = levels
	})
}

// WithSampleRate configures the sample rate as a percentage of events to be sent in the range of 0.0 to 1.0.
func WithSampleRate(rate float64) WriterOption {
	return optionFunc(func(cfg *config) {
		cfg.sampleRate = rate
	})
}

// WithRelease configures the release to be sent with events.
func WithRelease(release string) WriterOption {
	return optionFunc(func(cfg *config) {
		cfg.release = release
	})
}

// WithEnvironment configures the environment to be sent with events.
func WithEnvironment(environment string) WriterOption {
	return optionFunc(func(cfg *config) {
		cfg.environment = environment
	})
}

// WithServerName configures the server name field for events. Default value is OS hostname.
func WithServerName(serverName string) WriterOption {
	return optionFunc(func(cfg *config) {
		cfg.serverName = serverName
	})
}

// WithIgnoreErrors configures the list of regexp strings that will be used to match against event's message
// and if applicable, caught errors type and value. If the match is found, then a whole event will be dropped.
func WithIgnoreErrors(reList []string) WriterOption {
	return optionFunc(func(cfg *config) {
		cfg.ignoreErrors = reList
	})
}

// WithDebug enables sentry client debug logs.
func WithDebug() WriterOption {
	return optionFunc(func(cfg *config) {
		cfg.debug = true
	})
}

// New creates writer with provided DSN and options.
func New(dsn string, opts ...WriterOption) (*Writer, error) {
	cfg := newDefaultConfig()
	for _, opt := range opts {
		opt.apply(&cfg)
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:          dsn,
		SampleRate:   cfg.sampleRate,
		Release:      cfg.release,
		Environment:  cfg.environment,
		ServerName:   cfg.serverName,
		IgnoreErrors: cfg.ignoreErrors,
		Debug:        cfg.debug,
	})

	if err != nil {
		return nil, err
	}

	levels := make(map[zerolog.Level]struct{}, len(cfg.levels))
	for _, lvl := range cfg.levels {
		levels[lvl] = struct{}{}
	}

	return &Writer{
		hub:          sentry.CurrentHub(),
		levels:       levels,
		flushTimeout: cfg.flushTimeout,
	}, nil
}

func newDefaultConfig() config {
	return config{
		levels: []zerolog.Level{
			zerolog.ErrorLevel,
			zerolog.FatalLevel,
			zerolog.PanicLevel,
		},
		sampleRate:   1.0,
		flushTimeout: 3 * time.Second,
	}
}
