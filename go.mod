module github.com/appbaseio/reactivesearch-api

go 1.16

require (
	github.com/antonmedv/expr v1.9.0
	github.com/bbalet/stopwords v1.0.0
	github.com/bcicen/go-units v1.0.3
	github.com/buger/jsonparser v1.1.1
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/dgraph-io/ristretto v0.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gdexlab/go-render v1.0.1
	github.com/gertd/go-pluralize v0.1.7
	github.com/getsentry/sentry-go v0.11.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-playground/validator/v10 v10.14.1
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/go-version v1.3.0
	github.com/invopop/jsonschema v0.7.0
	github.com/keygen-sh/keygen-go v1.11.0
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/kljensen/snowball v0.6.0
	github.com/kr/pretty v0.3.1
	github.com/lithammer/fuzzysearch v1.1.3
	github.com/lithammer/shortuuid/v4 v4.0.0
	github.com/mackerelio/go-osstat v0.2.0
	github.com/natefinch/lumberjack v2.0.1-0.20190411184413-94d9e492cc53+incompatible
	github.com/olivere/elastic v6.2.21+incompatible
	github.com/olivere/elastic/v7 v7.0.24
	github.com/outcaste-io/badger/v3 v3.2202.1-0.20220409155652-232f5040a908
	github.com/pkg/profile v1.5.0
	github.com/robfig/cron v1.1.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/rs/cors v1.6.0
	github.com/sergi/go-diff v1.2.0
	github.com/sirupsen/logrus v1.9.3
	github.com/smartystreets/goconvey v1.6.4
	github.com/tiktoken-go/tokenizer v0.1.0
	github.com/ulule/limiter v2.2.2+incompatible
	go.kuoruan.net/v8go-polyfills v0.5.0
	go.mongodb.org/mongo-driver v1.12.0
	golang.org/x/crypto v0.21.0
	golang.org/x/net v0.23.0
	golang.org/x/sys v0.18.0
	golang.org/x/text v0.14.0
	gopkg.in/DataDog/dd-trace-go.v1 v1.52.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/olivere/elastic.v6 v6.2.26
	moul.io/http2curl v1.0.0
	rogchap.com/v8go v0.8.0
)

replace github.com/blugelabs/bluge => github.com/zinclabs/bluge v1.1.0

replace github.com/blugelabs/ice => github.com/zinclabs/ice v1.1.1

replace github.com/blugelabs/bluge_segment_api => github.com/zinclabs/bluge_segment_api v1.0.0

replace github.com/invopop/jsonschema => github.com/deepjyoti30/jsonschema v0.6.1-0.20221003125618-472393044c04
