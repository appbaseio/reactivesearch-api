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
            "header":{
               "properties":{
                  "Accept":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Accept-Encoding":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Accept-Language":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Authorization":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cache-Control":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cdn-Loop":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cf-Connecting-Ip":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cf-Ipcountry":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cf-Ray":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cf-Request-Id":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cf-Visitor":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Connection":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Content-Length":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Content-Type":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cookie":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Origin":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Pragma":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Purpose":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Referer":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Sec-Fetch-Dest":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Sec-Fetch-Mode":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Sec-Fetch-Site":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Upgrade-Insecure-Requests":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "User-Agent":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "X-Forwarded-For":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "X-Forwarded-Proto":{
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
            "Headers":{
               "properties":{
                  "Access-Control-Allow-Credentials":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Access-Control-Allow-Origin":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cf-Cache-Status":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cf-Ray":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Cf-Request-Id":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Connection":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Content-Type":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Date":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Expect-Ct":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Server":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Set-Cookie":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Warning":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "Www-Authenticate":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "X-Content-Type-Options":{
                     "type":"text",
                     "fields":{
                        "keyword":{
                           "type":"keyword",
                           "ignore_above":256
                        }
                     }
                  },
                  "X-Origin":{
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
