FROM golang:1.11-alpine as build

# Install tools required for project
# Run `docker build --no-cache .` to update dependencies
RUN apk add --no-cache git
RUN go get github.com/golang/dep/cmd/dep

# List project dependencies with Gopkg.toml and Gopkg.lock
# These layers are only re-built when Gopkg files are updated
COPY Gopkg.lock Gopkg.toml /go/src/github.com/appbaseio-confidential/arc/
WORKDIR /go/src/github.com/appbaseio-confidential/arc/cmd/arc

# Install library dependencies
RUN dep ensure -vendor-only

# Copy the entire project and build it
# This layer is rebuilt when a file changes in the project directory
COPY . /go/src/github.com/appbaseio-confidential/arc
RUN go build -o /bin/github.com/appbaseio-confidential/arc/cmd/arc/main.go

# This results in a single layer image
FROM scratch
COPY --from=build /bin/github.com/appbaseio-confidential/arc/cmd/arc /bin/github.com/appbaseio-confidential/arc/cmd/arc
ENTRYPOINT ["/bin/github.com/appbaseio-confidential/arc/cmd/arc/main"]
CMD ["--help"]
