BINARY := diz
GO     := go

.PHONY: all build install test race cover vet fmt fmt-check tidy tidy-check check clean

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

## tidy-check: fail if go.mod/go.sum need updating, without changing them
tidy-check:
	$(GO) mod tidy -diff

## check: check formatting and dependencies, vet, build and test (use in CI)
check: fmt-check tidy-check vet build test

## clean: remove build artifacts
clean:
	$(GO) clean
	rm -f $(BINARY) coverage.out
