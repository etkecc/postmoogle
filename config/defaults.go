package config

var defaultConfig = &Config{
	LogLevel: "INFO",
	Domain:   "localhost",
	Port:     "25",
	Prefix:   "!pm",
	MaxSize:  1024,
	DB: DB{
		DSN:     "local.db",
		Dialect: "sqlite3",
	},
	Sentry: Sentry{
		SampleRate: 20,
	},
}
