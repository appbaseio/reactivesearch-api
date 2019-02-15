FROM golang:1.11-alpine as build

# Install tools required for project
# Run `docker build --no-cache .` to update dependencies
RUN apk add --no-cache git
RUN go get -u github.com/gobuffalo/packr/packr
WORKDIR /arc

# List project dependencies with go.mod and go.sum
COPY go.mod .
COPY go.sum .

# Install library dependencies
RUN go mod download 

# Copy the entire project and build it
# This layer is rebuilt when a file changes in the project directory
COPY . .
RUN CGO_ENABLED=0 GOOS=linux packr build -a -installsuffix cgo -o /go/bin/arc arc/cmd/main.go

## This results in a single layer image
FROM scratch
COPY --from=build /go/bin/arc arc

EXPOSE 8000
CMD ["/arc", "--log", "stdout", "--plugins"]
