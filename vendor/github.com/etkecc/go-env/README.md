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
if err := envfile.Load(".env.dev", ".env.local"); err != nil {
	log.Println(err)
}
```

see more in godoc

## .env format

`.env` is read first, then every additional file in the order given. Every value is written into the
process environment, overriding whatever was there, so a `.env` file always wins over the environment
the process was started with.

The file is trusted input: everything in it lands in the process environment verbatim, and child
processes inherit it, so anything able to write `.env` can influence the process (including variables
like `PATH`).

```sh
# comment
export APP_LOGIN=user
APP_PORT: 8080
APP_SECRET="p@ss # not a comment"
APP_REGEX='^\d+$'
APP_TIMEOUT=$APP_PORT
```

- `KEY=value` and `KEY: value`, an optional `export ` prefix, blank lines, and `#` comments (whole
  line, or trailing after whitespace)
- LF, CRLF and CR line endings, a missing trailing newline, and a UTF-8 BOM are all accepted
- single quotes keep the value raw: no escapes and no expansion; use double quotes for values that
  contain a `'`
- double quotes decode `\n`, `\r`, `\t`, `\\`, `\"` and `\$`; every other backslash stays as written,
  so `"C:\Temp"` and `"\d+"` survive; a value ending in a backslash must be written `"foo\\"`
- unquoted values are trimmed of surrounding whitespace, and a `#` preceded by whitespace starts an
  inline comment; right after a quoted value a `#` starts one too
- `$NAME` and `${NAME}` expand from variables defined earlier in the same file; names are ASCII and may
  not start with a digit. Unknown references, `$5` and `$2a$10$...` stay as written, and the process
  environment is never consulted
- a broken line is skipped and reported with its line number, the rest of the file still loads; a
  missing file is not an error

`Load` returns those errors. The `env` package calls it at init, logs failures to stderr and carries on,
so importing it never fails.
