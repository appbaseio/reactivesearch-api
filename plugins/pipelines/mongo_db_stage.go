package pipelines

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBInput struct {
	Credentials       *string                   `json:"credentials,omitempty" jsonschema:"title=Credentials,required" jsonschema_description:"Auth credentials. For example, 'user@pass'."`
	HOST              *string                   `json:"host,omitempty" jsonschema:"title=Host,required" jsonschema_description:"Host name and port of instance(s). For example, 'sample.host:27017'."`
	Protocol          *string                   `json:"protocol,omitempty" jsonschema:"title=Protocol" jsonschema_description:"Protocol. For example, 'mongodb+srv'."`
	DB                *string                   `json:"db,omitempty" jsonschema:"title=Database Name,required" jsonschema_description:"Database name, e.g. 'sample_airbnb'"`
	Collection        *string                   `json:"collection,omitempty" jsonschema:"title=Collection,required" jsonschema_description:"Collection name, e.g. 'listingsAndReviews'"`
	ConnectionOptions string                    `json:"connectionOptions,omitempty" jsonschema:"title=Connection Options" jsonschema_description:"Connection options, e.g 'maxPoolSize=20&w=majority'"`
	Body              *[]map[string]interface{} `json:"body,omitempty" jsonschema:"title=Request Body" jsonschema_description:"MongoDB custom aggregations query, for e.g, [ { '$facet': { 'hits': [ { '$limit': 10 } ], 'total': [ { '$count': 'count' } ] } } ]"`
}

func GetMongoDBQueryInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&MongoDBInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

func toDoc(v interface{}) (doc *bson.D, err error) {
	data, err := bson.Marshal(v)
	if err != nil {
		return
	}
	err = bson.Unmarshal(data, &doc)
	return
}

