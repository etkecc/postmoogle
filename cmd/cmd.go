package main

import (
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mileusna/crontab"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/linkpearl"
	lpcfg "gitlab.com/etke.cc/linkpearl/config"

	"gitlab.com/etke.cc/postmoogle/bot"
	"gitlab.com/etke.cc/postmoogle/config"
	"gitlab.com/etke.cc/postmoogle/smtp"
	"gitlab.com/etke.cc/postmoogle/utils"
)

var (
	mxb   *bot.Bot
	cron  *crontab.Crontab
	smtpm *smtp.Manager
	log   *logger.Logger
)

func main() {
	quit := make(chan struct{})

	cfg := config.New()
	log = logger.New("postmoogle.", cfg.LogLevel)
	utils.SetLogger(log)
	utils.SetDomains(cfg.Domains)

	log.Info("#############################")
	log.Info("Postmoogle")
	log.Info("Matrix: true")
	log.Info("#############################")

	log.Debug("starting internal components...")
	initSentry(cfg)
	initBot(cfg)
	initSMTP(cfg)
	initCron()
	initShutdown(quit)
	defer recovery()

	go startBot(cfg.StatusMsg)

	if err := smtpm.Start(); err != nil {
		//nolint:gocritic
		log.Fatal("SMTP server crashed: %v", err)
	}

	<-quit
}

func initSentry(cfg *config.Config) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.Sentry.DSN,
		AttachStacktrace: true,
	})
	if err != nil {
		log.Fatal("cannot initialize sentry: %v", err)
	}
}

func initBot(cfg *config.Config) {
	db, err := sql.Open(cfg.DB.Dialect, cfg.DB.DSN)
	if err != nil {
		log.Fatal("cannot initialize SQL database: %v", err)
	}
	mxlog := logger.New("matrix.", cfg.LogLevel)
	lp, err := linkpearl.New(&lpcfg.Config{
		Homeserver:        cfg.Homeserver,
		Login:             cfg.Login,
		Password:          cfg.Password,
		DB:                db,
		Dialect:           cfg.DB.Dialect,
		NoEncryption:      cfg.NoEncryption,
		AccountDataSecret: cfg.DataSecret,
		LPLogger:          mxlog,
		APILogger:         logger.New("api.", "INFO"),
		StoreLogger:       logger.New("store.", "INFO"),
		CryptoLogger:      logger.New("olm.", "INFO"),
	})
	if err != nil {
		// nolint // Fatal = panic, not os.Exit()
		log.Fatal("cannot initialize matrix bot: %v", err)
	}

	mxb, err = bot.New(lp, mxlog, cfg.Prefix, cfg.Domains, cfg.Admins)
	if err != nil {
		// nolint // Fatal = panic, not os.Exit()
		log.Fatal("cannot start matrix bot: %v", err)
	}
	log.Debug("bot has been created")
}

func initSMTP(cfg *config.Config) {
	smtpm = smtp.NewManager(&smtp.Config{
		Domains:     cfg.Domains,
		Port:        cfg.Port,
		TLSCerts:    cfg.TLS.Certs,
		TLSKeys:     cfg.TLS.Keys,
		TLSPort:     cfg.TLS.Port,
		TLSRequired: cfg.TLS.Required,
		LogLevel:    cfg.LogLevel,
		MaxSize:     cfg.MaxSize,
		Bot:         mxb,
	})
}

func initCron() {
	cron = crontab.New()

	err := cron.AddJob("* * * * *", mxb.ProcessQueue)
	if err != nil {
		log.Error("cannot start ProcessQueue cronjob: %v", err)
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
	log.Debug("starting matrix bot: %s...", statusMsg)
	err := mxb.Start(statusMsg)
	if err != nil {
		//nolint:gocritic
		log.Fatal("cannot start the bot: %v", err)
	}
}

func shutdown() {
	log.Info("Shutting down...")
	cron.Shutdown()
	smtpm.Stop()
	mxb.Stop()

	sentry.Flush(5 * time.Second)
	log.Info("Postmoogle has been stopped")
	os.Exit(0)
}

func recovery() {
	defer shutdown()
	err := recover()
	// no problem just shutdown
	if err == nil {
		sentry.CurrentHub().Recover(err)
	}
}
