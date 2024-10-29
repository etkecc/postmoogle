package config

import (
	"time"

	"github.com/etkecc/go-env"
)

const prefix = "postmoogle"

// New config
func New() *Config {
	env.SetPrefix(prefix)

	cfg := &Config{
		Homeserver:   env.String("homeserver", defaultConfig.Homeserver),
		Login:        env.String("login", defaultConfig.Login),
		Password:     env.String("password", defaultConfig.Password),
		SharedSecret: env.String("sharedsecret", defaultConfig.SharedSecret),
		Prefix:       env.String("prefix", defaultConfig.Prefix),
		Domains:      migrateDomains("domain", "domains"),
		Port:         env.String("port", defaultConfig.Port),
		Proxies:      env.Slice("proxies"),
		DKIM: DKIM{
			PrivKey:   env.String("dkim.privkey", defaultConfig.DKIM.PrivKey),
			Signature: env.String("dkim.signature", defaultConfig.DKIM.Signature),
		},
		DataSecret: env.String("data.secret", defaultConfig.DataSecret),
		MaxSize:    env.Int("maxsize", defaultConfig.MaxSize),
		StatusMsg:  env.String("statusmsg", defaultConfig.StatusMsg),
		Admins:     env.Slice("admins"),
		Mailboxes: Mailboxes{
			Reserved:   env.Slice("mailboxes.reserved"),
			Forwarded:  env.Slice("mailboxes.forwarded"),
			Activation: env.String("mailboxes.activation", defaultConfig.Mailboxes.Activation),
		},
		TLS: TLS{
			Certs:    env.Slice("tls.cert"),
			Keys:     env.Slice("tls.key"),
			Required: env.Bool("tls.required"),
			Port:     env.String("tls.port", defaultConfig.TLS.Port),
		},
		Monitoring: Monitoring{
			SentryDSN:            env.String("monitoring.sentry.dsn", env.String("sentry.dsn", "")),
			SentrySampleRate:     env.Int("monitoring.sentry.rate", env.Int("sentry.rate", 0)),
			HealthchecksURL:      env.String("monitoring.healthchecks.url", defaultConfig.Monitoring.HealthchecksURL),
			HealthchecksUUID:     env.String("monitoring.healthchecks.uuid"),
			HealthchecksDuration: time.Duration(env.Int("monitoring.healthchecks.duration", int(defaultConfig.Monitoring.HealthchecksDuration))) * time.Second,
		},
		LogLevel: env.String("loglevel", defaultConfig.LogLevel),
		DB: DB{
			DSN:     env.String("db.dsn", defaultConfig.DB.DSN),
			Dialect: env.String("db.dialect", defaultConfig.DB.Dialect),
		},
		PSD: PSD{
			URL:      env.String("psd.url"),
			Login:    env.String("psd.login"),
			Password: env.String("psd.password"),
		},
		Relay: Relay{
			Host:     env.String("relay.host", defaultConfig.Relay.Host),
			Port:     env.String("relay.port", defaultConfig.Relay.Port),
			Username: env.String("relay.username", defaultConfig.Relay.Username),
			Password: env.String("relay.password", defaultConfig.Relay.Password),
		},
	}

	return cfg
}

func migrateDomains(oldKey, newKey string) []string {
	domains := []string{}
	old := env.String(oldKey, "")
	if old != "" {
		domains = append(domains, old)
	}

	return append(domains, env.Slice(newKey)...)
}
