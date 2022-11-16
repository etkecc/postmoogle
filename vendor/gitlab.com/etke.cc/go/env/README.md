# ENV

Simple go environment variables reader, for env-based configs

```go
env.SetPrefix("app") // all env vars should start with APP_
login := env.String("matrix.login", "none") // export APP_MATRIX_LOGIN=buscarron
enabled := env.Bool("form.enabled") // export APP_FORM_ENABLED=1
size := env.Int("cache.size", 1000) // export APP_CACHE_SIZE=500
slice := env.Slice("form.fields") // export APP_FORM_FIELDS="one two three"
```

see more in godoc
