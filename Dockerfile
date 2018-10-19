FROM golang:1.11-alpine as build

# Install tools required for project
# Run `docker build --no-cache .` to update dependencies
RUN apk add --no-cache git
RUN go get github.com/golang/dep/cmd/dep

# List project dependencies with Gopkg.toml and Gopkg.lock
# These layers are only re-built when Gopkg files are updated
COPY Gopkg.lock Gopkg.toml /go/src/github.com/appbaseio-confidential/arc/
WORKDIR /go/src/github.com/appbaseio-confidential/arc

# Install library dependencies
RUN dep ensure -vendor-only

# Copy the entire project and build it
# This layer is rebuilt when a file changes in the project directory
COPY . /go/src/github.com/appbaseio-confidential/arc
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/arc cmd/arc/main.go

## This results in a single layer image
FROM scratch
COPY --from=build /go/bin/arc arc
COPY --from=build /go/src/github.com/appbaseio-confidential/arc/plugins/es/api /plugins/es/api
ENTRYPOINT ["/arc"]

EXPOSE 8000
