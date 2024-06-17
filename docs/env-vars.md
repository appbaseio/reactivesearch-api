# Environment Variables

Plugins might require certain environment variables to be in order initialize the components it needs for its functioning. Those variables can be declared in any file. The path to that file must be provided via the `--env` flag.

**Note:** `ES_CLUSTER_URL` is used by all the plugins that are interacting with elasticsearch. `USERNAME` and `PASSWORD` are temporary entry point master credentials in order to test the plugins. 

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
