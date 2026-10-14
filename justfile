#!/usr/bin/env just --justfile

go-mod := `go list`

# run tests
test:
    go vet ./...
    go test -race ./...

# generate code
generate:
    go generate ./...

# update deps
update:
    go get -u
    go mod tidy -v

# publish
publish tag:
    GOPROXY=proxy.golang.org go list -m {{go-mod}}@{{tag}}
