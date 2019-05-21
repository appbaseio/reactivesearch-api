FROM golang:1.11-alpine as build

# Install tools required for project
# Run `docker build --no-cache .` to update dependencies
RUN apk add --no-cache build-base
WORKDIR /arc

# List project dependencies with go.mod and go.sum
COPY go.mod .
COPY go.sum .

# Install library dependencies
RUN go mod download 

# Copy the entire project and build it
# This layer is rebuilt when a file changes in the project directory
COPY . .
RUN make

EXPOSE 8000
CMD ["build/arc", "--log", "stdout", "--env", "config/docker.env", "--plugins"]
