FROM golang:1.11-alpine as builder

# Default value
# Run `--build-arg BILLING=true` to enable billing
ARG BILLING=false
ENV BILLING="${BILLING}"

# Run `--build-arg HOSTED_BILLING=true` to enable billing for hosted arc
ARG HOSTED_BILLING=false
ENV HOSTED_BILLING="${HOSTED_BILLING}"

# Run `--build-arg CLUSTER_BILLING=true` to enable billing for clusters
ARG CLUSTER_BILLING=false
ENV CLUSTER_BILLING="${CLUSTER_BILLING}"

# Run `--build-arg IGNORE_BILLING_MIDDLEWARE=true` to disable billing middleware for testing
ARG IGNORE_BILLING_MIDDLEWARE=false
ENV IGNORE_BILLING_MIDDLEWARE="${IGNORE_BILLING_MIDDLEWARE}"

# Run `--build-arg PLAN_REFRESH_INTERVAL=X` to change the default interval of 1 hour, where 'X' is an integer represent the hours unit
ARG PLAN_REFRESH_INTERVAL=1
ENV PLAN_REFRESH_INTERVAL="${PLAN_REFRESH_INTERVAL}"

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

# Get ca certs, for making api calls
RUN apk add --no-cache ca-certificates


# Create env folder
RUN mkdir /arc-data && touch /arc-data/.env && chmod 777 /arc-data/.env

# Import the compiled executable from the first stage.
COPY --from=builder /arc /arc
WORKDIR /arc

EXPOSE 8000
ENTRYPOINT ["build/arc", "--log", "stdout", "--plugins"]
CMD ["--env", "config/docker.env"]
