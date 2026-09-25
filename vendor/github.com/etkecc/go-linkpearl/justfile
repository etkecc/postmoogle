# show help by default
default:
    @just --list --justfile {{ justfile() }}

# update go deps
update *flags:
    go get {{ flags }} .
    go mod tidy

# run linter
lint: comment-guard
    golangci-lint run ./...

# allow single-line comments only, max 120 chars (exemptions in the case block below).
comment-guard:
    #!/usr/bin/env sh
    set -u
    out=$(
      git ls-files -- '*.go' '*.rs' '*.ts' '*.tsx' '*.py' '*.cpp' \
      | while IFS= read -r f; do
          case "$f" in
            */vendor/*|vendor/*) continue ;;
            */node_modules/*|node_modules/*) continue ;;
            */mocks/*|mocks/*) continue ;;
            */automock/*|automock/*) continue ;;
            */target/*|target/*) continue ;;
            */dist/*|dist/*) continue ;;
            */build/*|build/*) continue ;;
            *.pb.go) continue ;;
            */docs/docs.go) continue ;;
          esac
          [ -f "$f" ] || continue
          if head -n 10 "$f" | grep -qE '^(//|#) Code generated .* DO NOT EDIT\.$'; then
            continue
          fi
          case "$f" in *.py) ispy=1 ;; *) ispy=0 ;; esac
          awk -v PY="$ispy" -v DQ='"""' -v SQ="'''" '
            function report(ln, msg) { printf "%s:%d: %s\n", FILENAME, ln, msg }
            function chk(n) { if (length($0) > MAXLEN) report(n, "comment over " MAXLEN " chars") }
            BEGIN { MAXLEN = 120; MAXDOC = 5 }
            {
              t = $0
              sub(/^[ \t]+/, "", t)
              if (PY == "1") {
                if (in_doc) {
                  doc_lines++
                  chk(NR)
                  if (index($0, QT) > 0) {
                    in_doc = 0
                    if (doc_lines > MAXDOC) report(doc_start, "docstring over " MAXDOC " lines; keep it short")
                  }
                  next
                }
                p = t
                while (length(p) > 3 && index("rRbBuUfF", substr(p, 1, 1)) > 0) p = substr(p, 2)
                q = ""
                if (substr(p, 1, 3) == DQ) { q = DQ } else if (substr(p, 1, 3) == SQ) { q = SQ }
                if (q != "") {
                  if (index(substr(p, 4), q) > 0) { chk(NR); run = 0; next }
                  in_doc = 1; QT = q; doc_lines = 1; doc_start = NR; chk(NR); run = 0; next
                }
                if (substr(t, 1, 1) == "#") {
                  if (substr(t, 1, 2) == "#!") { run = 0; next }
                  if (t ~ /^#[ \t]*(noqa|nosec|type:|pragma:|fmt:|isort:|pylint:|mypy:|ruff:|flake8:|coding[:=]|-\*-)/) { run = 0; next }
                  chk(NR)
                  run++
                  if (run == 1) run_start = NR
                  if (run == 2) report(run_start, "multi-line comment; single-line only")
                } else {
                  run = 0
                }
                next
              }
              if (in_block) {
                chk(NR)
                if (index($0, "*/") > 0) {
                  in_block = 0
                  report(block_start, "multi-line /* */ comment; single-line only")
                }
                next
              }
              if (substr(t, 1, 2) == "/*") {
                if (index(substr(t, 3), "*/") > 0) { chk(NR); run = 0; next }
                in_block = 1; block_start = NR; chk(NR); next
              }
              if (substr(t, 1, 2) == "//") {
                if (t ~ /^\/\/[ \t]*$/) { next }
                if (t ~ /^\/\/[a-z]+:/ || t ~ /^\/\/(line|export|cgo|sys)[ \t]/ || t ~ /^\/\/\/[ \t]*</ || t ~ /^\/\/[ \t]*@[A-Za-z]/) { run = 0; next }
                chk(NR)
                run++
                if (run == 1) run_start = NR
                if (run == 2) report(run_start, "multi-line comment; single-line only")
                next
              }
              run = 0
            }
          ' "$f"
        done
    )
    if [ -n "$out" ]; then
      printf '%s\n' "$out" | sort
      echo "comment-guard: FAIL (single-line comments only, max 120 chars, docstrings max 5 lines)"
      exit 1
    fi

# automatically fix liter issues
lintfix:
    golangci-lint run --fix ./...

# run unit tests
test:
    @go test -cover -coverprofile=cover.out -coverpkg=./... -covermode=set ./...
    @go tool cover -func=cover.out
    -@rm -f cover.out
