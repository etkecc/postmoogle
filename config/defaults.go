package config

var defaultConfig = &Config{
	LogLevel:  "INFO",
	Domains:   []string{"localhost"},
	Port:      "25",
	Prefix:    "!pm",
	MaxSize:   1024,
	StatusMsg: "Delivering emails",
	Mailboxes: Mailboxes{
		Activation: "none",
	},
	DB: DB{
		DSN:     "local.db",
		Dialect: "sqlite3",
	},
	Monitoring: Monitoring{
		SentrySampleRate:   20,
		HealthechsDuration: 5,
	},
	TLS: TLS{
		Port: "587",
	},
}
