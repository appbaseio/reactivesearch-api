package suggestions

import (
	"net/http"
	"testing"

	"github.com/appbaseio-confidential/reactivesearch/util"
	. "github.com/smartystreets/goconvey/convey"
)

var saveQuerySuggestionsRequest = map[string]interface{}{
	"blacklist": []interface{}{},
	"external_suggestions": []interface{}{
		map[string]interface{}{
			"count": 10000,
			"key":   "iphoneX",
			"meta": map[string]interface{}{
				"image": "https://abc.com/cat.png",
			},
		},
		map[string]interface{}{
			"count": 700,
			"key":   "samsung",
		},
	},
	"indices":        interface{}(nil),
	"min_count":      5,
	"min_hits":       2,
	"number_of_days": 60,
}

var saveQuerySuggestionsResponse = map[string]interface{}{
	"blacklist":           interface{}(nil),
	"externalSuggestions": interface{}(nil),
	"indices":             interface{}(nil),
	"minChars":            3,
	"minCount":            1,
	"minHits":             5,
	"numberOfDays":        30,
	"size":                0,
	"transformDiacritics": false,
}

func TestSuggestions(t *testing.T) {
	Convey("Testing Get Suggestions", t, func() {
		Convey("Save Query Suggestions Preferences", func() {
			_, err, _ := util.MakeHttpRequest(http.MethodPut, "/_suggestions/preferences", saveQuerySuggestionsRequest)

			if err != nil {
				t.Fatalf("SaveQuerySuggestionTest Failed %v instead\n", err)
			}
		})

		Convey("Get Query Suggestions Preferences", func() {
			response, err, _ := util.MakeHttpRequest(http.MethodGet, "/_suggestions/preferences", nil)

			parsedResponse, _ := response.(map[string]interface{})

			if err != nil {
				t.Fatalf("getQuerySuggestionTest Failed %v instead\n", err)
			}

			delete(parsedResponse, "index")
			delete(parsedResponse, "lastSyncedTime")

			mockMap := util.StructToMap(saveQuerySuggestionsResponse)

			So(parsedResponse, ShouldResemble, mockMap)
		})
	})
}

func testSuggestionsByPlanFail(t *testing.T, plan util.Plan) {
	build := util.BuildArc{Tier: &plan}
	util.StartArc(&build)
	build.Start()
	defer build.Close()
	Convey("402: ERROR", t, func() {
		Convey("Suggestions preferences route must throw 402 error", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_suggestions/preferences", nil)
			if err == nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 403)
		})
	})
}

func testSuggestionsByPlanSuccess(t *testing.T, plan util.Plan) {
	build := util.BuildArc{Tier: &plan}
	util.StartArc(&build)
	build.Start()
	defer build.Close()
	Convey("200: OK", t, func() {
		Convey("Suggestions preferences should work", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_suggestions/preferences", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 200)

		})
	})
}

func testSuggestionsByPlanWithFE(t *testing.T, plan util.Plan) {
	build := util.BuildArc{Tier: &plan, FeatureSuggestions: true}
	util.StartArc(&build)
	build.Start()
	defer build.Close()
	Convey("200 OK: With FeatureException", t, func() {
		Convey("Suggestions preferences should work", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_suggestions/preferences", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 200)
		})
	})
}

/* --------------------- Cluster Basic Plans ------------------------ */
func TestCustomEventsSandbox(t *testing.T) {
	testSuggestionsByPlanFail(t, util.Sandbox2019)
}

func TestCustomEventsSandboxWithFE(t *testing.T) {
	testSuggestionsByPlanWithFE(t, util.Sandbox2019)
}

func TestCustomEventsHobby(t *testing.T) {
	testSuggestionsByPlanFail(t, util.Hobby2019)
}

func TestCustomEventsHobbyWithFE(t *testing.T) {
	testSuggestionsByPlanWithFE(t, util.Hobby2019)
}

func TestCustomEventsStarter(t *testing.T) {
	testSuggestionsByPlanFail(t, util.Starter2019)
}

func TestCustomEventsStarterWithFE(t *testing.T) {
	testSuggestionsByPlanWithFE(t, util.Starter2019)
}

/* --------------------- Cluster Premium Plans ------------------------ */
func TestCustomEventsProductionI(t *testing.T) {
	testSuggestionsByPlanSuccess(t, util.ProductionFirst2019)
}
func TestCustomEventsProductionII(t *testing.T) {
	testSuggestionsByPlanSuccess(t, util.ProductionSecond2019)
}
func TestCustomEventsProductionIII(t *testing.T) {
	testSuggestionsByPlanSuccess(t, util.ProductionThird2019)
}

/* --------------------- Arc Basic Plans ------------------------ */
func TestCustomEventsBasic(t *testing.T) {
	testSuggestionsByPlanFail(t, util.ArcBasic)
}

func TestCustomEventsBasicWithFE(t *testing.T) {
	testSuggestionsByPlanWithFE(t, util.ArcBasic)
}

func TestCustomEventsStandard(t *testing.T) {
	testSuggestionsByPlanFail(t, util.ArcStandard)
}

func TestCustomEventsStandardWithFE(t *testing.T) {
	testSuggestionsByPlanWithFE(t, util.ArcStandard)
}

/* --------------------- Arc Premium Plans ------------------------ */
func TestCustomEventsEnterprise(t *testing.T) {
	testSuggestionsByPlanSuccess(t, util.ArcEnterprise)
}

/* --------------------- Byoc Basic Plans ------------------------ */
func TestCustomEventsByocBasic(t *testing.T) {
	testSuggestionsByPlanFail(t, util.HostedArcBasic)
}

func TestCustomEventsByocBasicWithFE(t *testing.T) {
	testSuggestionsByPlanWithFE(t, util.HostedArcBasic)
}

func TestCustomEventsByocStandard(t *testing.T) {
	testSuggestionsByPlanFail(t, util.HostedArcStandard)
}

func TestCustomEventsByocStandardWithFE(t *testing.T) {
	testSuggestionsByPlanWithFE(t, util.HostedArcStandard)
}

/* --------------------- Byoc Premium Plans ------------------------ */
func TestCustomEventsByocEnterprise(t *testing.T) {
	testSuggestionsByPlanSuccess(t, util.HostedArcEnterprise)
}
