package pipelines

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const (
	PIPELINE_ID_KEY          = "pipeline-id"
	PIPELINE_VERSION_KEY     = "pipeline-version"
	PIPELINE_ROUTE_INDEX_KEY = "pipeline-route-index"
	PIPELINE_IS_VALIDATE_KEY = "pipeline-is-validate"
)

// generateRouteName will generate the pipeline route name
// using the pipeline ID passed.
func generateRouteName(pipelineID string) string {
	return fmt.Sprintf("pipeline_%s", pipelineID)
}

// CatchAllMatcher will act as a matcher function for the
// catch-all route and determine whether or not the attached
// handler should be called.
func CatchAllMatcher() mux.MatcherFunc {
	return func(req *http.Request, rm *mux.RouteMatch) bool {
		// Determine whether the passed route matches any of the available pipeline routes.
		//
		// This determination will be done using the methods
		// provided by Mux so we will create a new dummy router for
		// verifying the route.
		dummyMuxRouter := mux.NewRouter().StrictSlash(true)

		// Use pipelines from cache
		pipelines := GetPipelinesFromCache()
		// Add routes for active pipelines
		// Iterate over all the pipelines in the cache
		for _, pipeline := range pipelines {
			// If versions are present for the given pipeline
			// then we need to check the enabled flag of the live version
			// of the pipeline.
			if pipeline.Versions != nil && len(*pipeline.Versions) > 0 && pipeline.LiveVersion != nil {
				// Get the live version from the cache
				pipelineLiveVersion, _ := GetPipelineVersion(pipeline, *pipeline.LiveVersion)
				if pipelineLiveVersion != nil {
					// Unmarshal the pipeline into a ESPipelineDoc container
					// Unmarshal the current version of the pipeline
					var currentVersionPipeline ESPipelineDoc
					json.Unmarshal([]byte(*pipelineLiveVersion.Content), &currentVersionPipeline)

					// Set the enabled flag based on this version
					pipeline.Enabled = currentVersionPipeline.Enabled
				}
			}

			if pipeline.Enabled == nil || *pipeline.Enabled {
				var name string

				// NOTE: Very unlikely that ID won't be passed
				// since it will be populated by API.
				if pipeline.ID != nil {
					name = generateRouteName(*pipeline.ID)
				} else {
					name = "Path to pipeline"
				}

				// For older pipelines, we need to make the current pipeline as the only
				// version available
				if pipeline.Versions == nil || len(*pipeline.Versions) == 0 {
					defaultPipelineVersion := make([]Version, 0)

					// Handle marshalling the pipeline
					pipelineMarshalled, _ := json.Marshal(pipeline)

					pipelineAsString := string(pipelineMarshalled)

					pipelineVersion := 1

					defaultPipelineVersion = append(defaultPipelineVersion, Version{Content: &pipelineAsString, Version: &pipelineVersion})
					pipeline.Versions = &defaultPipelineVersion

					pipeline.LiveVersion = &pipelineVersion
				}

				// Iterate over all pipeline versions and add them in the
				// router.
				for _, pipelineVersion := range *pipeline.Versions {
					versionName := fmt.Sprintf("%s-%d", name, *pipelineVersion.Version)

					// Unmarshal the current version of the pipeline
					var currentVersionPipeline ESPipelineDoc
					json.Unmarshal([]byte(*pipelineVersion.Content), &currentVersionPipeline)

					// Decode the pipeline
					currentVersionPipeline = decodePipelineScripts(currentVersionPipeline)

					isValidateStagePresent := false
					if currentVersionPipeline.Stages != nil {
						for _, stage := range *currentVersionPipeline.Stages {
							if stage.Use != nil && *stage.Use == ValidateStage {
								isValidateStagePresent = true
								break
							}
						}
					}

					if currentVersionPipeline.Routes != nil {
						for routeIndex, pipelineRoute := range *currentVersionPipeline.Routes {
							if pipelineRoute.Path != nil {
								// When pipeline is created the method is a required
								// param so if route is defined, a method will be defined
								// for it as well.
								methods := []string{}

								if pipelineRoute.Method != nil {
									// Append the user defined method
									methods = append(methods, *pipelineRoute.Method)
								} else {
									methods = []string{http.MethodGet}
								}

								// Build version specific route with a suffix
								routeWithSuffix := path.Join(fmt.Sprintf("/v%d", *pipelineVersion.Version), *pipelineRoute.Path)

								// For each version, the self version will be passed through the LiveVersion
								// field.
								//
								// Though LiveVersion serves a different purpose, in this case, it will be used to
								// pass the version of the pipeline from the router to the handler.
								currentVersionPipeline.LiveVersion = pipelineVersion.Version

								// Match the route against the passed request to see if it
								// actually matches and should be used.
								//
								// NOTE: It's important to know that the `Match` function called below will
								// also update the `vars` field of the passed routeMatch object. This means that
								// if the route of the pipeline has a dynamic field like {index} it will be extracted
								// in the following call because the current matchers route is just a catch-all
								// route.
								//
								// This is an important aspect of the execution flow because the index value is read
								// in various places during the pipelines execution.
								doesRouteMatch := dummyMuxRouter.Methods(methods...).Name(versionName).Path(routeWithSuffix).Match(req, rm)
								if doesRouteMatch {
									// Do the pipeline related matcher things
									if currentVersionPipeline.PipelineMatcher(pipelineRoute)(req, rm) {
										rm.Vars[PIPELINE_ID_KEY] = *pipeline.ID
										rm.Vars[PIPELINE_VERSION_KEY] = fmt.Sprintf("%d", *pipelineVersion.Version)
										rm.Vars[PIPELINE_ROUTE_INDEX_KEY] = fmt.Sprintf("%d", routeIndex)
										return true
									}
								}

								// If the route did not match but a validate stage is present, check if the route
								// matches with a `/validate` appended to the end of the route.
								if isValidateStagePresent {
									routeWithSuffix += "/validate"
									doesRouteMatch := dummyMuxRouter.Methods(methods...).Name(versionName).Path(routeWithSuffix).Match(req, rm)
									if doesRouteMatch {
										// Do the pipeline related matcher things
										if currentVersionPipeline.PipelineMatcher(pipelineRoute)(req, rm) {
											rm.Vars[PIPELINE_ID_KEY] = *pipeline.ID
											rm.Vars[PIPELINE_VERSION_KEY] = fmt.Sprintf("%d", *pipelineVersion.Version)
											rm.Vars[PIPELINE_ROUTE_INDEX_KEY] = fmt.Sprintf("%d", routeIndex)
											rm.Vars[PIPELINE_IS_VALIDATE_KEY] = "true"
											return true
										}
									}
								}

								// If the current version is the live version, need to add the pipeline without
								// the suffix in route.
								//
								// NOTE: As stated above, the `Match` function will update the `vars` to reflect
								// any dynamic entries present in the route.
								if *pipelineVersion.Version == *pipeline.LiveVersion {
									doesRouteMatch = dummyMuxRouter.Methods(methods...).Name(name).Path(*pipelineRoute.Path).Match(req, rm)
									if doesRouteMatch {
										// Do the pipeline related matcher things
										if currentVersionPipeline.PipelineMatcher(pipelineRoute)(req, rm) {
											rm.Vars[PIPELINE_ID_KEY] = *pipeline.ID
											rm.Vars[PIPELINE_VERSION_KEY] = fmt.Sprintf("%d", *pipeline.LiveVersion)
											rm.Vars[PIPELINE_ROUTE_INDEX_KEY] = fmt.Sprintf("%d", routeIndex)
											return true
										}
									}

									if isValidateStagePresent {
										pathWithValidate := *pipelineRoute.Path + "/validate"
										doesRouteMatch = dummyMuxRouter.Methods(methods...).Name(name).Path(pathWithValidate).Match(req, rm)
										if doesRouteMatch {
											// Do the pipeline related matcher things
											if currentVersionPipeline.PipelineMatcher(pipelineRoute)(req, rm) {
												rm.Vars[PIPELINE_ID_KEY] = *pipeline.ID
												rm.Vars[PIPELINE_VERSION_KEY] = fmt.Sprintf("%d", *pipeline.LiveVersion)
												rm.Vars[PIPELINE_ROUTE_INDEX_KEY] = fmt.Sprintf("%d", routeIndex)
												rm.Vars[PIPELINE_IS_VALIDATE_KEY] = "true"
												return true
											}
										}
									}
								}
							}
						}
					}
				}

			}
		}

		// Return false by default
		return false
	}
}

