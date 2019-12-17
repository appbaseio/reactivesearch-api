# Arc

Arc is a simple, modular API Gateway that sits between a client and an [ElasticSearch](https://elastic.co) cluster. It acts as a reverse proxy, routing requests from clients to services. Arc is extended through plugins, which provide extra functionality and services beyond the ElasticSearch's RESTful API. It can perform various cross-cutting tasks such as basic authentication, logging, rate-limiting, source/referers whitelisting, analytics etc. These functionalities can clearly be extended by adding a plugin encapsulating a desired  functionality. It also provides some useful abstractions that helps in managing and controlling the access
to ElasticSearch's RESTful API.

## Table of contents
- [Overview](#overview)
- [Building](#building)
- [Installation](#installation)
- [Implementation](#implementation)
- [Available Plugins](#available-plugins)
- [Use cases](#use-cases)
- [Docs](#docs)

## Overview

When Arc is deployed, every client request being made to the Elasticsearch
will hit Arc first and then be proxied to the Elasticsearch cluster. In between requests and responses, Arc
may execute the installed plugins, essentially extending the Elasticsearch API feature set. Arc effectively
becomes an entry point for every API request made to Elasticsearch. Arc can be used and deployed against any
Elasticsearch cluster (locally and hosted as provided by [Appbase.io](https://appbase.io)).

```
                             .------------------------------------------.
                             |                     |                    |
                             |                     |                    |
                             |   Authentication    |  Administration    |
                             |                     |                    |
                             |_____________________|____________________|
                             |                     |                    |
                             |                     |                    |
                             |   Security   _______|_______  Caching    |
                             |             |               |            |
.----------------.           |             |               |            |           .-----------------.
|   Dashboard/   | --------> |_____________|      Arc      |____________| --------> |  Elasticsearch  |
|   REST APIs    | <-------- |             |               |            | <-------- |    upstream     |
.----------------.           |             |               |            |           .-----------------.
                             |             |_______________|            |
                             |    Logging          |         ACLs       |
                             |                     |                    |
                             |_____________________|____________________|
                             |                     |                    |
                             |                     |                    |
                             |   Query Rules       |   Rate-Limiting    |
                             |                     |                    |
                             |                     |                    |
                             .------------------------------------------.
```

## Installation

### Running it
In order to run arc, you'll require an Elasticsearch node. There are multiple ways you can [setup an Elasticsearch](https://www.elastic.co/guide/en/elasticsearch/reference/current/setup.html), either locally or remotely. We, however, are delineating the steps for local setup of a single node Elasticsearch via it's Docker image.

**Note**: The steps described here assumes a [docker](https://docs.docker.com/install/) installation on the system.

1. Create a docker network

        docker network create arc

2. Start a single node Elasticsearch cluster locally

        docker run -d --rm --name elasticsearch -p 9200:9200 -p 9300:9300 --net=arc -e "discovery.type=single-node" docker.elastic.co/elasticsearch/elasticsearch-oss:7.2.0

3. Start Arc locally

        docker build -t arc . && docker run --rm --name arc -p 8000:8000 --net=arc --env-file=config/docker.env arc

For convenience, the steps described above are combined into a single `docker-compose` file. You can execute the file with command:

    docker-compose up

## Building

To build from source you need [Git](https://git-scm.com/downloads) and [Go](https://golang.org/doc/install) (version 1.11 or higher).

You can build the binary locally by executing the following command from the project directory:

    make

This produces an executable & plugin libraries in the root project directory. To start the Arc server, run:

    ./build/arc --env=config/manual.env

Alternatively, you could execute the following commands to start the server without producing an executable, (but still produce the plugin libraries):

    make plugins
    go run main.go --env=config/manual.env


**Note**: Running the executable assumes an active Elasticsearch connection whose url is to be provided in the `.env` file. manual.env configures it to be the localhost.

### Logging
Define the run time flag (`log`) to change the default log mode, the possible options are:

#### debug
Most verbose, use this to get logs for elasticsearch interactions.
#### info (default)
Prints the basic information
#### error
Only log the errors

#### TLS Support

You can optionally start arc to serve https requests instead of http requests using the flag https.
You also need to provide the server key & certificate file location through the environment file.
manual.env is configured to use demo server key & certificates, which work for localhost.
    go run main.go --log=stdout --env=config/manual.env --https

If you wish to manually test TLS support at localhost,
curl needs to be also passed an extra parameter providing the cacert, in this case.
    curl https://foo:bar@localhost:8000/_user --cacert sample/rootCA.pem

#### JWT Key Loading through HTTP

If you wish to test loading JWT Key through HTTP, you can use the following commands to start a HTTP
server serving the key
    cd sample
    python -m SimpleHTTPServer 8500

Then start arc using the command:
    go run main.go --log=stdout --env=config/manual-http-jwt.env

#### Run Tests

Currently, tests are WIP and implemented for auth and logs modules. You can run tests using:

    go test ./...

### Implementation

The functionality in Arc can extended via plugins. An Arc plugin can be considered as a service in itself; it can have its
own set of routes that it handles (keeping in mind it doesn't overlap with existing routes of other plugins), its own chain of
middleware and more importantly its own database it intends to interact with (in our case it is Elasticsearch). For example, one
can easily have multiple plugins providing specific services that interact with more than one database. The plugin is responsible for its own request lifecycle in this case.

However, it is not necessary for a plugin to define a set of routes for a service. A plugin can easily be a middleware
that can be used by other plugins with no new defined routes whatsoever. A middleware can either interact with a database or
not is an implementation choice, but the important point here is that a plugin can be used by other plugins as long as it
doesn't end up being a cyclic dependency.

Each plugin is structured in a particular way for brevity. Refer to the plugin [docs](https://github.com/appbaseio/arc/blob/master/docs/plugins.md) which describes a basic plugin implementation.

### Abstractions

Since every request made to Elasticsearch hits Arc first, it becomes beneficial to provide a set of abstractions that allows
the client to define control over the Elasticsearch RESTful API and Arc's functionality. Arc provides several essential abstractions that are required in order to interact with Elasticsearch and Arc itself.

## Available Plugins

### User

In order to interact with Arc, the client must define a `User`. A `User` encapsulates its own set of [properties](https://arc-api.appbase.io/) that defines its capabilities.

- `username`: uniquely identifies the user
- `password`: verifies the identity of the user
- `is_admin`: distinguishes an admin user
- `categories`: analogous to the Elasticsearch's API categories, like **Cat API**, **Search API**, **Docs API** and so on
- `acls`: adds another layer of granularity within each Elasticsearch API category
- `ops`: operations a user can perform
- `indices`: name/pattern of indices the user has access to
- `email`: user's email address
- `created_at`: time at which the user was created

### Permission

A `User` grants a `Permission` to a certain `User`, predefining its capabilities, in order to access Elasticsearch's RESTful API. Permissions serve as an entrypoint for accessing the Elasticsearch API and has a fixed *time-to-live* unlike a user, after which it will no longer be operational. A `User` is always in charge of the `Permission` it creates.

- `username`: an auto generated username that uniquely identifies the permission
- `password`: an auto generated password that verifies the identity of the permission
- `owner`: represents the owner of the permission
- `creator`: represents the creator of the permission
- `categories`: analogous to the Elasticsearch's API categories, like **Cat API**, **Search API**, **Docs API** and so on
- `acls`: adds another layer of granularity within each Elasticsearch API category
- `ops`: operations a permission can perform
- `indices`: name/pattern of indices the permission has access to
- `sources`: source IPs from which a permission is allowed to make requests
- `referers`: referers from which a permission is allowed to make requests
- `created_at`: time at which the permission was created
- `ttl`: time-to-live represents the duration till which a permission remains valid
- `limits`: request limits per `categories` given to the permission
- `description`: describes the use-case of the permission

#### Category

Categories can be used to control access to data and APIs in Arc. Along with Elasticsearch APIs, Categories cover the APIs provided by Arc itself to allow fine-grained control over the API consumption. For Elasticsearch, Categories broadly resembles to the API [classification](https://www.elastic.co/guide/en/elasticsearch/reference/current/index.html) that Elasticsearch
provides such as **Document APIs**, **Search APIs**, **Indices APIs** and so on. For Arc, Categories resembles to the
additional APIs on top of Elasticsearch APIs, such as analytics and book keeping. Refer to category [docs](https://github.com/appbaseio/arc/blob/ugo/update-readme/31-12-2018/docs/categories.md) for the list of
categories that Arc supports.

#### ACL

ACLs allow a fine grained control over the Elasticsearch APIs in addition to the Categories. Each ACL resembles an
action performed by an Elasticsearch API. For brevity, setting and organising Categories automatically sets the default
ACLs associated with the set Categories. Setting ACLs adds just another level of control to provide access to
Elasticsearch APIs within a given Category. Refer to acl [docs](https://github.com/appbaseio/arc/blob/ugo/update-readme/31-12-2018/docs/acls.md) for the list of acls that Arc supports.

#### Op

Operation delineates the kind of operation a request intends to make. The operation of the request is identified
before the request is served. The classification of the request operation depends on the use-case and the implementation
of the plugin. Operation is currently classified into three kinds:

- `Read`: operation permits read requests exclusively.
- `Write`: operation permits write requests exclusively.
- `Delete`: operation permits delete requests exclusively.

In order to allow a user or permission to make requests that involve modifying the data, a combination of the above
operations would be required. For example: `["read", "write"]` operation would allow a user or permission to perform
both read and write requests but would forbid making delete requests.

#### Request Logging

Arc currently maintains records for all the requests made to elasticsearch. Both request and responses are stored
for the users to view and inspect it later. The request logs can be fetched for both specific indices or the whole
cluster. The dedicated endpoints to fetch the index/cluster logs can be found [here](https://arc-api.appbase.io/).

## Docs

Refer to the RESTful API [docs](https://arc-api.appbase.io/) that are currently included in Arc for more information.

## Improvements

- Improve the way middleware are handled
- Propagate the es upstream errors back to the clients
