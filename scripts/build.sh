#!/bin/sh
# https://golang.org/doc/install/source#environment
mkdir -p build && cd build

VERSION=0.1.0

if ! type "packr" > /dev/null; then
  go get -u github.com/gobuffalo/packr/packr
fi

export GOARCH=amd64

export GOOS=darwin
packr build -o "reactivesearch-${GOOS}-${VERSION}" ./../reactivesearch/cmd/...
zip -r "reactivesearch-${GOOS}-${VERSION}.zip" "reactivesearch-${GOOS}-${VERSION}"

export GOOS=windows
packr build -o "reactivesearch-${GOOS}-${VERSION}.exe" ./../reactivesearch/cmd/...
zip -r "reactivesearch-${GOOS}-${VERSION}.zip" "reactivesearch-${GOOS}-${VERSION}.exe"

export GOOS=linux
packr build -o "abc-${GOOS}-${VERSION}" ./../reactivesearch/cmd/...
zip -r "reactivesearch-${GOOS}-${VERSION}.zip" "reactivesearch-${GOOS}-${VERSION}"
