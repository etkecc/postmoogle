# update go dependencies
update:
	go get .
	go get -u maunium.net/go/mautrix
	go mod tidy

mock:
	-@rm -rf mocks
	@mockery --all

# run linter
lint:
	golangci-lint run ./...

# run linter and fix issues if possible
lintfix:
	golangci-lint run --fix ./...

vuln:
	govulncheck ./...

# run unit tests
test:
	@go test ${BUILDFLAGS} -coverprofile=cover.out ./...
	@go tool cover -func=cover.out
	-@rm -f cover.out
