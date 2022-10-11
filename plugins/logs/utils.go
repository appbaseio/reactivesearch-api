package logs

import log "github.com/sirupsen/logrus"

// LogsMappings mappings for .logs indices
const LogsMappings = `{
   "dynamic":false,
   "properties":{
      "@timestamp":{
         "type":"date"
      },
      "category":{
         "type":"text",
         "fields":{
            "keyword":{
               "type":"keyword",
               "ignore_above":256
            }
         }
      },
      "indices":{
         "type":"text",
         "fields":{
            "keyword":{
               "type":"keyword",
               "ignore_above":256
            }
         }
      },
      "request":{
         "properties":{
            "body":{
               "type":"text",
               "fields":{
                  "keyword":{
                     "type":"keyword",
                     "ignore_above":256
                  }
               }
            },
            "headers_string":{
               "type":"text",
               "fields":{
                  "keyword":{
                     "type":"keyword",
                     "ignore_above":256
                  }
               }
            },
            "method":{
               "type":"text",
               "fields":{
                  "keyword":{
                     "type":"keyword",
                     "ignore_above":256
                  }
               }
            },
            "uri":{
               "type":"text",
               "fields":{
                  "keyword":{
                     "type":"keyword",
                     "ignore_above":256
                  }
               }
            }
         }
      },
      "response":{
         "properties":{
            "headers_string":{
               "type":"text",
               "fields":{
                  "keyword":{
                     "type":"keyword",
                     "ignore_above":256
                  }
               }
            },
            "body":{
               "type":"text",
               "fields":{
                  "keyword":{
                     "type":"keyword",
                     "ignore_above":256
                  }
               }
            },
            "code":{
               "type":"long"
            },
            "status":{
               "type":"text",
               "fields":{
                  "keyword":{
                     "type":"keyword",
                     "ignore_above":256
                  }
               }
            },
            "took":{
               "type":"long"
            }
         }
      },
      "timestamp":{
         "type":"date"
      }
   }
}`

var blacklistedPaths = []string{
	0: "/_cluster/health",
}

// Check if the passed path is blacklisted
func isPathBlacklisted(path string) bool {
	for _, blacklistedPath := range blacklistedPaths {
		if blacklistedPath == path {
			log.Debugln(logTag, "ignoring blacklisted path:", path)
			return true
		}
	}
	return false
}
