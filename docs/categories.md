# Category

Categories can be used to control access to data and APIs in Arc. Along with Elasticsearch APIs, Categories cover the APIs provided 
by Arc itself to allow fine-grained control over the API consumption. For Elasticsearch, Categories broadly resembles to the 
API [classification](https://www.elastic.co/guide/en/elasticsearch/reference/current/index.html) that Elasticsearch 
provides such as **Document APIs**, **Search APIs**, **Indices APIs** and so on. For Arc, Categories resembles to the 
additional APIs on top of Elasticsearch APIs, such as analytics and book keeping. A combination of Categories determine the 
APIs a user or permission can access. The list of Categories currently supported are as follows:

- `Docs`: allows access to Elasticsearch's [**Document APIs**](https://www.elastic.co/guide/en/elasticsearch/reference/current/docs.html).
- `Search`: allows access to Elasticsearch's [**Search APIs**](https://www.elastic.co/guide/en/elasticsearch/reference/current/search.html)
- `Indices`: allows access to Elasticsearch's [**Indices APIs**](https://www.elastic.co/guide/en/elasticsearch/reference/current/indices.html)
- `Cat`: allows access to Elasticsearch's [**Cat APIs**](https://www.elastic.co/guide/en/elasticsearch/reference/current/cat.html)
- `Clusters`: allows access to Elasticsearch's [**Clusters APIs**](https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster.html)
- `Misc`: allows access to Elasticsearch's APIs that includes **Scripts**, [**Ingest**](https://www.elastic.co/guide/en/elasticsearch/reference/current/ingest-apis.html), and [**Snapshot**](https://www.elastic.co/guide/en/elasticsearch/reference/current/modules-snapshots.html) APIs)
- `User`: allows access to [**User APIs**]() in Arc.
- `Permission`: allows access to [**Permission APIs**]() in Arc.
- `Analytics`: allows access to [**Analytics APIs**]() in Arc.
- `Streams`: allows access to **Streams** in Arc.
