platforms := env_var_or_default("PLATFORMS", "linux/amd64")
tag := if env_var_or_default("CI_COMMIT_TAG", "main") == "main" { "latest" } else { env_var_or_default("CI_COMMIT_TAG", "latest") }
repo := trim_end_match(replace(replace_regex(env_var_or_default("CI_REPOSITORY_URL", `git remote get-url origin`), ".*@|", ""), ":", "/"), ".git")
project := file_name(repo)
gitlab_image := "registry." + repo + ":" + tag
etke_image := replace(gitlab_image, "gitlab.com", "etke.cc")

# show help by default
default:
    @just --list --justfile {{ justfile() }}

# update go deps
update *flags:
    go get {{ flags }} ./cmd
    go mod tidy
    go mod vendor

# run linter
lint:
    golangci-lint run ./...

# automatically fix liter issues
lintfix:
    golangci-lint run --fix ./...

# generate mocks
mocks:
    @mockery --all --inpackage --testonly --exclude vendor

# run cpu or mem profiler UI
profile type:
    go tool pprof -http 127.0.0.1:8000 .pprof/{{ type }}.prof

# run unit tests
test packages="./...":
    @go test -cover -coverprofile=cover.out -coverpkg={{ packages }} -covermode=set {{ packages }}
    @go tool cover -func=cover.out
    -@rm -f cover.out

# run app
run:
    @CGO_ENABLED=0 go run -ldflags '-extldflags "-static"' -tags timetzdata,goolm ./cmd

# build app
build:
    CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -tags timetzdata,goolm -v -o {{ project }} ./cmd

# docker login
login:
    @docker login -u gitlab-ci-token -p $CI_JOB_TOKEN $CI_REGISTRY

# docker build
docker:
    docker buildx create --use
    docker buildx build --pull --provenance=false --platform {{ platforms }} --push -t {{ gitlab_image }} -t {{ etke_image }} .
    docker buildx rm
