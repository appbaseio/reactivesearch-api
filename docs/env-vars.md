# Environment Variables

Plugins might require certain environment variables to be in order initialize the components it needs for its functioning. Those variables can be declared in any file. The path to that file must be provided via the `--env` flag.

**Note:** `ES_CLUSTER_URL` is used by all the plugins that are interacting with elasticsearch. Basic auth credentials may be embedded in `ES_CLUSTER_URL` (for example `https://user:pass@host:443`). Alternatively, set `ES_API_KEY` to use API key auth for upstream elasticsearch (for example Elasticsearch Serverless); `ES_CLUSTER_URL` must not contain credentials when `ES_API_KEY` is set. `USERNAME` and `PASSWORD` are temporary entry point master credentials in order to test the plugins. 

**Meta index naming:** ReactiveSearch stores its metadata in dot-prefixed indices (e.g. `.pipelines`, `.users`). Elasticsearch Serverless does not allow creating dot-prefixed indices, so when a Serverless cluster is detected (via `build_flavor` from `GET /`), meta indices are automatically created with the `rs_` prefix instead (e.g. `rs_pipelines`). Set `RS_META_INDEX_PREFIX` to force an alternate prefix on any cluster (the prefix must not start with `_`, `-` or `+`). On Serverless, platform-managed index settings (`index.hidden`, shard/replica counts) are also stripped from index creation requests.

List of specific env vars required by respective plugins are listed below:

##### 1. Users
- `USER_ES_INDEX`

##### 2. Permissions
- `PERMISSIONS_ES_INDEX`

##### 3. Auth
- `USERS_ES_INDEX`
- `PERMISSIONS_ES_INDEX`

##### 4. Analytics
- `ANALYTICS_ES_INDEX`

##### 5. Logs
- `LOGS_ES_INDEX`
