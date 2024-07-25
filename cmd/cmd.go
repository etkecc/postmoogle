package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	zlogsentry "github.com/archdx/zerolog-sentry"
	"github.com/getsentry/sentry-go"
	_ "github.com/lib/pq"
	"github.com/mileusna/crontab"
	"github.com/rs/zerolog"
	"gitlab.com/etke.cc/go/healthchecks/v2"
	"gitlab.com/etke.cc/go/psd"
	"gitlab.com/etke.cc/linkpearl"
	_ "modernc.org/sqlite"

	"gitlab.com/etke.cc/postmoogle/bot"
	mxconfig "gitlab.com/etke.cc/postmoogle/bot/config"
	"gitlab.com/etke.cc/postmoogle/bot/queue"
	"gitlab.com/etke.cc/postmoogle/config"
	"gitlab.com/etke.cc/postmoogle/smtp"
	"gitlab.com/etke.cc/postmoogle/utils"
)

var (
	q     *queue.Queue
	hc    *healthchecks.Client
	mxc   *mxconfig.Manager
	mxb   *bot.Bot
	cron  *crontab.Crontab
	smtpm *smtp.Manager
	log   zerolog.Logger
)

func main() {
	quit := make(chan struct{})

	cfg := config.New()
	initLog(cfg)
	utils.SetDomains(cfg.Domains)

	log.Info().Msg("#############################")
	log.Info().Msg("Postmoogle")
	log.Info().Msg("Matrix: true")
	log.Info().Msg("#############################")

	log.Debug().Msg("starting internal components...")
	initHealthchecks(cfg)
	initMatrix(cfg)
	initSMTP(cfg)
	initCron()
	initShutdown(quit)
	defer recovery()

	go startBot(cfg.StatusMsg)

	if err := smtpm.Start(); err != nil {
		//nolint:gocritic
		log.Fatal().Err(err).Msg("SMTP server crashed")
	}

	<-quit
}

func initLog(cfg *config.Config) {
	loglevel, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		loglevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(loglevel)
	var w io.Writer
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, PartsExclude: []string{zerolog.TimestampFieldName}}
	sentryWriter, err := zlogsentry.New(cfg.Monitoring.SentryDSN)
	if err == nil {
		w = io.MultiWriter(sentryWriter, consoleWriter)
	} else {
		w = consoleWriter
	}
	log = zerolog.New(w).With().Timestamp().Caller().Logger()
}

func initHealthchecks(cfg *config.Config) {
	if cfg.Monitoring.HealthchecksUUID == "" {
		return
	}
	hc = healthchecks.New(
		healthchecks.WithBaseURL(cfg.Monitoring.HealthchecksURL),
		healthchecks.WithCheckUUID(cfg.Monitoring.HealthchecksUUID),
		healthchecks.WithErrLog(func(operation string, err error) {
			log.Error().Err(err).Str("operation", operation).Msg("healthchecks operation failed")
		}),
	)
	hc.Start(strings.NewReader("starting postmoogle"))
	go hc.Auto(cfg.Monitoring.HealthchecksDuration)
}

func initMatrix(cfg *config.Config) {
	if cfg.DB.Dialect == "sqlite3" {
		cfg.DB.Dialect = "sqlite"
	}
	db, err := sql.Open(cfg.DB.Dialect, cfg.DB.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot initialize SQL database")
	}
	if cfg.DB.Dialect == "sqlite" {
		db.SetMaxOpenConns(1)
	}

	lp, err := linkpearl.New(&linkpearl.Config{
		Homeserver:        cfg.Homeserver,
		Login:             cfg.Login,
		Password:          cfg.Password,
		SharedSecret:      cfg.SharedSecret,
		DB:                db,
		Dialect:           cfg.DB.Dialect,
		AccountDataSecret: cfg.DataSecret,
		Logger:            log,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("cannot initialize matrix bot")
	}

	psdc := psd.NewClient(cfg.PSD.URL, cfg.PSD.Login, cfg.PSD.Password)
	mxc = mxconfig.New(lp, &log)
	q = queue.New(lp, mxc, &log)
	mxb, err = bot.New(q, lp, &log, mxc, psdc, cfg.Proxies, cfg.Prefix, cfg.Domains, cfg.Admins, bot.MBXConfig(cfg.Mailboxes))
	if err != nil {
		log.Panic().Err(err).Msg("cannot start matrix bot")
	}
	log.Debug().Msg("bot has been created")
}

func initSMTP(cfg *config.Config) {
	smtpm = smtp.NewManager(&smtp.Config{
		Domains:     cfg.Domains,
		Port:        cfg.Port,
		TLSCerts:    cfg.TLS.Certs,
		TLSKeys:     cfg.TLS.Keys,
		TLSPort:     cfg.TLS.Port,
		TLSRequired: cfg.TLS.Required,
		Logger:      &log,
		MaxSize:     cfg.MaxSize,
		Bot:         mxb,
		Callers:     []smtp.Caller{mxb, q},
		Relay: &smtp.RelayConfig{
			Host:     cfg.Relay.Host,
			Port:     cfg.Relay.Port,
			Username: cfg.Relay.Username,
			Password: cfg.Relay.Password,
		},
	})
}

func initCron() {
	cron = crontab.New()

	err := cron.AddJob("* * * * *", q.Process)
	if err != nil {
		log.Error().Err(err).Msg("cannot start queue processing cronjob")
	}

	err = cron.AddJob("*/5 * * * *", mxb.SyncRooms)
	if err != nil {
		log.Error().Err(err).Msg("cannot start sync rooms cronjob")
	}
}

func initShutdown(quit chan struct{}) {
	listener := make(chan os.Signal, 1)
	signal.Notify(listener, os.Interrupt, syscall.SIGABRT, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	go func() {
		<-listener
		defer close(quit)

		shutdown()
	}()
}

func startBot(statusMsg string) {
	log.Debug().Str("status message", statusMsg).Msg("starting matrix bot...")
	err := mxb.Start(statusMsg)
	if err != nil {
		//nolint:gocritic
		log.Panic().Err(err).Msg("cannot start the bot")
	}
}

func shutdown() {
	log.Info().Msg("Shutting down...")
	cron.Shutdown()
	smtpm.Stop()
	mxb.Stop()
	if hc != nil {
		hc.Shutdown()
		hc.ExitStatus(0, strings.NewReader("shutting down postmoogle"))
	}

	sentry.Flush(5 * time.Second)
	log.Info().Msg("Postmoogle has been stopped")
	os.Exit(0)
}

func recovery() {
	defer shutdown()
	err := recover()
	if err != nil {
		if hc != nil {
			hc.ExitStatus(1, strings.NewReader(fmt.Sprintf("panic: %v", err)))
		}
		sentry.CurrentHub().Recover(err)
	}
}