func executeMongoDBStage(
	stage ESPipelineStage,
	parsedInputs *string,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	timeTook := time.Now()
	id := getStageID(stage)
	scriptContextInBytes := globalScriptContext.Get()
	var scriptContext rules.ScriptContext
	err2 := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}
	// validate inputs
	if parsedInputs == nil {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("Inputs are missing for stage id: " + *id),
			Code: http.StatusBadRequest,
		}
	}
	var inputs MongoDBInput
	err := json.Unmarshal([]byte(*parsedInputs), &inputs)
	if err != nil {
		return scriptContextInBytes, false, &Error{
			Err:  err,
			Code: http.StatusBadRequest,
		}
	}

	if inputs.HOST == nil || *inputs.HOST == "" {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("The 'host' input must be present for stage: " + *id),
			Code: http.StatusBadRequest,
		}
	}

	if inputs.DB == nil || *inputs.DB == "" {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("The 'db' input must be present for stage: " + *id),
			Code: http.StatusBadRequest,
		}
	}

	if inputs.Collection == nil || *inputs.Collection == "" {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("The 'collection' input must be present for stage: " + *id),
			Code: http.StatusBadRequest,
		}
	}

	if inputs.Credentials == nil || *inputs.Credentials == "" {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("The 'credentials' input must be present for stage: " + *id),
			Code: http.StatusBadRequest,
		}
	}

	if inputs.Protocol == nil || *inputs.Protocol == "" {
		defaultProtocol := "mongodb+srv"
		inputs.Protocol = &defaultProtocol
	}

	// ----------- connect mongodb client --------------
	uri := *inputs.Protocol + "://" + *inputs.Credentials + "@" + *inputs.HOST + "/?" + inputs.ConnectionOptions
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Errorln(logTag, ":", err)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("Error encountered while connecting the mongodb client: " + err.Error()),
			Code: http.StatusBadRequest,
		}
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Errorln("Error encountered while disconnecting the mongodb client")
		}
	}()
	// ----------- connect mongodb client --------------

	// ----------- perform search ----------------------
	collection := client.Database(*inputs.DB).Collection(*inputs.Collection)

	bodyPassed := ""

	if inputs.Body != nil {
		var pipeline mongo.Pipeline
		for _, pipelineQuery := range *inputs.Body {
			pipelineBSON, err := toDoc(pipelineQuery)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return scriptContextInBytes, false, &Error{
					Err:  err,
					Code: http.StatusBadRequest,
				}
			}
			pipeline = append(pipeline, *pipelineBSON)
		}
		if len(pipeline) > 0 {
			opts := options.Aggregate()
			cursor, err := collection.Aggregate(
				context.TODO(),
				pipeline,
				opts)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return scriptContextInBytes, false, &Error{
					Err:  err,
					Code: http.StatusBadRequest,
				}
			}

			pipelineMarshalled, marshalErr := json.Marshal(pipeline)
			if marshalErr != nil {
				log.Warnln(logTag, ": error while marshalling pipeline: ", marshalErr.Error())
			} else {
				bodyPassed = string(pipelineMarshalled)
			}

			var resultsBSON []bson.D
			if err = cursor.All(context.TODO(), &resultsBSON); err != nil {
				log.Errorln(logTag, ":", err)
				return scriptContextInBytes, false, &Error{
					Err:  err,
					Code: http.StatusBadRequest,
				}
			}
			var results []interface{}
			for _, v := range resultsBSON {
				resultsInBytes, err := bson.MarshalExtJSON(v, false, false)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return scriptContextInBytes, false, &Error{
						Err:  err,
						Code: http.StatusBadRequest,
					}
				}
				var result interface{}
				err2 := json.Unmarshal(resultsInBytes, &result)
				if err2 != nil {
					log.Errorln(logTag, ":", err2)
					return scriptContextInBytes, false, &Error{
						Err:  err2,
						Code: http.StatusBadRequest,
					}
				}
				results = append(results, result)
			}
			resultsInBytes, err := json.Marshal(results)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return scriptContextInBytes, false, &Error{
					Err:  err,
					Code: http.StatusBadRequest,
				}
			}
			// write to response
			resultsOutput := resultsInBytes
			var output interface{}
			if async {
				// write output to a top-level variable
				output = map[string]interface{}{
					*id: string(resultsOutput),
				}
			} else {
				// set response code
				// TODO: Return status code in response (realm-function)
				scriptContext.Response.Code = http.StatusOK
				scriptContext.Response.Body = string(resultsOutput)
				scriptContext.Response.Headers["X-Origin"] = "reactivesearch.io"
				output = scriptContext
			}
			responseInBytes, err := json.Marshal(output)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, false, &Error{
					Err: err,
				}
			}
			return responseInBytes, false, nil
		}
		return scriptContextInBytes, false, nil
	} else {
		// Get query from request body
		var requestBody map[string][]map[string]interface{}
		err4 := json.Unmarshal([]byte(scriptContext.Request.Body), &requestBody)
		if err4 != nil {
			log.Errorln(logTag, ":", err4)
			return scriptContextInBytes, false, &Error{
				Err:  err4,
				Code: http.StatusBadRequest,
			}
		}

		// RS API request body from context
		rsAPIRequestBody := rsAPIRequest.Get()

		var wg sync.WaitGroup
		out := make(chan map[string]interface{})

		for queryId, queryObject := range requestBody {
			// prepare mongo pipeline
			var pipeline mongo.Pipeline
			var defaultQuery *bson.D
			for _, pipelineQuery := range queryObject {
				pipelineBSON, err := toDoc(pipelineQuery)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return scriptContextInBytes, false, &Error{
						Err:  err,
						Code: http.StatusBadRequest,
					}
				}
				pipeline = append(pipeline, *pipelineBSON)
			}

			var rsQuery map[string]interface{}
			for _, v := range rsAPIRequestBody.Query {
				if *v.ID == queryId {
					// Set default query
					if v.DefaultQuery != nil {
						defaultQueryBSON, err := toDoc(*v.DefaultQuery)
						if err != nil {
							log.Errorln(logTag, ":", err)
							return scriptContextInBytes, false, &Error{
								Err:  err,
								Code: http.StatusBadRequest,
							}
						}
						defaultQuery = defaultQueryBSON
					}
					queryBytes, _ := json.Marshal(v)
					err := json.Unmarshal(queryBytes, &rsQuery)
					if err != nil {
						log.Errorln(logTag, ":", err)
						return scriptContextInBytes, false, &Error{
							Err:  err,
							Code: http.StatusBadRequest,
						}
					}
				}
			}

			if len(pipeline) > 0 {
				wg.Add(1)
				go func(out chan<- map[string]interface{}, queryId string, rsQuery map[string]interface{}) {
					start := time.Now()
					defer wg.Done()
					opts := options.Aggregate()
					cursor, err := collection.Aggregate(
						context.TODO(),
						pipeline,
						opts)
					if err != nil {
						log.Errorln(logTag, ":", err)
						out <- map[string]interface{}{
							"error":   err,
							"rsQuery": rsQuery,
							"took":    time.Since(start).Milliseconds(),
						}
						return
					}
					var resultsBSON []bson.D
					if err = cursor.All(context.TODO(), &resultsBSON); err != nil {
						log.Errorln(logTag, ":", err)
						out <- map[string]interface{}{
							"error":   err,
							"rsQuery": rsQuery,
							"took":    time.Since(start).Milliseconds(),
						}
						return
					}
					var results []interface{}
					for _, v := range resultsBSON {
						resultsInBytes, err := bson.MarshalExtJSON(v, false, false)
						if err != nil {
							log.Errorln(logTag, ":", err)
							out <- map[string]interface{}{
								"error":   err,
								"rsQuery": rsQuery,
								"took":    time.Since(start).Milliseconds(),
							}
							return
						}
						var result interface{}
						err2 := json.Unmarshal(resultsInBytes, &result)
						if err2 != nil {
							log.Errorln(logTag, ":", err2)
							out <- map[string]interface{}{
								"error":   err2,
								"rsQuery": rsQuery,
								"took":    time.Since(start).Milliseconds(),
							}
							return
						}
						results = append(results, result)
					}

					// fetch default query and write raw results
					var defaultQueryResults []interface{}
					if defaultQuery != nil {
						defaultQueryCursor, err := collection.Aggregate(
							context.TODO(),
							mongo.Pipeline{*defaultQuery},
							opts)
						if err != nil {
							log.Errorln(logTag, ":", err)
							out <- map[string]interface{}{
								"error":    err,
								"rsQuery":  rsQuery,
								"took":     time.Since(start).Milliseconds(),
								"response": results,
							}
							return
						}
						var defaultQueryResultsBSON []bson.D
						if err = defaultQueryCursor.All(context.TODO(), &defaultQueryResultsBSON); err != nil {
							log.Errorln(logTag, ":", err)
							out <- map[string]interface{}{
								"error":    err,
								"rsQuery":  rsQuery,
								"took":     time.Since(start).Milliseconds(),
								"response": results,
							}
							return
						}
						for _, v := range resultsBSON {
							defaultQueryResultsInBytes, err := bson.MarshalExtJSON(v, false, false)
							if err != nil {
								log.Errorln(logTag, ":", err)
								out <- map[string]interface{}{
									"error":   err,
									"rsQuery": rsQuery,
									"took":    time.Since(start).Milliseconds(),
								}
								return
							}
							var defaultQueryResult interface{}
							err2 := json.Unmarshal(defaultQueryResultsInBytes, &defaultQueryResult)
							if err2 != nil {
								log.Errorln(logTag, ":", err2)
								out <- map[string]interface{}{
									"error":   err2,
									"rsQuery": rsQuery,
									"took":    time.Since(start).Milliseconds(),
								}
								return
							}
							defaultQueryResults = append(defaultQueryResults, defaultQueryResult)
						}
					}

					out <- map[string]interface{}{
						"response": results,
						"raw":      defaultQueryResults,
						"error":    nil,
						"rsQuery":  rsQuery,
						"took":     time.Since(start).Milliseconds(),
					}
				}(out, queryId, rsQuery)
			}
		}

		// wait for the queries to be resolved
		go func() {
			wg.Wait()
			close(out)
		}()

		// parse response to RS API response
		timeTaken := time.Since(timeTook).Milliseconds()
		var responses []interface{}

		for result := range out {
			// Handle errors if error key is present and is not nil or not
			// an empty string
			errorValue, isPresent := result["error"]
			if isPresent && errorValue != nil && errorValue != "" {
				return scriptContextInBytes, false, &Error{
					Err:  fmt.Errorf("error while executing mongo query: %s", errorValue),
					Code: http.StatusInternalServerError,
				}
			}
			responses = append(responses, result)
		}

		contextInBytes, err := json.Marshal(map[string]interface{}{
			"timeTaken": timeTaken,
			"responses": responses,
		})
		if err != nil {
			log.Errorln(logTag, ":", err)
			return scriptContextInBytes, false, &Error{
				Err: err,
			}
		}
		// translate mongodb query response to RS API response
		script := `function handleRequest() {
		const a = new reactivesearch.ReactiveSearch({})
		return a.transformResponse(context.timeTaken, context.responses);
	}`
		scriptOutput, _, err := rules.RunScript(contextInBytes, script, 5*time.Second)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return scriptContextInBytes, false, &Error{
				Err: err,
			}
		}
		// write to response
		var output interface{}
		if async {
			// write output to a top-level variable
			output = map[string]interface{}{
				*id: string(scriptOutput),
			}
		} else {
			// set response code
			// TODO: Return status code in response (realm-function)
			scriptContext.Response.Code = http.StatusOK
			scriptContext.Response.Body = string(scriptOutput)
			scriptContext.Response.Headers["X-Origin"] = "reactivesearch.io"

			// Set request values
			scriptContext.Request.Body = bodyPassed
			scriptContext.Request.URL = uri
			scriptContext.Request.Headers = make(map[string]string)
			scriptContext.Request.Method = http.MethodPost

			output = scriptContext
		}
		responseInBytes, err := json.Marshal(output)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, false, &Error{
				Err: err,
			}
		}
		return responseInBytes, false, nil
	}
}
