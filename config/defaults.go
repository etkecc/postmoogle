package config

var defaultConfig = &Config{
	LogLevel:  "INFO",
	Domains:   []string{"localhost"},
	Port:      "25",
	Prefix:    "!pm",
	MaxSize:   1024,
	StatusMsg: "Delivering emails",
	DB: DB{
		DSN:     "local.db",
		Dialect: "sqlite3",
	},
	TLS: TLS{
		Port: "587",
	},
}