// CatchAllHandler will handle the catch-all route in
// case it matches.
//
// This handler will invoke the pipeline handler but before
// that the req properties will be updated so that the
// request actually has the pipeline route properties
func CatchAllHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Read the vars to see if they are mutated from the matcher
		vars := mux.Vars(req)

		// Read the pipeline-id and return it
		pipelineID := vars[PIPELINE_ID_KEY]

		// Fetch the pipeline using the ID
		pipelineFromID, _ := GetPipelineAndLocFromCache(pipelineID)

		// NOTE: It is very unlikely that the pipelineFromID will be nil
		// since the ID will be set in the matcher, still we will handle
		// it.
		if pipelineFromID == nil {
			errMsg := fmt.Sprintf("error while fetching pipeline with ID `%s`: not present", pipelineID)
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// For older pipelines, we need to make the current pipeline as the only
		// version available
		isVersionAbsent := false
		if pipelineFromID.Versions == nil || len(*pipelineFromID.Versions) == 0 {
			isVersionAbsent = true
			defaultPipelineVersion := make([]Version, 0)

			// Handle marshalling the pipeline
			pipelineMarshalled, _ := json.Marshal(pipelineFromID)

			pipelineAsString := string(pipelineMarshalled)

			pipelineVersion := 1

			defaultPipelineVersion = append(defaultPipelineVersion, Version{Content: &pipelineAsString, Version: &pipelineVersion})
			pipelineFromID.Versions = &defaultPipelineVersion

			pipelineFromID.LiveVersion = &pipelineVersion
		}

		pipelineVersionAsStr := vars[PIPELINE_VERSION_KEY]

		// Set default version as 1 since if version is not present,
		// this will be set above
		pipelineVersion := 1
		if !isVersionAbsent {
			var convertErr error
			pipelineVersion, convertErr = strconv.Atoi(pipelineVersionAsStr)

			// NOTE: ConvertErr is very unlikely since the integer is saved in
			// the matcher, still we will handle it.
			if convertErr != nil {
				errMsg := fmt.Sprint("error while converting pipeline version into integer after reading from vars: ", convertErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}
		}

		// Fetch the pipeline version now
		pipelineFromVersion, _ := GetPipelineVersion(*pipelineFromID, pipelineVersion)

		if pipelineFromVersion == nil {
			errMsg := fmt.Sprintf("pipeline with ID `%s` doesn't have version `%d`!", pipelineID, pipelineVersion)
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Unmarshal the current version of the pipeline
		var currentVersionPipeline ESPipelineDoc
		unmarshalErr := json.Unmarshal([]byte(*pipelineFromVersion.Content), &currentVersionPipeline)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling version of fetched pipeline: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Decode the pipeline
		currentVersionPipeline = decodePipelineScripts(currentVersionPipeline)

		// For each version, the self version will be passed through the LiveVersion
		// field.
		//
		// Though LiveVersion serves a different purpose, in this case, it will be used to
		// pass the version of the pipeline from the router to the handler.
		currentVersionPipeline.LiveVersion = pipelineFromVersion.Version

		// Fetch the pipeline Route from the pipeline
		routeAsStr := vars[PIPELINE_ROUTE_INDEX_KEY]
		routeIndex, indexConvertErr := strconv.Atoi(routeAsStr)

		// NOTE: ConvertErr is very unlikely since the integer is saved in
		// the matcher, still we will handle it.
		if indexConvertErr != nil {
			errMsg := fmt.Sprint("error while converting pipeline route index into integer after reading from vars: ", indexConvertErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		pipelineRoute := (*currentVersionPipeline.Routes)[routeIndex]

		// If the match was through a validate path, inject a boolean indicating
		// that in the context.
		isValidate := vars[PIPELINE_IS_VALIDATE_KEY]
		isValidateBool := false
		if isValidate == "true" {
			isValidateBool = true
		}

		newIsValidateCtx := PipelineIsValidateNewContext(req.Context(), &isValidateBool)
		req = req.WithContext(newIsValidateCtx)

		// classify category at the top
		classifyCategory := pipelineRoute.classifyRouteCategory()
		// classify acl
		classifyACL := pipelineRoute.classifyRouteACL()

		// get built-in middleware for routes
		mw := getPipelineStageMiddleware(currentVersionPipeline, pipelineRoute)
		mw = append([]middleware.Middleware{classifyCategory, classifyACL, classify.Indices(), classify.Op()}, mw...)
		mw = append(mw, pipelineRoute.initLogsRecorder)

		// Finally build the handler function and call it
		handlerFunc := (&chain{}).WrapPipelineStageMw(currentVersionPipeline.pipelineHandler(), mw)
		handlerFunc(w, req)
		return
	}
}
