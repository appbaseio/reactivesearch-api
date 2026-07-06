# ReactiveSearch API

[![Tests](https://github.com/appbaseio/reactivesearch-api/actions/workflows/tests.yml/badge.svg)](https://github.com/appbaseio/reactivesearch-api/actions/workflows/tests.yml) [![Docker](https://github.com/appbaseio/reactivesearch-api/actions/workflows/docker-image.yml/badge.svg)](https://github.com/appbaseio/reactivesearch-api/actions/workflows/docker-image.yml)

ReactiveSearch API is an open-source, self-hosted search middleware for Elasticsearch and OpenSearch. It is a versatile stack for incrementally adopting AI for your site search — author composable search pipelines, build UIs, analyze performance and scale securely.

![ReactiveSearch Architecture](assets/reactivesearch-architecture.svg)

## Why ReactiveSearch API

### 1. A secure gateway to Elasticsearch and OpenSearch

ReactiveSearch sits between your application and the search cluster so clients never talk to Elasticsearch directly. API keys and users support granular permissions: restrict by **index pattern**, **API category** (Docs, Search, Indices, Cat, Clusters, Analytics, etc.), individual **ACLs**, **operations** (read / write / delete), **source IPs**, **HTTP referers**, **include/exclude fields**, per-category **rate limits** and **time-to-live** expiration. JWT-based auth with configurable RSA public keys is also supported. You get a production-ready search endpoint without handing out cluster credentials or full DSL access.

### 2. Search pipelines — fully programmable request lifecycle

Pipelines let you define the entire request/response lifecycle as a DAG of stages. Choose from 28+ pre-built stages (`reactivesearchQuery`, `elasticsearchQuery`, `useCache`, `recordAnalytics`, `kNN`, `openAIEmbeddings`, `AIAnswer`, `httpRequest` and more) or write custom **JavaScript functions** with full `async/await` and `fetch` support. Stages run in parallel, trigger conditionally and chain with `needs` dependencies — making it possible to enrich queries with external APIs or ML models, merge results and reshape responses without touching application code.

### 3. AI and vector search, built in

- **OpenAI Embeddings** stage generates vector embeddings at query time or index time and feeds them directly into kNN queries.
- **kNN** stage executes vector similarity search natively on Elasticsearch / OpenSearch.
- **AI Answer** stage sends top search results as context to GPT and returns a natural-language answer alongside traditional results — with session support for follow-up questions.
- **Knowledge Graph** integration via pipeline scripts to merge structured data from external APIs into search responses.

### 4. Composable query API — safe to expose, easy to target

Search is expressed as independent, composable query blocks wired together with the `react` property — each block maps to a facet, filter or result set without nested DSL. Because the format is declarative (no arbitrary scripts in queries), it is safe to expose to web and mobile clients. The API is also a stable, documented contract for tools, agents and UI libraries to generate against: the same query shape maps 1-to-1 to [ReactiveSearch](https://github.com/appbaseio/reactivesearch) and [Searchbox](https://github.com/appbaseio/searchbox) component props across React, Vue, React Native, Flutter and Vanilla JS.

### 5. Query rules, search relevancy and suggestions

**Query rules** let you promote, hide or inject results, replace search terms, add filters and schedule rules via cron — all configurable as data, not code. **Search Relevancy** persists per-index relevancy profiles (field weights, fuzziness, language settings) applied automatically to queries. **Suggestions** powers seven types out of the box — popular, recent, predictive, featured, FAQ, document and index — for a complete search-as-you-type experience.

### 6. Analytics, caching and observability

**Analytics** records every search, click, conversion, favorite and saved search via dedicated pipeline stages, feeding actionable insights such as slow queries, zero-result searches and geo distribution. **Caching** via the `useCache` stage serves repeat queries from a configurable in-memory cache with sub-millisecond latency. Request logging and audit trails capture what was sent to the cluster, so you can debug relevance and performance without reproducing production traffic locally.

Full API reference is available [here](https://docs.reactivesearch.io/docs/search/reactivesearch-api/reference).

## Deploy to Render

[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy?repo=https://github.com/appbaseio/reactivesearch-api)

Deploy ReactiveSearch API to [Render](https://render.com) in one click. For a fully free stack, pair it with [Aiven's free OpenSearch](https://aiven.io/free-opensearch) — host search on Aiven and the API on Render's free web service tier.

1. Create a free OpenSearch service on Aiven and copy its connection URI.
2. Click **Deploy to Render** above and set `ES_CLUSTER_URL`, `USERNAME`, and `PASSWORD` when prompted (`rs-demo` / `rs-password` are good defaults).
3. After deploy, verify the API is up:

```sh
curl https://<your-service>.onrender.com -u <username>:<password>
```

Environment variables (set at deploy time unless noted):

| Variable | Suggested | Description |
| --- | --- | --- |
| `ES_CLUSTER_URL` | — | Upstream Elasticsearch / OpenSearch URL (required) |
| `USERNAME` | `rs-demo` | Master user created on first start |
| `PASSWORD` | `rs-password` | Password for the master user |
| `RS_SETUP_PROFILE` | `minimal` | Meta indices to create at startup (pre-filled) |
| `PORT` | set by Render | Listen port (Render injects this automatically) |

## Getting Started

Get up and running in minutes. Four steps to a fully functional search stack with Elasticsearch, search pipelines, analytics and a visual dashboard.

**Prerequisites:** [Docker](https://docs.docker.com/install/) and Docker Compose installed on your machine.

### 1. Clone and start the services

Clone the Docker Compose template and start all services with a single command.

```sh
git clone https://github.com/appbaseio/reactivesearch-api-docker.git \
  && cd reactivesearch-api-docker

docker-compose -f docker-compose-with-elasticsearch.yaml up -d
```

This starts Elasticsearch, ReactiveSearch API, Nginx (with TLS) and Fluent Bit — all with a single command.

> **Using OpenSearch instead?** Replace the compose file with `docker-compose-with-opensearch.yaml`.

### 2. Verify the service is running

Once the containers are up, verify ReactiveSearch is accessible.

```sh
curl http://localhost:8000 -u rs-admin-user:rs-password
```

You should see a response like:

```json
{
  "name": "elasticsearch",
  "cluster_name": "docker-cluster",
  "version": {
    "number": "8.17.0"
  },
  "tagline": "You Know, for Search"
}
```

This confirms ReactiveSearch is running and connected to your search cluster.

### 3. Connect the Dashboard

Open [dash.reactivesearch.io](https://dash.reactivesearch.io/) in your browser. Enter your ReactiveSearch URL, username and password:

- **URL:** `http://localhost:8000`
- **Username:** `rs-admin-user`
- **Password:** `rs-password`

![ReactiveSearch Dashboard — Sign in](assets/reactivesearch-dashboard-login.png)

### 4. Start building

After signing in you'll land on the Cluster Overview — your central hub for managing indices, configuring search relevancy, building search UIs, setting up analytics and more.

![ReactiveSearch Dashboard — Cluster Overview](assets/reactivesearch-dashboard-overview.png)

### What's next?

- [Quickstart Guide](https://docs.reactivesearch.io/docs/gettingstarted/quickstart/) — Import data, preview search and build your first search UI.
- [Create a Search Pipeline](https://docs.reactivesearch.io/docs/pipelines/how-to/) — Learn how to create a search pipeline.
- [Build a Search UI](https://www.reactivesearch.io/how-to/build-search-ui) — Hands-on demos to build search UIs with ReactiveSearch.

## Building

To build from source you need [Git](https://git-scm.com/downloads) and [Go](https://golang.org/doc/install) (version 1.16 or higher).

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

#### ElasticSearch Plugin changes

If the ES plugin's static files are changed (in the `plugins/elasticsearch/api` directory) then the following command should be run in the root of the repo:

```sh
cd plugins/elasticsearch && go-bindata -o api.go -pkg static api/
```

> NOTE: Above command uses `go-bindata` package that needs to be installed in the machine and available as a binary. It is **not** required in the final production build so is not added as a dependency in the go.mod.

### Logging

Define the run time flag (`log`) to change the default log mode, the possible options are:

#### debug

Most verbose, use this to get logs for elasticsearch interactions.

#### info

Prints the basic information

#### error (default)

Only log the errors

#### profiling

Set the `profiling` flag to `true` at runtime to enable net profiling endpoints. Profiling endpoints are exposed at `/debug/pprof` route.

Profiling CPU (time taken):

An example of profiling for CPU usage is: /debug/pprof/profile (will wait for 30s and return a profile), or
You can directly hit go tool pprof -http=":8080" http://localhost:8000/debug/pprof/profile to get a profile UI (top, graph, flamegraph) for time taken.

Profiling Heap:

An example for profiling for heap usage is: /debug/pprof/heap (will return a profile for the point in time), or
You can directly hit go tool pprof -http=":8080" http://localhost:8000/debug/pprof/heap to get a profile UI (top, graph, flamegraph) of heap usage.

#### diff-logs

Set the `diff-logs` flag to `false` (defaults to `true`) at runtime to disable the use of log diffing algorithm. For high-throughput use-cases (100 requests/sec or above), the log diffing algorithm consumes signficant CPU usage (about ~2 CPU cores per 100 requests/sec) that can be saved by turning off log diffing.

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

In order to interact with ReactiveSearch, the client must define either a `User` or a permission. A `User` encapsulates its own set of [properties](https://api.reactivesearch.io/) that defines its capabilities.

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

A `User` can create a `Permission` resource and associate it with a `User`, defining its capabilities in order to access Elasticsearch's RESTful API. Permissions serve as an entry point for accessing the Elasticsearch API and has a fixed _time-to-live_ unlike a user, after which it will no longer be operational. A `User` is always in charge of the `Permission` they create.

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
cluster. The dedicated endpoints to fetch the index/cluster logs can be found [here](https://api.reactivesearch.io/).

## ReactiveSearch: UI Libraries

The ReactiveSearch API is used by [ReactiveSearch](https://github.com/appbaseio/reactivesearch) and [Searchbox](https://github.com/appbaseio/searchbox) libraries. If you're building a search UI using React, Vue, Flutter, React Native or Vanilla JS, these libraries provide scaffolding and commonly used search components that can compress weeks of development time into days.

## ReactiveSearch 🍞 and reactivesearch.io 🧈

reactivesearch.io extends the opensource ReactiveSearch API with the following functionalities:

1. **Actionable Analytics** capture telemetry from the ReactiveSearch API and provide powerful search-driven insights into users, clicks, conversions, geographical distribution, slow searches and more.
2. **Search Relevance** provides a REST API and point-and-click interface to deploy a search relevance strategy by being able to configure language, Search/Aggregation/Results settings, Query Rules.
3. **Application Cache** provides a blazing fast search performance and improved thorughput for search.
4. **UI Builder** allows creating Search and Recommendations UI widgets with no code.

You can deploy [reactivesearch.io in cloud](https://www.reactivesearch.io/). We also provide one-click installs for AWS, Heroku, Docker and Kubernetes. Get started with these over [here](https://docs.reactivesearch.io/docs/hosting/byoc/#quickstart-recipes).

## Docs

Refer to the REST API [docs](https://api.reactivesearch.io/) for ReactiveSearch.
