# Copied from the baseline (stack/makefile.md). Adjust per its rule 5; record
# any other deviation in the README.

# Aseprite is a GUI application on macOS, so its CLI lives in the bundle.
ASEPRITE = /Applications/Aseprite.app/Contents/MacOS/aseprite

# The main package: the little server that hosts the lab. The lab itself only
# builds for js/wasm, so it has its own target.
MAIN = ./cmd/serve

# The headline number, as a gate. A claim nothing checks is a memory.
WASM_MAX_BYTES = 320000

# Targets are alphabetical, so the default is named rather than first.
.DEFAULT_GOAL = check
.PHONY: build check ci clean fmt run sheets test wasm

# Release-shaped local binary in bin/ (go build creates the directory). The
# wildcard skips ./cmd/lab, which has no files for the host platform.
build:
	CGO_ENABLED=0 go build -trimpath -o bin/ ./cmd/...

# Default. Every gate, in this order (operations/ci.md), against the working
# tree. Run before every commit. The second vet line is this project's one
# addition: half the engine and all of the lab only build for js/wasm, so the
# host vet never sees them. It names packages rather than ./... because
# cmd/serve has no build tag, and vetting an HTTP server for js/wasm is noise.
check:
	test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	go vet ./...
	GOOS=js GOARCH=wasm go vet . ./cmd/lab/...
	go fix -diff ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go mod tidy -diff
	go test -race -shuffle=on ./...
	CGO_ENABLED=0 go build -trimpath ./...

# The same gates against the commit: a file never added, or a .env, cannot
# make it green. Run before every push. go version runs first, inside the
# copy, so the run records which toolchain ran. The archive goes through a
# file so git's exit status stops the run; one shell line so the trap cleans
# up however check ends.
ci:
	t=$$(mktemp); d=$$(mktemp -d); trap 'rm -rf "$$t" "$$d"' EXIT; git archive -o "$$t" HEAD && tar -xf "$$t" -C "$$d" && go -C "$$d" version && $(MAKE) -C "$$d" check

clean:
	rm -rf bin/

# goimports first: go fix type-checks, so a missing import would stop the
# recipe before goimports could add it. go fix manages the imports its own
# rewrites need.
fmt:
	go run golang.org/x/tools/cmd/goimports@latest -w .
	go fix ./...

# Loads .env when it is there, so a local start is one command. Only run:
# check and test MUST NOT depend on a developer's machine (rule 6). One shell
# line, because each recipe line gets its own shell.
run:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go run $(MAIN)

# The art. Aseprite is a GUI app with a batch mode; the README says how to put
# it on PATH. Each source becomes a packed sheet and the JSON that describes
# it, which is what the engine reads instead of guessing a grid.
sheets:
	for f in assets/*.aseprite; do n=$$(basename "$$f" .aseprite); $(ASEPRITE) -b "$$f" --sheet cmd/serve/web/static/img/$$n.png --sheet-type packed --shape-padding 1 --data cmd/serve/web/static/img/$$n.json --format json-array --list-tags || exit 1; done

# The inner loop.
test:
	go test -race -shuffle=on ./...

# The lab. TinyGo and wasm-opt are the two tools check does not need; the
# README says how to install them. wasm_exec.js is copied from TinyGo because
# it must match the compiler that built the module. The optimized file is the
# one cmd/serve serves, and it is committed so a fresh clone runs. The size
# check is last, because 309 KB is the claim this engine makes.
wasm:
	cp "$$(tinygo env TINYGOROOT)/targets/wasm_exec.js" cmd/serve/web/static/js/wasm_exec.js
	mkdir -p bin
	tinygo build -target wasm -opt=z -o bin/lab.wasm ./cmd/lab
	wasm-opt -Oz --strip-debug --strip-producers -o cmd/serve/web/static/lab.wasm bin/lab.wasm
