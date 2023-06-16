# show help by default
default:
    @just --list --justfile {{ justfile() }}

# update go deps
update:
    go get .
    go get maunium.net/go/mautrix@latest
    go mod tidy

# run linter
lint:
    golangci-lint run ./...

# automatically fix liter issues
lintfix:
    golangci-lint run --fix ./...

vuln:
    govulncheck ./...

# run unit tests
test:
    @go test ${BUILDFLAGS} -coverprofile=cover.out ./...
    @go tool cover -func=cover.out
    -@rm -f cover.out
