package pipelines

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/plugins"
)

func (p *Pipelines) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	return append([]plugins.Route{
		{
			Name:        "Get pipeline schema",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipeline/schema",
			HandlerFunc: middleware(p.getPipelineSchema()),
			Description: "Returns the schema definition for pipeline request",
		},
		{
			Name:        "Create a new pipeline",
			Methods:     []string{http.MethodPost},
			Path:        "/_pipeline",
			HandlerFunc: middleware(p.postFormPipeline()),
			Description: "Creates a new pipeline with the user passed details",
		},
		{
			Name:        "Validate pipeline",
			Methods:     []string{http.MethodPost},
			Path:        "/_pipeline/validate",
			HandlerFunc: middleware(p.validateFormPipeline()),
			Description: "Validate and renders the pipeline output",
		},
		{
			Name:        "Get pipeline validate details using ID",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipeline/validate/{validateId}",
			HandlerFunc: middleware(p.getValidateLogsAndTime()),
			Description: "Get the logs and time taken for the validate ID passed",
		},
		{
			Name:        "Get all pipelines",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipelines",
			HandlerFunc: middleware(p.getPipelines()),
			Description: "Get all pipelines sorted on the basis of priority",
		},
		{
			Name:        "Get pipeline logs",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipelines/logs",
			HandlerFunc: middleware(p.getLogsForPipelines()),
			Description: "Get logs for the pipelines",
		},
		{
			Name:        "Get pipeline log for the passed log ID",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipelines/log/{id}",
			HandlerFunc: middleware(p.getLogById()),
			Description: "Get logs for the pipelines based on the passed log ID",
		},
		{
			Name:        "Get Pipeline",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipeline/{id}",
			HandlerFunc: middleware(p.getPipeline()),
			Description: "Get pipeline by using the passed ID",
		},
		{
			Name:        "Get pipeline versions for passed pipeline ID",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipeline/{id}/versions",
			HandlerFunc: middleware(p.getPipelineVersions()),
			Description: "Get all pipeline versions for the pipeline with the passed ID",
		},
		{
			Name:        "Create a new pipeline version",
			Methods:     []string{http.MethodPost},
			Path:        "/_pipeline/{id}/version",
			HandlerFunc: middleware(p.postPipelineVersion()),
			Description: "Create a new pipeline version for the passed pipeline ID",
		},
		{
			Name:        "Update pipeline version for pipeline",
			Methods:     []string{http.MethodPut},
			Path:        "/_pipeline/{id}/version/{version_id}",
			HandlerFunc: middleware(p.putPipelineVersion()),
			Description: "Update pipeline version based on the passed pipeline ID and version ID",
		},
		{
			Name:        "Set live version for pipeline",
			Methods:     []string{http.MethodPost},
			Path:        "/_pipeline/{id}/version/{version_id}/live",
			HandlerFunc: middleware(p.setLiveVersion()),
			Description: "Set the selected version as the live version for the passed pipeline",
		},
		{
			Name:        "Get pipeline logs for passed pipeline ID",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipeline/{id}/logs",
			HandlerFunc: middleware(p.getLogsForPipeline()),
			Description: "Get logs for the passed pipeline using the pipeline ID",
		},
		{
			Name:        "Get Script Ref content",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipeline/{id}/scriptRef",
			HandlerFunc: middleware(p.getScriptRef()),
			Description: "Get the script ref content by using the passed key",
		},
		{
			Name:        "Get Script Ref content for passed version",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipeline/{id}/version/{version_id}/scriptRef",
			HandlerFunc: middleware(p.getVersionScriptRef()),
			Description: "Get the script ref content by using the passed key and the version",
		},
		{
			Name:        "Update pipeline",
			Methods:     []string{http.MethodPut},
			Path:        "/_pipeline/{id}",
			HandlerFunc: middleware(p.putFormPipeline()),
			Description: "Update the pipeline body by using the passed ID",
		},
		{
			Name:        "Delete pipeline by ID",
			Methods:     []string{http.MethodDelete},
			Path:        "/_pipeline/{id}",
			HandlerFunc: middleware(p.deletePipeline()),
			Description: "Delete the pipeline using the passed ID",
		},
		{
			Name:        "Get Pipelines Usage",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/pipelines/usage",
			HandlerFunc: middleware(p.getPipelinesUsage()),
			Description: "Get pipelines usage for the current user",
		},
		{
			Name:        "Get Pipeline Stages Usage",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/pipeline/{id}/stages/usage",
			HandlerFunc: middleware(p.getPipelineStagesUsage()),
			Description: "Get pipeline stages usage for the passed pipeline ID",
		},
		{
			Name:        "Get Pipeline Version Avg Time Taken",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/pipeline/{id}/time-taken",
			HandlerFunc: middleware(p.getPipelineVersionTimeTaken()),
			Description: "Get the pipeline avg time taken based on versions using passed pipeline ID",
		},
		{
			Name:        "Get Pipeline Stage Avg Time Taken",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/pipeline/{id}/version/{version_id}/time-taken",
			HandlerFunc: middleware(p.getPipelineStageTimeTaken()),
			Description: "Get the pipeline stages avg time taken for the passed pipeline ID and the version ID",
		},
		{
			Name:        "Get Pipeline Version Error Rate",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/pipeline/{id}/error-rate",
			HandlerFunc: middleware(p.getPipelineVersionErrorRate()),
			Description: "Get the pipeline error rate based on versions using passed pipeline ID",
		},
		{
			Name:        "Get Pipeline Stage Error Rate",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/pipeline/{id}/version/{version_id}/error-rate",
			HandlerFunc: middleware(p.getPipelineStageErrorRate()),
			Description: "Get the pipeline stages error rate for the passed pipeline ID and the version ID",
		},
		{
			Name:        "Get Pipeline Version Usage",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/pipeline/{id}/versions/usage",
			HandlerFunc: middleware(p.getPipelineVersionUsage()),
			Description: "Get pipeline version usage for the passed pipeline ID",
		},
		{
			Name:        "Get Pipeline Variables",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipelines/envs",
			HandlerFunc: middleware(p.getPipelinesVars()),
			Description: "Get the pipeline environment stored for the current cluster",
		},
		{
			Name:        "Save Pipeline Variable",
			Methods:     []string{http.MethodPost},
			Path:        "/_pipelines/env",
			HandlerFunc: middleware(p.postPipelineVar()),
			Description: "Save the passed pipeline env to the current cluster",
		},
		{
			Name:        "Get Pipeline Var with Key",
			Methods:     []string{http.MethodGet},
			Path:        "/_pipelines/env/{key}",
			HandlerFunc: middleware(p.getPipelineVar()),
			Description: "Get the pipeline environment against the passed key",
		},
		{
			Name:        "Delete Pipeline Var with key",
			Methods:     []string{http.MethodDelete},
			Path:        "/_pipelines/env/{key}",
			HandlerFunc: middleware(p.deletePipelineVar()),
			Description: "Delete the pipeline environment against the passed key",
		},
		{
			Name:        "Update Pipeline Var with key",
			Methods:     []string{http.MethodPut},
			Path:        "/_pipelines/env/{key}",
			HandlerFunc: middleware(p.putPipelineVar()),
			Description: "Update the pipeline environment against the passed key",
		},
	},
		// add pipeline stage routes
		p.alternateRoutes()...,
	)
}

