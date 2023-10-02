tag := if env_var_or_default("CI_COMMIT_TAG", "main") == "main" { "latest" } else { env_var_or_default("CI_COMMIT_TAG", "latest") }
repo := trim_end_match(replace(replace_regex(env_var_or_default("CI_REGISTRY_IMAGE", `git remote get-url origin`), ".*@|", ""), ":", "/"),".git")
project := file_name(repo)
gitlab_image := "registry." + repo + ":" + tag
etke_image := replace(gitlab_image, "gitlab.com", "etke.cc")

try:
    @echo {{ project }}
    @echo {{ repo }}
    @echo {{ gitlab_image }}
    @echo {{ etke_image }}

# show help by default
default:
    @just --list --justfile {{ justfile() }}

# update go deps
update:
    go get ./cmd
    go get gitlab.com/etke.cc/linkpearl@latest
    go mod tidy
    go mod vendor

# run linter
lint: try
    golangci-lint run ./...

# automatically fix liter issues
lintfix:
    golangci-lint run --fix ./...

# run unit tests
test: try
    @go test -cover -coverprofile=cover.out -coverpkg=./... -covermode=set ./...
    @go tool cover -func=cover.out
    -@rm -f cover.out

# run app
run:
    @go run ./cmd

# build app
build:
    go build -v -o {{ project }} ./cmd

# docker login
login: try
    @docker login -u gitlab-ci-token -p $CI_JOB_TOKEN $CI_REGISTRY

# docker build
docker: try
    docker buildx create --use
    docker buildx build --pull --platform linux/arm64/v8,linux/amd64 --push -t {{ gitlab_image }} -t {{ etke_image }} .
