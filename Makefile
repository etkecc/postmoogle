### CI vars
CI_LOGIN_COMMAND = @echo "Not a CI, skip login"
CI_REGISTRY_IMAGE ?= registry.gitlab.com/etke.cc/postmoogle
REGISTRY_IMAGE ?= registry.etke.cc/etke.cc/postmoogle
CI_COMMIT_TAG ?= latest
# for main branch it must be set explicitly
ifeq ($(CI_COMMIT_TAG), main)
CI_COMMIT_TAG = latest
endif
# login command
ifdef CI_JOB_TOKEN
CI_LOGIN_COMMAND = @docker login -u gitlab-ci-token -p $(CI_JOB_TOKEN) $(CI_REGISTRY)
endif

# update go dependencies
update:
	go get ./cmd
	go mod tidy
	go mod verify
	go mod vendor

mock:
	-@rm -rf mocks
	@mockery --all

# run linter
lint:
	golangci-lint run ./...

# run linter and fix issues if possible
lintfix:
	golangci-lint run --fix ./...

# run unit tests
test:
	@go test -coverprofile=cover.out ./...
	@go tool cover -func=cover.out
	-@rm -f cover.out

# note: make doesn't understand exit code 130 and sets it == 1
run:
	@go run ./cmd || exit 0

build:
	go build -v -o postmoogle ./cmd

# CI: docker login
login:
	@echo "trying to login to docker registry..."
	$(CI_LOGIN_COMMAND)

# docker build
docker:
	docker buildx create --use
	docker buildx build --platform linux/arm64/v8,linux/amd64 --push -t ${CI_REGISTRY_IMAGE}:${CI_COMMIT_TAG} -t ${REGISTRY_IMAGE}:${CI_COMMIT_TAG} .