// alternateRoutes should return all pipeline specific routes
func (p *Pipelines) alternateRoutesLegacy() []plugins.Route {
	var pipelineRoutes = make([]plugins.Route, 0)

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

				if currentVersionPipeline.Routes != nil {
					for _, pipelineRoute := range *currentVersionPipeline.Routes {
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
							// classify category at the top
							classifyCategory := pipelineRoute.classifyRouteCategory()
							// classify acl
							classifyACL := pipelineRoute.classifyRouteACL()

							// get built-in middleware for routes
							mw := getPipelineStageMiddleware(currentVersionPipeline, pipelineRoute)
							mw = append([]middleware.Middleware{classifyCategory, classifyACL, classify.Indices(), classify.Op()}, mw...)

							// Conditionally add logsRecorder for route
							//
							// the initLogsRecorder will be a route specific middleware
							// so we cannot append it to mw
							routeMw := mw
							routeMw = append(routeMw, pipelineRoute.initLogsRecorder)

							// Build version specific route with a suffix
							routeWithSuffix := path.Join(fmt.Sprintf("/v%d", *pipelineVersion.Version), *pipelineRoute.Path)

							// For each version, the self version will be passed through the LiveVersion
							// field.
							//
							// Though LiveVersion serves a different purpose, in this case, it will be used to
							// pass the version of the pipeline from the router to the handler.
							currentVersionPipeline.LiveVersion = pipelineVersion.Version

							// Apply default/built-in middleware

							var pipelinePath = plugins.Route{
								Name:        versionName,
								Methods:     methods,
								Path:        routeWithSuffix,
								IsPipeline:  true,
								HandlerFunc: (&chain{}).WrapPipelineStageMw(currentVersionPipeline.pipelineHandler(), routeMw),
								Matcher:     currentVersionPipeline.PipelineMatcher(pipelineRoute),
							}
							pipelineRoutes = append(pipelineRoutes, pipelinePath)

							// If the current version is the live version, need to add the pipeline without
							// the suffix in route.
							if *pipelineVersion.Version == *pipeline.LiveVersion {
								pipelineRoutes = append(pipelineRoutes, plugins.Route{
									Name:        name,
									Methods:     methods,
									Path:        *pipelineRoute.Path,
									IsPipeline:  true,
									HandlerFunc: (&chain{}).WrapPipelineStageMw(currentVersionPipeline.pipelineHandler(), routeMw),
									Matcher:     currentVersionPipeline.PipelineMatcher(pipelineRoute),
								})
							}
						}
					}
				}
			}

		}
	}
	return pipelineRoutes
}

func (p *Pipelines) alternateRoutes() []plugins.Route {
	return []plugins.Route{
		p.handleItAllRoute(),
	}
}

// handleItAllRoute will be a catch it all route that will
// check if the passed route matches any of the pipeline routes
// defined.
func (p *Pipelines) handleItAllRoute() plugins.Route {
	return plugins.Route{
		Name:        "Catch it all route for Pipelines",
		Methods:     GetHttpMethods(),
		Path:        "/{rest_of_path:.+}",
		Matcher:     CatchAllMatcher(),
		HandlerFunc: CatchAllHandler(),
	}
}
