# Environment Variables

Plugins might require certain environment variables to be in order initialize the components it needs for its functioning. Those variables can be declared in any file. The path to that file must be provided via the `--env` flag.

**Note:** `ES_CLUSTER_URL` is used by all the plugins that are interacting with elasticsearch. Basic auth credentials may be embedded in `ES_CLUSTER_URL` (for example `https://user:pass@host:443`). Alternatively, set `ES_API_KEY` to use API key auth for upstream elasticsearch (for example Elasticsearch Serverless); `ES_CLUSTER_URL` must not contain credentials when `ES_API_KEY` is set. `USERNAME` and `PASSWORD` are temporary entry point master credentials in order to test the plugins. 

**Meta index naming:** ReactiveSearch stores its metadata in dot-prefixed indices (e.g. `.pipelines`, `.users`). Elasticsearch Serverless does not allow creating dot-prefixed indices, so when a Serverless cluster is detected (via `build_flavor` from `GET /`), meta indices are automatically created with the `rs_` prefix instead (e.g. `rs_pipelines`). Set `RS_META_INDEX_PREFIX` to force an alternate prefix on any cluster (the prefix must not start with `_`, `-` or `+`). On Serverless, platform-managed index settings (`index.hidden`, shard/replica counts) are also stripped from index creation requests.

**Serverless rollover:** Elasticsearch Serverless rejects conditional index rollover (`max_age`, `max_docs`, `max_size`) with `rollover with conditions is not supported in serverless mode`. On Serverless clusters, ReactiveSearch evaluates those thresholds **client-side** before calling the rollover API without conditions. Rollover runs when any threshold is met (OR semantics), same as on self-managed clusters. After rollover, old backing indices beyond the latest two are deleted as usual.

**Setup profile:** Set `RS_SETUP_PROFILE` to control which meta (system) Elasticsearch indices are created at startup. All profiles use 1 primary shard per index except `full`, which keeps the current per-index defaults. Unset defaults to `full` (backward compatible).

| Profile | Primary shards (fresh install) | Indexes created |
|---------|-------------------------------|-----------------|
| `minimal` | 4 | `.users`, `.permissions`, `.pipelines`, `.pipeline_vars` |
| `standard` | 12 | minimal + `.synonyms`, `.searchrelevancy`, `.logs`, `.pipeline_logs`, `.pipeline_invocations`, `.analytics`, `.user_sessions`, `.analytics_preferences` |
| `full` | ~35–48 | All meta indexes (current behavior) |

On `minimal` and `standard`, the `.publickey` index is not created — set `JWT_RSA_PUBLIC_KEY_LOC` for JWT auth. Rollover aliases (`.logs`, `.analytics`, pipeline telemetry) may grow to two backing indices each over time.

A local test env for the minimal profile is provided as `config/minimal.env.example`. Copy it to `config/minimal.env` (gitignored), set `ES_CLUSTER_URL`, then start the server with `--env=config/minimal.env`.

| Meta index (Serverless default) | `max_age` (non-production) | `max_age` (production plan) |
|---------------------------------|----------------------------|-----------------------------|
| `rs_analytics` | 30 days | 30 days |
| `rs_logs` | 7 days | 30 days |
| `rs_pipeline_logs` | 7 days | 30 days |
| `rs_pipeline_invocations` | 3 days | 7 days |

Rollover cron jobs run at `@midnight` and `@hourly`. On Serverless, expect `serverless rollover skipped, conditions not met` when thresholds are not yet satisfied; when rollover does run, logs include `rollover res oldIndex`, `rollover res newIndex`, and `rollover res isRolledover`. On non-Serverless clusters, conditional rollover is sent in the API request and post-rollover index cleanup (keeping the latest two backing indices) is unchanged.

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
