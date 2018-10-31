FROM golang:1.11-alpine as build

# Install tools required for project
# Run `docker build --no-cache .` to update dependencies
RUN apk add --no-cache git
WORKDIR /go/src/github.com/appbaseio-confidential/arc

# Enable the use of go modules inside GOPATH
ENV GO111MODULE=on

# List project dependencies with go.mod and go.sum
COPY go.mod .
COPY go.sum .

# Install library dependencies
RUN go mod download 

# Copy the entire project and build it
# This layer is rebuilt when a file changes in the project directory
COPY . .
RUN ls -la
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/arc cmd/arc/main.go

## This results in a single layer image
FROM scratch
COPY --from=build /go/bin/arc arc
COPY --from=build /go/src/github.com/appbaseio-confidential/arc/plugins/es/api /plugins/es/api
ENTRYPOINT ["/arc"]

EXPOSE 8000
