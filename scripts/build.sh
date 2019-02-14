#!/bin/sh
# https://golang.org/doc/install/source#environment
mkdir -p build && cd build

VERSION=0.1.0

if ! type "packr" > /dev/null; then
  go get -u github.com/gobuffalo/packr/packr
fi

export GOARCH=amd64

export GOOS=darwin
packr build -o "arc-${GOOS}-${VERSION}" ./../arc/cmd/...
zip -r "arc-${GOOS}-${VERSION}.zip" "arc-${GOOS}-${VERSION}"

export GOOS=windows
packr build -o "arc-${GOOS}-${VERSION}.exe" ./../arc/cmd/...
zip -r "arc-${GOOS}-${VERSION}.zip" "arc-${GOOS}-${VERSION}.exe"

export GOOS=linux
packr build -o "abc-${GOOS}-${VERSION}" ./../arc/cmd/...
zip -r "abc-${GOOS}-${VERSION}.zip" "abc-${GOOS}-${VERSION}"
