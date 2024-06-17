package analytics

import (
	"net/http"
	"testing"

	"github.com/appbaseio-confidential/reactivesearch/util"
	. "github.com/smartystreets/goconvey/convey"
)

func testCustomEventsByPlanFail(t *testing.T, plan util.Plan) {
	build := util.BuildArc{Tier: &plan}
	util.StartArc(&build)
	build.Start()
	defer build.Close()
	Convey("402: Error", t, func() {
		Convey("Filter Labels route must throw 402 error", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_analytics/filter-labels", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 402)

		})
		Convey("Filter Values route must throw 402 error", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_analytics/filter-values/device", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 402)
		})
		Convey("Analytics routes with filters must throw 402 error", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_analytics/popular-searches?platform=mac", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 402)
		})
		// TODO: test all routes
	})
}

func testCustomEventsByPlanSuccess(t *testing.T, plan util.Plan) {
	build := util.BuildArc{Tier: &plan}
	util.StartArc(&build)
	build.Start()
	defer build.Close()
	Convey("200: OK", t, func() {
		Convey("Filter Labels route should work", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_analytics/filter-labels", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 200)

		})
		Convey("Filter Values route should work", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_analytics/filter-values/device", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 200)
		})
		Convey("Analytics routes with filters should work", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_analytics/popular-searches?platform=mac", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 200)
		})
		// TODO: test all routes
	})
}

func testCustomEventsByPlanWithFE(t *testing.T, plan util.Plan) {
	build := util.BuildArc{Tier: &plan, FeatureCustomEvents: true}
	util.StartArc(&build)
	build.Start()
	defer build.Close()
	Convey("200 OK: With FeatureException", t, func() {
		Convey("Filter Labels should work", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_analytics/filter-labels", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 200)
		})
		Convey("Filter Values should work", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_analytics/filter-values/device", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 200)
		})
		Convey("Analytics routes should work", func() {
			response, err, httpRes := util.MakeHttpRequest(http.MethodGet, "/_analytics/popular-searches?platform=mac", nil)
			if err != nil {
				t.Fatalf("test Failed %v instead\n: %v", err, response)
			}
			So(httpRes.StatusCode, ShouldEqual, 200)
		})
		// TODO: test all routes
	})
}

/* --------------------- Cluster Basic Plans ------------------------ */
func TestCustomEventsSandbox(t *testing.T) {
	testCustomEventsByPlanFail(t, util.Sandbox2019)
}

func TestCustomEventsSandboxWithFE(t *testing.T) {
	testCustomEventsByPlanWithFE(t, util.Sandbox2019)
}

func TestCustomEventsHobby(t *testing.T) {
	testCustomEventsByPlanFail(t, util.Hobby2019)
}

func TestCustomEventsHobbyWithFE(t *testing.T) {
	testCustomEventsByPlanWithFE(t, util.Hobby2019)
}

func TestCustomEventsStarter(t *testing.T) {
	testCustomEventsByPlanFail(t, util.Starter2019)
}

func TestCustomEventsStarterWithFE(t *testing.T) {
	testCustomEventsByPlanWithFE(t, util.Starter2019)
}

/* --------------------- Cluster Premium Plans ------------------------ */
func TestCustomEventsProductionI(t *testing.T) {
	testCustomEventsByPlanSuccess(t, util.ProductionFirst2019)
}
func TestCustomEventsProductionII(t *testing.T) {
	testCustomEventsByPlanSuccess(t, util.ProductionSecond2019)
}
func TestCustomEventsProductionIII(t *testing.T) {
	testCustomEventsByPlanSuccess(t, util.ProductionThird2019)
}

/* --------------------- Arc Basic Plans ------------------------ */
func TestCustomEventsBasic(t *testing.T) {
	testCustomEventsByPlanFail(t, util.ArcBasic)
}

func TestCustomEventsBasicWithFE(t *testing.T) {
	testCustomEventsByPlanWithFE(t, util.ArcBasic)
}

func TestCustomEventsStandard(t *testing.T) {
	testCustomEventsByPlanFail(t, util.ArcStandard)
}

func TestCustomEventsStandardWithFE(t *testing.T) {
	testCustomEventsByPlanWithFE(t, util.ArcStandard)
}

/* --------------------- Arc Premium Plans ------------------------ */
func TestCustomEventsEnterprise(t *testing.T) {
	testCustomEventsByPlanSuccess(t, util.ArcEnterprise)
}

/* --------------------- Byoc Basic Plans ------------------------ */
func TestCustomEventsByocBasic(t *testing.T) {
	testCustomEventsByPlanFail(t, util.HostedArcBasic)
}

func TestCustomEventsByocBasicWithFE(t *testing.T) {
	testCustomEventsByPlanWithFE(t, util.HostedArcBasic)
}

func TestCustomEventsByocStandard(t *testing.T) {
	testCustomEventsByPlanFail(t, util.HostedArcStandard)
}

func TestCustomEventsByocStandardWithFE(t *testing.T) {
	testCustomEventsByPlanWithFE(t, util.HostedArcStandard)
}

/* --------------------- Byoc Premium Plans ------------------------ */
func TestCustomEventsByocEnterprise(t *testing.T) {
	testCustomEventsByPlanSuccess(t, util.HostedArcEnterprise)
}
