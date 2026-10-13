#!/usr/bin/env just --justfile

go-mod := `go list`

# run tests
test:
    go vet ./...
    go test -race ./...

generate:
    go generate ./...

update:
    go get -u
    go mod tidy -v

publish tag:
    GOPROXY=proxy.golang.org go list -m {{go-mod}}@{{tag}}
