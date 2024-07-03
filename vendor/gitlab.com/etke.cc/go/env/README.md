# ENV

Simple go environment variables reader, for env-based configs

Automatically loads .env file if exists

```go
env.SetPrefix("app") // all env vars should start with APP_
login := env.String("matrix.login", "none") // export APP_MATRIX_LOGIN=buscarron
enabled := env.Bool("form.enabled") // export APP_FORM_ENABLED=1
size := env.Int("cache.size", 1000) // export APP_CACHE_SIZE=500
slice := env.Slice("form.fields") // export APP_FORM_FIELDS="one two three"

// need to load custom env file?
dotenv.Load(".env.dev", ".env.local")
```

see more in godoc
