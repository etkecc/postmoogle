package config

import (
	"fmt"

	"gitlab.com/etke.cc/go/env"
	"gitlab.com/etke.cc/postmoogle/utils"
)

const prefix = "postmoogle"

// New config
func New() (*Config, error) {
	env.SetPrefix(prefix)

	wildCardUserPatterns := env.Slice("users")
	regexUserPatterns, err := utils.WildcardUserPatternsToRegexPatterns(wildCardUserPatterns)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to convert wildcard user patterns (`%s`) to regular expression: %s",
			wildCardUserPatterns,
			err,
		)
	}

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
		StatusMsg:    env.String("statusmsg", defaultConfig.StatusMsg),
		Users:        *regexUserPatterns,
		Sentry: Sentry{
			DSN: env.String("sentry.dsn", defaultConfig.Sentry.DSN),
		},
		LogLevel: env.String("loglevel", defaultConfig.LogLevel),
		DB: DB{
			DSN:     env.String("db.dsn", defaultConfig.DB.DSN),
			Dialect: env.String("db.dialect", defaultConfig.DB.Dialect),
		},
	}

	return cfg, nil
}
