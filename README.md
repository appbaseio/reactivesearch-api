# ReactiveSearch API

ReactiveSearch API is a declarative, open-source API for querying Elasticsearch. It also acts as a reverse proxy and API gateway to an Elasticsearch cluster. ReactiveSearch API is best suited for site search, app search and e-commerce search use-cases.


## Why use ReactiveSearch API

Lets take a search query for a books e-commerce site where a user is searching for the keyword "chronicles" on either `title` or `author` fields, has a rating filter applied to only return books with a rating `gte` 4.

This query takes ~80 lines of code to write with Elasticsearch's DSL. The same query can be expressed in ~30 lines of code with ReactiveSearch.

![](https://i.imgur.com/tEx39Kq.png)

Lets understand the key differences between the two formats:

1. The Elasticsearch query is imperative in nature, makes use of search-engine specific terminologies. This makes it more expressive at the cost of a higher learning curve. In comparison, the ReactiveSearch query is declarative and hides the implementation details.

2. A ReactiveSearch query isn't prone to the nesting hell that Elasticsearch's query is. It expresses each query individually and then composes them together using the `react` property.

3. ReactiveSearch query's declarative nature also makes it composable. It is easy to capture intent, enrich the query and apply access control checks to the individual queries.

4. ReactiveSearch query's declarative nature also makes it a perfect fit for exposing it to publicly inspectable web and mobile networks. Exposing Elasticsearch's DSL in such a setting isn't recommended as it opens up a script injection attack vector.

Full API reference for ReactiveSearch is available over [here](https://docs.appbase.io/docs/search/reactivesearch-api/reference).

## Installation

### Running it

In order to run `reactivesearch-api`, you'll require an Elasticsearch node. There are multiple ways you can [setup an Elasticsearch](https://www.elastic.co/guide/en/elasticsearch/reference/current/setup.html), either locally or remotely. We, however, are delineating the steps for local setup of a single node Elasticsearch via it's Docker image.

**Note**: The steps described here assumes a [docker](https://docs.docker.com/install/) installation on the system.

1. Create a docker network

        docker network create reactivesearch

2. Start a single node Elasticsearch cluster locally

        docker run -d --rm --name elasticsearch -p 9200:9200 -p 9300:9300 --net=reactivesearch -e "discovery.type=single-node" docker.elastic.co/elasticsearch/elasticsearch-oss:7.10.2

3. Start ReactiveSearch locally

        docker build -t reactivesearch . && docker run --rm --name reactivesearch -p 8000:8000 --net=reactivesearch --env-file=config/docker.env reactivesearch

For convenience, the steps described above are combined into a single `docker-compose` file. You can execute the file with command:

    docker-compose up

## Building

To build from source you need [Git](https://git-scm.com/downloads) and [Go](https://golang.org/doc/install) (version 1.11 or higher).

You can build the binary locally by executing the following command from the project directory:

    make

This produces an executable & plugin libraries in the root project directory. To start the Reactivesearch server, run:

```bash
./build/reactivesearch --env=config/manual.env --log=info
```

Alternatively, you could execute the following commands to start the server without producing an executable, (but still produce the plugin libraries):

    make plugins
    go run main.go --env=config/manual.env


**Note**: Running the executable assumes an active Elasticsearch upstream whose URL is provided in the `.env` file.

### Logging
Define the run time flag (`log`) to change the default log mode, the possible options are:

#### debug
Most verbose, use this to get logs for elasticsearch interactions.
#### info
Prints the basic information
#### error (default)
Only log the errors

### CPU Profile
Set the `cpuprofile` flag to `true` at runtime to enable CPU profiling. Read this article to know more about the usage https://flaviocopes.com/golang-profiling/.

#### TLS Support

You can optionally start Reactivesearch to serve https requests instead of http requests using the flag https. You also need to provide the server key & certificate file location through the environment file. `config/manual.env` is configured to use demo server key & certificates, which works for localhost.

```bash
    go run main.go --log=info --env=config/manual.env --https
```

If you wish to manually test TLS support at localhost, curl needs to be also passed an extra parameter providing the cacert, in this case.

```bash
    curl https://foo:bar@localhost:8000/_user --cacert sample/rootCA.pem
```

#### JWT Key Loading through HTTP

If you wish to test loading JWT Key through HTTP, you can use the following commands to start a HTTP server serving the key
```bash
    cd sample
    python -m SimpleHTTPServer 8500
```
Then start ReactiveSearch using the command:
```bash
    go run main.go --log=info --env=config/manual-http-jwt.env
```

ReactiveSearch also exposes an API endpoint to set the key at runtime, so this need not be set initially.

#### Run Tests

Currently, tests are implemented for auth, permissions, users and billing modules. You can run tests using:

```bash
make test
```
or

```bash
go test -p 1 ./...
```

### Extending ReactiveSearch API

The functionality in ReactiveSearch can extended via plugins. A ReactiveSearch plugin can be considered as a service in itself; it can have its own set of routes that it handles (keeping in mind it doesn't overlap with existing routes of other plugins), define its own chain of middlewares and more importantly its own database it intends to interact with (in our case it is Elasticsearch). For example, one can easily have multiple plugins providing specific services that interact with more than one database. The plugin is responsible for its own request lifecycle in this case.

However, it is not necessary for a plugin to define a set of routes for a service. A plugin can easily be a middleware that can be used by other plugins with no new defined routes whatsoever. A middleware can either interact with a database or not is an implementation choice, but the important point here is that a plugin can be used by other plugins as long as it doesn't end up being a cyclic dependency.

Each plugin is structured in a particular way for brevity. Refer to the plugin [docs](https://github.com/appbaseio/reactivesearch-api/blob/master/docs/plugins.md) which describes a basic plugin implementation.

### Models

Since every request made to Elasticsearch hits ReactiveSearch server first, it becomes beneficial to provide a set of models that allow a client to define access control policies over the Elasticsearch RESTful API and ReactiveSearch's functionality. ReactiveSearch provides several essential abstractions as plugins that are required in order to interact with Elasticsearch and ReactiveSearch itself.

## Available Plugins

### User

In order to interact with ReactiveSearch, the client must define either a `User` or a permission. A `User` encapsulates its own set of [properties](https://arc-api.appbase.io/) that defines its capabilities.

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

A `User` can create a `Permission` resource and associate it with a  `User`, defining its capabilities in order to access Elasticsearch's RESTful API. Permissions serve as an entry point for accessing the Elasticsearch API and has a fixed *time-to-live* unlike a user, after which it will no longer be operational. A `User` is always in charge of the `Permission` they create.

- `username`: an auto generated username for Basic Auth access
- `password`: an auto generated password for Basic Auth access
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

Categories can be used to control access to data and APIs in ReactiveSearch. Along with Elasticsearch APIs, categories cover the APIs provided by ReactiveSearch itself to allow fine-grained control over the API consumption. For Elasticsearch, Categories broadly resembles to the API [classification](https://www.elastic.co/guide/en/elasticsearch/reference/current/index.html) that Elasticsearch
provides such as **Document APIs**, **Search APIs**, **Indices APIs** and so on. For ReactiveSearch, Categories resembles to the
additional APIs on top of Elasticsearch APIs, such as analytics and book keeping. Refer to category [docs](https://github.com/appbaseio/reactivesearch-api/blob/dev/docs/categories.md) for the list of
categories that ReactiveSearch supports.

#### ACL

ACLs allow a fine grained control over the Elasticsearch APIs in addition to the Categories. Each ACL resembles an
action performed by an Elasticsearch API. For brevity, setting and organising Categories automatically sets the default
ACLs associated with the set Categories. Setting ACLs adds just another level of control to provide access to
Elasticsearch APIs within a given Category. Refer to acl [docs](https://github.com/appbaseio/reactivesearch-api/blob/dev/docs/acls.md) for the list of acls that ReactiveSearch supports.

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

ReactiveSearch server currently maintains audit logs for all the requests made via it to elasticsearch. Both request and responses are stored
for the users to view and inspect later. The request logs can be fetched for both specific indices or the whole
cluster. The dedicated endpoints to fetch the index/cluster logs can be found [here](https://arc-api.appbase.io/).


## ReactiveSearch: UI Libraries

The ReactiveSearch API is used by [ReactiveSearch](https://github.com/appbaseio/reactivesearch-api) and [Searchbox](https://github.com/appbaseio/searchbox) libraries. If you're building a search UI using React, Vue, Flutter, React Native or Vanilla JS, these libraries provide scaffolding and commonly used search components that can compress weeks of development time into days.



## ReactiveSearch 🍞 and appbase.io 🧈

appbase.io extends the opensource ReactiveSearch API with the following functionalities:

1. **Actionable Analytics** capture telemetry from the ReactiveSearch API and provide powerful search-driven insights into users, clicks, conversions, geographical distribution, slow searches and more.
2. **Search Relevance** provides a REST API and point-and-click interface to deploy a search relevance strategy by being able to configure language, Search/Aggregation/Results settings, Query Rules.
3. **Application Cache** provides a blazing fast search performance and improved thorughput for search.
4. **UI Builder** allows creating Search and Recommendations UI widgets with no code.

You can deploy [appbase.io in cloud](https://www.appbase.io/). We also provide one-click installs for AWS, Heroku, Docker and Kubernetes. Get started with these over [here](https://docs.appbase.io/docs/hosting/byoc/#quickstart-recipes).


## Docs

Refer to the REST API [docs](https://arc-api.appbase.io/) for ReactiveSearch.
