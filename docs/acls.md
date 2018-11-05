# ACLs

ACLs allow a fine grained control over the Elasticsearch APIs in addition to the Categories. Each ACL resembles an
action performed by an Elasticsearch API. For brevity, setting and organising Categories automatically sets the default 
ACLs associated with the set Categories. Setting ACLs adds just another level of control to provide access to 
Elasticsearch APIs within a given Category. Each Elasticsearch Category maps to the following acls:

1. `Docs`:
	- [**Create**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-index_.html)
	- [**Get**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-get.html)
	- [**Mget**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-multi-get.html)
	- [**Update**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-update.html)
	- [**UpdateByQuery**](https://www.elastic.co/guide/en/elasticsearch/reference/master/docs-update-by-query.html)
	- [**Index**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-index_.html)
    - [**Reindex**](https://www.elastic.co/guide/en/elasticsearch/reference/master/docs-reindex.html)
	- [**Delete**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-delete.html)
	- [**DeleteByQuery**](https://www.elastic.co/guide/en/elasticsearch/reference/master/docs-delete-by-query.html)
	- [**Termvectors**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-termvectors.html)
	- [**Mtermvectors**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-multi-termvectors.html)
	- [**Bulk**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-bulk.html)
	- [**Source**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-get.html)
	- [**Exists**](http://www.elastic.co/guide/en/elasticsearch/reference/master/docs-get.html)

2. `Search`: 
	- [**Search**](http://www.elastic.co/guide/en/elasticsearch/reference/master/search-search.html)
	- [**Msearch**](http://www.elastic.co/guide/en/elasticsearch/reference/master/search-multi-search.html)
	- [**Count**](http://www.elastic.co/guide/en/elasticsearch/reference/master/search-count.html)
	- [**Explain**](http://www.elastic.co/guide/en/elasticsearch/reference/master/search-explain.html)
    - [**FieldCaps**](http://www.elastic.co/guide/en/elasticsearch/reference/master/search-field-caps.html)
	- [**Validate**](http://www.elastic.co/guide/en/elasticsearch/reference/master/search-validate.html)
	- [**RankEval**](https://www.elastic.co/guide/en/elasticsearch/reference/master/search-rank-eval.html)
	- [**Render**](http://www.elasticsearch.org/guide/en/elasticsearch/reference/master/search-template.html)
	- [**SearchShards**](http://www.elastic.co/guide/en/elasticsearch/reference/master/search-shards.html)

3. `Cat`:
    - [**Cat**](https://www.elastic.co/guide/en/elasticsearch/reference/current/cat.html)

4. `Indices`:
	- [**Indices**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-get-index.html)
	- [**Settings**](https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-update-settings.html)
	- [**Upgrade**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-upgrade.html)
	- [**Split**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-split-index.html)
	- [**Alias**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-aliases.html)
	- [**Aliases**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-aliases.html)
	- [**Stats**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-stats.html)
	- [**Template**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-templates.html)
	- [**Open**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-open-close.html)
	- [**Mapping**](https://www.elastic.co/guide/en/elasticsearch/reference/current/indices.html#mapping-management)
	- [**Recovery**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-recovery.html)
	- [**Analyze**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-analyze.html)
	- [**Cache**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-clearcache.html)
	- [**Forcemerge**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-forcemerge.html)
	- [**Refresh**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-refresh.html)
	- [**Segments**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-segments.html)
	- [**Close**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-open-close.html)
	- [**Flush**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-flush.html)
	- [**Shrink**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-shrink-index.html)
	- [**ShardStores**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-shards-stores.html)
	- [**Rollover**](http://www.elastic.co/guide/en/elasticsearch/reference/master/indices-rollover-index.html)

5. `Clusters`:
	- [**Remote**](http://www.elastic.co/guide/en/elasticsearch/reference/master/cluster-remote-info.html)
	<!--- [**Cat**](http://www.elastic.co/guide/en/elasticsearch/reference/master/tasks.html)-->
	- [**Nodes**](http://www.elastic.co/guide/en/elasticsearch/reference/master/cluster-nodes-info.html)
	- [**Tasks**](http://www.elastic.co/guide/en/elasticsearch/reference/master/tasks.html)
	- [**Cluster**](https://www.elastic.co/guide/en/elasticsearch/reference/master/cluster.html)

6. `Misc`:
	- [**Scripts**](http://www.elastic.co/guide/en/elasticsearch/reference/master/modules-scripting.html)
	<!--- [**Get**]()-->
	- [**Ingest**](https://www.elastic.co/guide/en/elasticsearch/plugins/master/ingest.html)
	- [**Snapshot**](http://www.elastic.co/guide/en/elasticsearch/reference/master/modules-snapshots.html)


