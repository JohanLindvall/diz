BINARY := diz
GO     := go

.PHONY: all build install test race cover vet fmt fmt-check tidy check clean

## all: format, tidy, vet, build, then run the race and coverage suites (default)
all: fmt tidy vet build race cover

## build: compile all packages
build:
	$(GO) build ./...

## install: build and install the diz binary
install:
	$(GO) install .

## test: run the test suite
test:
	$(GO) test ./...

## race: run the test suite with the race detector
race:
	$(GO) test -race ./...

## cover: run tests and print total coverage
cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

## vet: run go vet
vet:
	$(GO) vet ./...

## fmt: format all sources in place
fmt:
	gofmt -w .

## fmt-check: fail if any source is not gofmt-clean
fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

## tidy: prune and update go.mod/go.sum
tidy:
	$(GO) mod tidy

## check: format check, vet and test (use in CI)
check: fmt-check vet test

## clean: remove build artifacts
clean:
	$(GO) clean
	rm -f $(BINARY) coverage.out
