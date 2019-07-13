FROM golang:1.11-alpine as builder

# Install tools required for project
# Run `docker build --no-cache .` to update dependencies
RUN apk add --no-cache build-base git
WORKDIR /arc

# List project dependencies with go.mod and go.sum
COPY go.mod go.sum ./

# Install library dependencies
RUN go mod download 

# Copy the entire project and build it
# This layer is rebuilt when a file changes in the project directory
COPY . .
RUN make

# Final stage: Create the running container
FROM alpine:3.10.1 AS final

# Import the compiled executable from the first stage.
COPY --from=builder /arc /arc
WORKDIR /arc

EXPOSE 8000
CMD ["build/arc", "--log", "stdout", "--plugins"]
