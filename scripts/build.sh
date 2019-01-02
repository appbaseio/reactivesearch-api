#!/bin/sh
# https://golang.org/doc/install/source#environment
mkdir -p build && cd build

VERSION=0.1.0

export GOARCH=amd64

export GOOS=darwin
go build -o "arc-${GOOS}-${VERSION}" ./../arc/cmd/...
zip -r "arc-${GOOS}-${VERSION}.zip" "arc-${GOOS}-${VERSION}"

export GOOS=windows
go build -o "arc-${GOOS}-${VERSION}.exe" ./../arc/cmd/...
zip -r "arc-${GOOS}-${VERSION}.zip" "arc-${GOOS}-${VERSION}.exe"

export GOOS=linux
go build -o "abc-${GOOS}-${VERSION}" ./../arc/cmd/...
zip -r "abc-${GOOS}-${VERSION}.zip" "abc-${GOOS}-${VERSION}"
