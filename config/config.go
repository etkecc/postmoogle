package config

import (
	"gitlab.com/etke.cc/go/env"
)

const prefix = "postmoogle"

// New config
func New() *Config {
	env.SetPrefix(prefix)
	cfg := &Config{
		Homeserver:   env.String("homeserver", defaultConfig.Homeserver),
		Login:        env.String("login", defaultConfig.Login),
		Password:     env.String("password", defaultConfig.Password),
		Prefix:       env.String("prefix", defaultConfig.Prefix),
		Domain:       env.String("domain", defaultConfig.Domain),
		Port:         env.String("port", defaultConfig.Port),
		NoEncryption: env.Bool("noencryption"),
		NoOwner:      env.Bool("noowner"),
		Federation:   env.Bool("federation"),
		MaxSize:      env.Int("maxsize", defaultConfig.MaxSize),
		Sentry: Sentry{
			DSN: env.String("sentry.dsn", defaultConfig.Sentry.DSN),
		},
		LogLevel: env.String("loglevel", defaultConfig.LogLevel),
		DB: DB{
			DSN:     env.String("db.dsn", defaultConfig.DB.DSN),
			Dialect: env.String("db.dialect", defaultConfig.DB.Dialect),
		},
	}

	return cfg
}
