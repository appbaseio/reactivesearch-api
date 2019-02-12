
# A custom container image for building arc
#
# Build locally:
#   docker build .circleci/images/primary
#
# Test locally:
#   docker run -it <hash> /bin/bash
#
# Tag:
#   docker build -t appbaseio/golang:<version> .circleci/images/primary
#
# Release:
#   docker push appbaseio/golang:<version>
FROM golang:1.11

RUN apt-get update

# Install tools required to add checks in config.yml
RUN go get -u github.com/golang/lint/golint
RUN curl -L https://git.io/vp6lP | sh
