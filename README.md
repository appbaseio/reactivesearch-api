Arc
====

Build
-----
```
go build cmd/arc/main.go
```

```
./main --env=path/to/.env --log=stdout --plugins
```

Run
----
```
go run cmd/arc/main.go --env=path/to/.env --log=stdout --plugins
```

Quick Setup
-----------

1. Run Elasticsearch

`docker run --name es -d -p 9200:9200  docker.elastic.co/elasticsearch/elasticsearch-oss:6.4.0 bin/elasticsearch`

2. Run Arc

`go run cmd/arc/main.go --env=path/to/.env --log=stdout --plugins`

Example ENV file

```
USERNAME=admin
PASSWORD=password
ES_CLUSTER_URL=http://localhost:9200
```


Docs
----

https://arc-docs.appbase.io/
