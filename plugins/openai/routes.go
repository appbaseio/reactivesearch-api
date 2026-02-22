package openai

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins"
)

func (o OpenAI) routes() []plugins.Route {
	routes := make([]plugins.Route, 0)
	middlewareFunction := (&chain{}).Wrap
	routes = append(routes, plugins.Route{
		Name:        "Save OpenAI Preferences",
		Methods:     []string{http.MethodPost},
		Path:        "/_ai/preferences",
		HandlerFunc: middlewareFunction(o.postOpenAIConfig()),
		Description: "Save the passed OpenAI preferences for later use",
	})
	routes = append(routes, plugins.Route{
		Name:        "Get OpenAI Preferences",
		Methods:     []string{http.MethodGet},
		Path:        "/_ai/preferences",
		HandlerFunc: middlewareFunction(o.getOpenAIConfig()),
		Description: "Get the OpenAI preferences saved for the cluster",
	})
	routes = append(routes, plugins.Route{
		Name:        "Get list of FAQss",
		Methods:     []string{http.MethodGet},
		Path:        "/_ai/faqs",
		HandlerFunc: middlewareFunction(o.getFAQs()),
		Description: "Get a list of FAQ's with support for size and from query params",
	})
	routes = append(routes, plugins.Route{
		Name:        "Get list of FAQs filtered by a searchboxId",
		Methods:     []string{http.MethodGet},
		Path:        "/_ai/faqs/searchbox/{searchboxId}",
		HandlerFunc: middlewareFunction(o.getFAQsBySearchBox()),
		Description: "Get a list of FAQs filtered by a searchboxId with support for size and from query params",
	})
	routes = append(routes, plugins.Route{
		Name:        "Create/Update FAQ body",
		Methods:     []string{http.MethodPut},
		Path:        "/_ai/faq/{id}",
		HandlerFunc: middlewareFunction(o.createOrUpdateFAQ()),
		Description: "Create the passed FAQ and update it if it already exists",
	})
	routes = append(routes, plugins.Route{
		Name:        "Patch FAQ",
		Methods:     []string{http.MethodPatch},
		Path:        "/_ai/faq/{id}",
		HandlerFunc: middlewareFunction(o.patchFAQ()),
		Description: "Patch update an FAQ with the passed ID",
	})
	routes = append(routes, plugins.Route{
		Name:        "Delete FAQ",
		Methods:     []string{http.MethodDelete},
		Path:        "/_ai/faq/{id}",
		HandlerFunc: middlewareFunction(o.deleteFAQ()),
		Description: "Delete the FAQ by using the passed ID",
	})
	routes = append(routes, plugins.Route{
		Name:        "Get FAQ body",
		Methods:     []string{http.MethodGet},
		Path:        "/_ai/faq/{id}",
		HandlerFunc: middlewareFunction(o.getFAQById()),
		Description: "Get the FAQ by using the passed ID",
	})
	routes = append(routes, plugins.Route{
		Name:        "Get OpenAI Session Analytics",
		Methods:     []string{http.MethodGet},
		Path:        "/_ai/analytics",
		HandlerFunc: middlewareFunction(o.getSessionAnalytics()),
		Description: "Get the OpenAI session analytics for the passed duration",
	})
	routes = append(routes, plugins.Route{
		Name:        "Fetch OpenAI Session Analytics Docs",
		Methods:     []string{http.MethodGet},
		Path:        "/_ai/analytics/filter",
		HandlerFunc: middlewareFunction(o.getSessionAnalyticsByFilter()),
		Description: "Get the OpenAI session analytics docs based on the filter",
	})
	routes = append(routes, plugins.Route{
		Name:        "Create a new session based on the passed context",
		Methods:     []string{http.MethodPost},
		Path:        "/_ai",
		HandlerFunc: middlewareFunction(o.createSession()),
		Description: "Route to initiate a new session based on the passed context",
	})
	routes = append(routes, plugins.Route{
		Name:        "Fetch the AIAnswer response",
		Methods:     []string{http.MethodGet},
		Path:        "/_ai/{AISessionId}",
		HandlerFunc: middlewareFunction(o.fetchAIAnswer()),
		Description: "Route to fetch the AIAnswer response based on the passed sessionId",
	})
	routes = append(routes, plugins.Route{
		Name:        "Ask a follow-up question for AI Answer",
		Methods:     []string{http.MethodPost},
		Path:        "/_ai/{AISessionId}",
		HandlerFunc: middlewareFunction(o.postFollowUpQuestion()),
		Description: "Route to ask a follow-up question for a given sessionId",
	})
	routes = append(routes, plugins.Route{
		Name:        "Ask a follow-up question for AI Answer with SSE response",
		Methods:     []string{http.MethodPost},
		Path:        "/_ai/{AISessionId}/sse",
		HandlerFunc: middlewareFunction(o.postFollowUpQuestionSSE()),
		Description: "Route to ask a follow-up question for a given sessionId with SSE support",
	})
	routes = append(routes, plugins.Route{
		Name:        "Get response with SSE",
		Methods:     []string{http.MethodGet},
		Path:        "/_ai/{AISessionId}/sse",
		HandlerFunc: middlewareFunction(o.getResponseWithSSE()),
		Description: "Route to get response for a given sessionId with SSE support",
	})
	routes = append(routes, plugins.Route{
		Name:        "Save useful/not useful analytics regarding the session",
		Methods:     []string{http.MethodPut},
		Path:        "/_ai/{AISessionId}/analytics",
		HandlerFunc: middlewareFunction(o.putSessionAnalytics()),
		Description: "Route to set whether a session was useful/not useful",
	})
	routes = append(routes, plugins.Route{
		Name:        "Return session's analytics details",
		Methods:     []string{http.MethodGet},
		Path:        "/_ai/{AISessionId}/detail",
		HandlerFunc: middlewareFunction(o.getSessionDetails()),
		Description: "Route to get the analytics details for the passed session",
	})
	return routes
}
