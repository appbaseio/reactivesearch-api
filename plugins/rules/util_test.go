package rules

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"go.kuoruan.net/v8go-polyfills/base64"
	"rogchap.com/v8go"
)

func TestValidateExpression(t *testing.T) {
	Convey("Basic expression: query", t, func() {
		err := validateTriggerExpression("$query contains 'abc'")
		So(err, ShouldBeNil)
	})
	Convey("Basic expression: index", t, func() {
		err := validateTriggerExpression("'logs' in $index")
		So(err, ShouldBeNil)
	})
	Convey("Basic expression: filter", t, func() {
		err := validateTriggerExpression("$filter.year matches 'abc'")
		So(err, ShouldBeNil)
	})
	Convey("Basic expression: type", t, func() {
		err := validateTriggerExpression(`$type in ["search", "term"]`)
		So(err, ShouldBeNil)
	})
	Convey("Invalid expression: query", t, func() {
		err := validateTriggerExpression("$query contain 'abc'")
		So(err, ShouldBeError)
	})
	Convey("Invalid expression: index", t, func() {
		err := validateTriggerExpression("'logs' in $inde")
		So(err, ShouldBeError)
	})
	Convey("Invalid expression: filter", t, func() {
		err := validateTriggerExpression("$filter.year matces 'abc'")
		So(err, ShouldBeError)
	})
	Convey("Invalid Advanced expression: filter", t, func() {
		err := validateTriggerExpression("('logs' in $index) (AND) ($filter.year matches 'abc')")
		So(err, ShouldBeError)
	})
	Convey("Valid Advanced expression: filter", t, func() {
		err := validateTriggerExpression("('logs' in $index) and ($filter.year matches 'abc')")
		So(err, ShouldBeNil)
	})
	Convey("Valid expression: cron", t, func() {
		err := validateCronTriggerExpression("0 0 12 * * *")
		So(err, ShouldBeNil)
	})
	Convey("Invalid expression: cron", t, func() {
		err := validateCronTriggerExpression("0 0 12 * * nana")
		So(err, ShouldBeError)
	})
	Convey("Valid expression: cron", t, func() {
		err := validateCronTriggerExpression("0 * * * * *")
		So(err, ShouldBeNil)
	})
	Convey("Valid expression: cron", t, func() {
		err := validateCronTriggerExpression("@every 1m")
		So(err, ShouldBeNil)
	})
}

func TestParsedExpression(t *testing.T) {
	Convey("Basic expression: query", t, func() {
		expression := parseTriggerExpression("$query contains 'abc'", Filter)
		So(expression, ShouldEqual, `Query contains 'abc'`)
	})
	Convey("Basic expression with regexp: query", t, func() {
		expression := parseTriggerExpression("$query matches '*.'", Filter)
		So(expression, ShouldEqual, `Query matches '*.'`)
	})
	Convey("Basic expression: filter with dot notation", t, func() {
		expression := parseTriggerExpression("$filter.user.name.keyword matches 'John'", Filter)
		So(expression, ShouldEqual, `Filter.user__name__keyword matches 'John'`)
	})
	Convey("Basic expression: filter with brackets", t, func() {
		expression := parseTriggerExpression(`$filter["user.name.keyword"] matches 'John'`, Filter)
		So(expression, ShouldEqual, `Filter["user__name__keyword"] matches 'John'`)
	})
	Convey("Advanced expression: filter with dot and query as regexp", t, func() {
		expression := parseTriggerExpression(`($filter.user.name.keyword matches 'John') and ($query matches '.*')`, Filter)
		So(expression, ShouldEqual, `(Filter.user__name__keyword matches 'John') and (Query matches '.*')`)
	})
	Convey("Advanced expression: filter as brackets and query as regexp", t, func() {
		expression := parseTriggerExpression(`($filter["user.name.keyword"] matches 'John') and ($query matches '.*')`, Filter)
		So(expression, ShouldEqual, `(Filter["user__name__keyword"] matches 'John') and (Query matches '.*')`)
	})
	Convey("Advanced expression: space separated field", t, func() {
		expression := parseTriggerExpression(`($filter[' Full Name (Transfer to First/Last).keyword'] matches 'B Blankenheim')`, Filter)
		So(expression, ShouldEqual, `(Filter[' Full Name (Transfer to First/Last)__keyword'] matches 'B Blankenheim')`)
	})
	Convey("Advanced expression: space separated field with comparison operator (==) ", t, func() {
		expression := parseTriggerExpression(`($filter[' Full Name (Transfer to First/Last).keyword'] == 'B Blankenheim')`, Filter)
		So(expression, ShouldEqual, `(Filter[' Full Name (Transfer to First/Last)__keyword'] == 'B Blankenheim')`)
	})
	Convey("Basic expression (single quoted): filter with brackets", t, func() {
		expression := parseTriggerExpression(`$filter['user.name.keyword'] matches 'John'`, Filter)
		So(expression, ShouldEqual, `Filter['user__name__keyword'] matches 'John'`)
	})
	Convey("Advanced expression (single quoted): filter as brackets and query as regexp", t, func() {
		expression := parseTriggerExpression(`($filter['user.name.keyword'] matches 'John') and ($query matches '.*')`, Filter)
		So(expression, ShouldEqual, `(Filter['user__name__keyword'] matches 'John') and (Query matches '.*')`)
	})
	Convey("Test Category from envs", t, func() {
		expression := parseTriggerExpression(`$category matches 'docs'`, Index)
		So(expression, ShouldEqual, `Category matches 'docs'`)
	})
	Convey("Test ACL from envs", t, func() {
		expression := parseTriggerExpression(`$acl matches 'bulk'`, Index)
		So(expression, ShouldEqual, `ACL matches 'bulk'`)
	})
	Convey("Test ACL and Category with and in between", t, func() {
		expression := parseTriggerExpression(`($category matches 'docs') and ($acl matches 'update')`, Index)
		So(expression, ShouldEqual, `(Category matches 'docs') and (ACL matches 'update')`)
	})
}

func TestGetDefaultOrder(t *testing.T) {
	Convey("Basic: without any rules", t, func() {
		So(getDefaultRuleOrder(make([]ESRuleDoc, 0)), ShouldEqual, 1)
	})
	Convey("With a rule with order value as len + 1(similar)", t, func() {
		order1 := 1
		order2 := 3
		rules := []ESRuleDoc{
			{
				Order: &order1,
			},
			{
				Order: &order2,
			},
		}
		So(getDefaultRuleOrder(rules), ShouldEqual, 4)
	})
	Convey("With a rule with order value as > len + 1", t, func() {
		order1 := 1
		order2 := 5
		rules := []ESRuleDoc{
			{
				Order: &order1,
			},
			{
				Order: &order2,
			},
		}
		So(getDefaultRuleOrder(rules), ShouldEqual, 6)
	})
}

func TestExtractRegexpFromTriggerExp(t *testing.T) {
	Convey("No match: Single Quoted", t, func() {
		expression := getRegexpFromTriggerExp("$query contains 'abc'")
		So(expression, ShouldEqual, "")
	})
	Convey("One match: Single Quoted", t, func() {
		expression := getRegexpFromTriggerExp("$query matches 'abc'")
		So(expression, ShouldEqual, "abc")
	})
	// First one should be picked
	Convey("Multiple matches: Single Quoted", t, func() {
		expression := getRegexpFromTriggerExp("$query matches 'abc' and $query matches 'def'")
		So(expression, ShouldEqual, "abc")
	})
	Convey("With Regex: Single Quoted", t, func() {
		expression := getRegexpFromTriggerExp("$query matches '/(\\d*) *x *(\\d*) *x *(\\d*)/g'")
		So(expression, ShouldEqual, "/(\\d*) *x *(\\d*) *x *(\\d*)/g")
	})
	Convey("No match: Double Quoted", t, func() {
		expression := getRegexpFromTriggerExp(`$query contains "abc"`)
		So(expression, ShouldEqual, "")
	})
	Convey("One match: Double Quoted", t, func() {
		expression := getRegexpFromTriggerExp(`$query matches "abc"`)
		So(expression, ShouldEqual, "abc")
	})
	// First one should be picked
	Convey("Multiple matches: Double Quoted", t, func() {
		expression := getRegexpFromTriggerExp(`$query matches "abc" and $query matches "def"`)
		So(expression, ShouldEqual, "abc")
	})
	Convey("With Regex: Double Quoted", t, func() {
		expression := getRegexpFromTriggerExp(`$query matches "/(\\d*) *x *(\\d*) *x *(\\d*)/g"`)
		So(expression, ShouldEqual, `/(\\d*) *x *(\\d*) *x *(\\d*)/g`)
	})
}

func TestParseTriggerExprToLowerCase(t *testing.T) {
	Convey("Basic: Single Quoted", t, func() {
		expression := parseTriggerExprToLowerCase("$query contains 'Abc'")
		So(expression, ShouldEqual, "$query contains 'abc'")
	})
	Convey("Advanced: Single Quoted", t, func() {
		expression := parseTriggerExprToLowerCase("'product-Data' in Index and $filter.Category.Name == 'Mobile' and $query contains 'Acb'")
		So(expression, ShouldEqual, "'product-data' in Index and $filter.Category.Name == 'mobile' and $query contains 'acb'")
	})
	Convey("Filter with Brackets: Single Quoted", t, func() {
		expression := parseTriggerExprToLowerCase("$filter['Account Name.name.keyword'] matches 'John'")
		So(expression, ShouldEqual, "$filter['Account Name.name.keyword'] matches 'john'")
	})
	Convey("With Regex: Single Quoted", t, func() {
		expression := parseTriggerExprToLowerCase("$query matches '/(\\D*) *x *(\\D*) *x *(\\D*)/g'")
		So(expression, ShouldEqual, "$query matches '/(\\d*) *x *(\\d*) *x *(\\d*)/g'")
	})
	/* Test with double quotes */
	Convey("Basic: Double Quoted", t, func() {
		expression := parseTriggerExprToLowerCase(`$query contains "Abc"`)
		So(expression, ShouldEqual, `$query contains "abc"`)
	})
	Convey("Advanced: Double Quoted", t, func() {
		expression := parseTriggerExprToLowerCase(`"product-Data" in Index and $filter.Category.Name == "Mobile" and $query contains "Acb"`)
		So(expression, ShouldEqual, `"product-data" in Index and $filter.Category.Name == "mobile" and $query contains "acb"`)
	})
	Convey("Filter with Brackets: Double Quoted", t, func() {
		expression := parseTriggerExprToLowerCase(`$filter["Account Name.name.keyword"] matches "John"`)
		So(expression, ShouldEqual, `$filter["Account Name.name.keyword"] matches "john"`)
	})
	Convey("With Regex: Double Quoted", t, func() {
		expression := parseTriggerExprToLowerCase(`$query matches "/(\\D*) *x *(\\D*) *x *(\\D*)/g"`)
		So(expression, ShouldEqual, `$query matches "/(\\d*) *x *(\\d*) *x *(\\d*)/g"`)
	})
}

func TestRunScript(t *testing.T) {

	// Create rule instance
	r := Instance()

	// Default timeout
	timeout := 5 * time.Second

	// Inject a context into v8go, else there will be
	// an out of memory error
	// instantiate a new JavaScript VM
	r.iso = v8go.NewIsolate()
	global := v8go.NewObjectTemplate(r.iso)

	// Inject fetch support
	fetchfn := r.getFetchFn()
	global.Set("fetch", fetchfn, v8go.ReadOnly)

	// Inject base64 support
	base64.InjectTo(r.iso, global)

	// Instantiate for the first time
	r.ReInstantiateV8Context(global)

	Convey("Async Fetch", t, func() {
		scriptRequest := ScriptRequest{
			Body: "{\"query\":[{\"id\":\"search\",\"react\":{\"and\":\"color\"},\"dataField\":[\"name\"],\"size\":5,\"value\":\"vinyl\",\"includeFields\":[\"name\",\"color\"],\"index\":\"best-buy-dataset\"},{\"id\":\"color\",\"type\":\"term\",\"dataField\":\"color.keyword\",\"value\":[\"Black\"],\"execute\":false}]}",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}
		scriptResponse := ScriptResponse{
			Code: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: mockResponse,
		}

		// Convert the envs to map
		var scriptEnvs = make(map[string]interface{})
		envsToPass := "{\"index\":[\"test1\",\"test2\"],\"filters\":{\"filter1\":\"2011\",\"filter2\":\"product\"},\"query\":\"harry\",\"type\":\"search\",\"origin\":\"https://my-search.domain.com\",\"referer\":\"https://my-search.domain.com/path?q=hello\",\"ipv4\":\"29.120.12.12\",\"ipv6\":\"2001:db8:3333:4444:5555:6666:7777:8888\",\"customEvents\":{\"platform\":\"mac\"}}"
		_ = json.Unmarshal([]byte(envsToPass), &scriptEnvs)

		context := ScriptContext{
			Request:      scriptRequest,
			Response:     scriptResponse,
			Environments: scriptEnvs,
		}

		// Script to pass
		//
		// NOTE: Replace the nlp() call with a static string since the
		// output can differ per call for that method.
		scriptStr := "function handleRequest() { fetch('https://b7GLrKxsd:095e2eab-3800-491b-abf6-6b15cf8edf87@appbase-demo-ansible-abxiydt-arc.searchbase.io'); const body = JSON.parse(context.request.body); return { ...context.request, body: JSON.stringify({ ...body, query: [ ...body.query, { id: 'brandFilter', execute: false, type: 'term', dataField: 'brand.keyword', value: 'hariom-zIPtlcA', }, ], }), }; }"

		response, err := r.runScript(context, scriptStr, false, false, timeout, true, false)

		// Make sure error is nil
		So(err, ShouldEqual, nil)

		// Verify the response as well
		outputRequestExpected := ScriptRequest{
			Body: "{\"query\":[{\"id\":\"search\",\"react\":{\"and\":\"color\"},\"dataField\":[\"name\"],\"size\":5,\"value\":\"vinyl\",\"includeFields\":[\"name\",\"color\"],\"index\":\"best-buy-dataset\"},{\"id\":\"color\",\"type\":\"term\",\"dataField\":\"color.keyword\",\"value\":[\"Black\"],\"execute\":false},{\"id\":\"brandFilter\",\"execute\":false,\"type\":\"term\",\"dataField\":\"brand.keyword\",\"value\":\"hariom-zIPtlcA\"}]}",
		}
		outputResponseExpected := ScriptResponse{
			Code: http.StatusOK,
			Body: mockResponse,
		}

		// Validate request body
		So(response.Request.Body, ShouldEqual, outputRequestExpected.Body)
		So(response.Response.Body, ShouldEqual, outputResponseExpected.Body)
		So(response.Response.Code, ShouldEqual, outputResponseExpected.Code)
	})

	Convey("Sync Fetch", t, func() {
		scriptRequest := ScriptRequest{
			Body: "{\"query\":[{\"id\":\"search\",\"react\":{\"and\":\"color\"},\"dataField\":[\"name\"],\"size\":5,\"value\":\"vinyl\",\"includeFields\":[\"name\",\"color\"],\"index\":\"best-buy-dataset\"},{\"id\":\"color\",\"type\":\"term\",\"dataField\":\"color.keyword\",\"value\":[\"Black\"],\"execute\":false}]}",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}
		scriptResponse := ScriptResponse{
			Code: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: mockResponse,
		}

		// Convert the envs to map
		var scriptEnvs = make(map[string]interface{})
		envsToPass := "{\"index\":[\"test1\",\"test2\"],\"filters\":{\"filter1\":\"2011\",\"filter2\":\"product\"},\"query\":\"harry\",\"type\":\"search\",\"origin\":\"https://my-search.domain.com\",\"referer\":\"https://my-search.domain.com/path?q=hello\",\"ipv4\":\"29.120.12.12\",\"ipv6\":\"2001:db8:3333:4444:5555:6666:7777:8888\",\"customEvents\":{\"platform\":\"mac\"}}"
		_ = json.Unmarshal([]byte(envsToPass), &scriptEnvs)

		context := ScriptContext{
			Request:      scriptRequest,
			Response:     scriptResponse,
			Environments: scriptEnvs,
		}

		// Script to pass
		scriptStr := "async function handleRequest() { const res = await fetch('https://b7GLrKxsd:095e2eab-3800-491b-abf6-6b15cf8edf87@appbase-demo-ansible-abxiydt-arc.searchbase.io/_search'); const parsedResponse = await res.json(); const body = JSON.parse(context.request.body); return { ...context.request, body: JSON.stringify({ ...body, query: [ ...body.query, { id: 'brandFilter', execute: false, type: 'term', dataField: 'brand.keyword', }, ], }), }; }"

		response, err := r.runScript(context, scriptStr, false, false, timeout, true, false)

		// Make sure error is nil
		So(err, ShouldEqual, nil)

		// Verify the response as well
		outputRequestExpected := ScriptRequest{
			Body: "{\"query\":[{\"id\":\"search\",\"react\":{\"and\":\"color\"},\"dataField\":[\"name\"],\"size\":5,\"value\":\"vinyl\",\"includeFields\":[\"name\",\"color\"],\"index\":\"best-buy-dataset\"},{\"id\":\"color\",\"type\":\"term\",\"dataField\":\"color.keyword\",\"value\":[\"Black\"],\"execute\":false},{\"id\":\"brandFilter\",\"execute\":false,\"type\":\"term\",\"dataField\":\"brand.keyword\"}]}",
		}

		So(response.Request.Body, ShouldEqual, outputRequestExpected.Body)
	})

	Convey("Crypto JS", t, func() {
		scriptRequest := ScriptRequest{
			Body: "{\"query\":[{\"dataField\":[\"name\"],\"id\":\"search\",\"includeFields\":[\"name\",\"color\"],\"index\":\"best-buy-dataset\",\"react\":{\"and\":\"color\"},\"size\":5,\"value\":\"vinyl\"},{\"dataField\":\"color.keyword\",\"execute\":false,\"id\":\"color\",\"type\":\"term\",\"value\":[\"Black\"]}]}",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}
		scriptResponse := ScriptResponse{
			Code: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: mockResponse,
		}

		// Convert the envs to map
		var scriptEnvs = make(map[string]interface{})
		envsToPass := "{\"index\":[\"test1\",\"test2\"],\"filters\":{\"filter1\":\"2011\",\"filter2\":\"product\"},\"query\":\"harry\",\"type\":\"search\",\"origin\":\"https://my-search.domain.com\",\"referer\":\"https://my-search.domain.com/path?q=hello\",\"ipv4\":\"29.120.12.12\",\"ipv6\":\"2001:db8:3333:4444:5555:6666:7777:8888\",\"customEvents\":{\"platform\":\"mac\"}}"
		_ = json.Unmarshal([]byte(envsToPass), &scriptEnvs)

		context := ScriptContext{
			Request:      scriptRequest,
			Response:     scriptResponse,
			Environments: scriptEnvs,
		}

		// Script to pass
		scriptStr := "function handleRequest() { return { ...context.request, body: context.request.body, headers: { ...context.request.headers, message: `${CryptoJS.SHA256( _.get(context.request.headers, `X-Customheader`) )}`, }, }; }"

		response, err := r.runScript(context, scriptStr, false, false, timeout, true, false)

		// Make sure err is nil
		So(err, ShouldEqual, nil)

		headerMessageExpected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

		So(response.Request.Headers["message"], ShouldEqual, headerMessageExpected)
	})

	Convey("CompromiseJS", t, func() {
		// Body doesn't matter since the script modifies the response
		scriptRequest := ScriptRequest{
			Body: "{\"query\":[{\"id\":\"search\",\"react\":{\"and\":\"color\"},\"dataField\":[\"name\"],\"size\":5,\"value\":\"vinyl\",\"includeFields\":[\"name\",\"color\"],\"index\":\"best-buy-dataset\"},{\"id\":\"color\",\"type\":\"term\",\"dataField\":\"color.keyword\",\"value\":[\"Black\"],\"execute\":false}]}",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}
		scriptResponse := ScriptResponse{
			Code: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: mockResponse,
		}

		// Convert the envs to map
		var scriptEnvs = make(map[string]interface{})
		envsToPass := "{\"index\":[\"test1\",\"test2\"],\"filters\":{\"filter1\":\"2011\",\"filter2\":\"product\"},\"query\":\"harry\",\"type\":\"search\",\"origin\":\"https://my-search.domain.com\",\"referer\":\"https://my-search.domain.com/path?q=hello\",\"ipv4\":\"29.120.12.12\",\"ipv6\":\"2001:db8:3333:4444:5555:6666:7777:8888\",\"customEvents\":{\"platform\":\"mac\"}}"
		_ = json.Unmarshal([]byte(envsToPass), &scriptEnvs)

		context := ScriptContext{
			Request:      scriptRequest,
			Response:     scriptResponse,
			Environments: scriptEnvs,
		}

		// Script to pass
		scriptStr := "function handleRequest() { const body = JSON.parse(context.request.body); return { ...context.request, body: JSON.stringify({ ...body, query: [ ...body.query, { id: 'brandFilter', execute: false, type: 'term', dataField: 'brand.keyword', value: nlp('the purple dinosaur').nouns().toPlural().text(), }, ], }), }; }"

		response, err := r.runScript(context, scriptStr, false, false, timeout, true, false)

		// Make sure err is nil
		So(err, ShouldEqual, nil)

		// Validate the request returned
		expectedRequestBody := "{\"query\":[{\"id\":\"search\",\"react\":{\"and\":\"color\"},\"dataField\":[\"name\"],\"size\":5,\"value\":\"vinyl\",\"includeFields\":[\"name\",\"color\"],\"index\":\"best-buy-dataset\"},{\"id\":\"color\",\"type\":\"term\",\"dataField\":\"color.keyword\",\"value\":[\"Black\"],\"execute\":false},{\"id\":\"brandFilter\",\"execute\":false,\"type\":\"term\",\"dataField\":\"brand.keyword\",\"value\":\"dinosaurs\"}]}"

		So(response.Request.Body, ShouldEqual, expectedRequestBody)
	})

	Convey("Response modification with Lodash", t, func() {
		// Body doesn't matter since the script modifies the response
		scriptRequest := ScriptRequest{
			Body: "{\"query\":[{\"id\":\"search\",\"react\":{\"and\":\"color\"},\"dataField\":[\"name\"],\"size\":5,\"value\":\"vinyl\",\"includeFields\":[\"name\",\"color\"],\"index\":\"best-buy-dataset\"},{\"id\":\"color\",\"type\":\"term\",\"dataField\":\"color.keyword\",\"value\":[\"Black\"],\"execute\":false}]}",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}
		scriptResponse := ScriptResponse{
			Code: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: "{\"search\":{\"_shards\":{\"failed\":0,\"skipped\":0,\"successful\":3,\"total\":3},\"hits\":{\"hits\":[{\"_id\":\"UXGArnYBpOdhck8TDcFv\",\"_index\":\"best-buy-dataset\",\"_score\":5.7865844,\"_source\":{\"color\":\"Black\",\"name\":\"Cricut-PremiumRemovableVinyl-Black\"},\"_type\":\"_doc\"},{\"_id\":\"4Od7rnYBR5qBrhW_U3tj\",\"_index\":\"best-buy-dataset\",\"_score\":5.5054154,\"_source\":{\"color\":\"Black\",\"name\":\"UncagedErgonomics-VinylWobbleStool-Black\"},\"_type\":\"_doc\"},{\"_id\":\"cOd_rnYBR5qBrhW_gZB7\",\"_index\":\"best-buy-dataset\",\"_score\":5.476978,\"_source\":{\"color\":\"Black\",\"name\":\"Crosley-VinylRecordCleaningSet-Black\"},\"_type\":\"_doc\"},{\"_id\":\"XedzrnYBR5qBrhW_KlLb\",\"_index\":\"best-buy-dataset\",\"_score\":5.2471347,\"_source\":{\"color\":\"Black\",\"name\":\"OfficeStarProducts-VinylDraftingStool-Black\"},\"_type\":\"_doc\"},{\"_id\":\"z3F0rnYBpOdhck8T-4lM\",\"_index\":\"best-buy-dataset\",\"_score\":5.237612,\"_source\":{\"color\":\"Black\",\"name\":\"Pro-Ject-VinylCleanerVC-S-Black\"},\"_type\":\"_doc\"}],\"max_score\":5.7865844,\"total\":{\"relation\":\"eq\",\"value\":22}},\"status\":200,\"timed_out\":false,\"took\":6},\"settings\":{\"searchRelevancy\":\"best-buy-dataset\",\"took\":6}}",
		}

		// Convert the envs to map
		var scriptEnvs = make(map[string]interface{})
		envsToPass := "{\"index\":[\"test1\",\"test2\"],\"filters\":{\"filter1\":\"2011\",\"filter2\":\"product\"},\"query\":\"harry\",\"type\":\"search\",\"origin\":\"https://my-search.domain.com\",\"referer\":\"https://my-search.domain.com/path?q=hello\",\"ipv4\":\"29.120.12.12\",\"ipv6\":\"2001:db8:3333:4444:5555:6666:7777:8888\",\"customEvents\":{\"platform\":\"mac\"}}"
		_ = json.Unmarshal([]byte(envsToPass), &scriptEnvs)

		context := ScriptContext{
			Request:      scriptRequest,
			Response:     scriptResponse,
			Environments: scriptEnvs,
		}

		// Script to pass
		scriptStr := "function handleResponse() { const body = _.omit(JSON.parse(context.response.body), `search._shards`); return { ...context.response, body: JSON.stringify({ ...body }) }; }"

		response, err := r.runScript(context, scriptStr, false, false, timeout, true, false)

		// Make sure err is nil
		So(err, ShouldEqual, nil)

		// Verify response
		responseBodyExpected := "{\"search\":{\"hits\":{\"hits\":[{\"_id\":\"UXGArnYBpOdhck8TDcFv\",\"_index\":\"best-buy-dataset\",\"_score\":5.7865844,\"_source\":{\"color\":\"Black\",\"name\":\"Cricut-PremiumRemovableVinyl-Black\"},\"_type\":\"_doc\"},{\"_id\":\"4Od7rnYBR5qBrhW_U3tj\",\"_index\":\"best-buy-dataset\",\"_score\":5.5054154,\"_source\":{\"color\":\"Black\",\"name\":\"UncagedErgonomics-VinylWobbleStool-Black\"},\"_type\":\"_doc\"},{\"_id\":\"cOd_rnYBR5qBrhW_gZB7\",\"_index\":\"best-buy-dataset\",\"_score\":5.476978,\"_source\":{\"color\":\"Black\",\"name\":\"Crosley-VinylRecordCleaningSet-Black\"},\"_type\":\"_doc\"},{\"_id\":\"XedzrnYBR5qBrhW_KlLb\",\"_index\":\"best-buy-dataset\",\"_score\":5.2471347,\"_source\":{\"color\":\"Black\",\"name\":\"OfficeStarProducts-VinylDraftingStool-Black\"},\"_type\":\"_doc\"},{\"_id\":\"z3F0rnYBpOdhck8T-4lM\",\"_index\":\"best-buy-dataset\",\"_score\":5.237612,\"_source\":{\"color\":\"Black\",\"name\":\"Pro-Ject-VinylCleanerVC-S-Black\"},\"_type\":\"_doc\"}],\"max_score\":5.7865844,\"total\":{\"relation\":\"eq\",\"value\":22}},\"status\":200,\"timed_out\":false,\"took\":6},\"settings\":{\"searchRelevancy\":\"best-buy-dataset\",\"took\":6}}"

		So(response.Response.Body, ShouldEqual, responseBodyExpected)
	})

	Convey("Index request modification", t, func() {
		// Body doesn't matter since the script modifies the response
		scriptRequest := ScriptRequest{
			Body: "{\"data\":\"helloworld\"}",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}
		scriptResponse := ScriptResponse{
			Code: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: "{\"search\":{\"_shards\":{\"failed\":0,\"skipped\":0,\"successful\":3,\"total\":3},\"hits\":{\"hits\":[{\"_id\":\"UXGArnYBpOdhck8TDcFv\",\"_index\":\"best-buy-dataset\",\"_score\":5.7865844,\"_source\":{\"color\":\"Black\",\"name\":\"Cricut-PremiumRemovableVinyl-Black\"},\"_type\":\"_doc\"},{\"_id\":\"4Od7rnYBR5qBrhW_U3tj\",\"_index\":\"best-buy-dataset\",\"_score\":5.5054154,\"_source\":{\"color\":\"Black\",\"name\":\"UncagedErgonomics-VinylWobbleStool-Black\"},\"_type\":\"_doc\"},{\"_id\":\"cOd_rnYBR5qBrhW_gZB7\",\"_index\":\"best-buy-dataset\",\"_score\":5.476978,\"_source\":{\"color\":\"Black\",\"name\":\"Crosley-VinylRecordCleaningSet-Black\"},\"_type\":\"_doc\"},{\"_id\":\"XedzrnYBR5qBrhW_KlLb\",\"_index\":\"best-buy-dataset\",\"_score\":5.2471347,\"_source\":{\"color\":\"Black\",\"name\":\"OfficeStarProducts-VinylDraftingStool-Black\"},\"_type\":\"_doc\"},{\"_id\":\"z3F0rnYBpOdhck8T-4lM\",\"_index\":\"best-buy-dataset\",\"_score\":5.237612,\"_source\":{\"color\":\"Black\",\"name\":\"Pro-Ject-VinylCleanerVC-S-Black\"},\"_type\":\"_doc\"}],\"max_score\":5.7865844,\"total\":{\"relation\":\"eq\",\"value\":22}},\"status\":200,\"timed_out\":false,\"took\":6},\"settings\":{\"searchRelevancy\":\"best-buy-dataset\",\"took\":6}}",
		}

		// Convert the envs to map
		var scriptEnvs = make(map[string]interface{})
		envsToPass := "{\"acl\":\"index\",\"index\":[\"test1\"]}"
		_ = json.Unmarshal([]byte(envsToPass), &scriptEnvs)

		context := ScriptContext{
			Request:      scriptRequest,
			Response:     scriptResponse,
			Environments: scriptEnvs,
		}

		// Script to pass
		scriptStr := "function handleRequest() { if (['update', 'index'].includes(context.envs.acl)) { const requestBody = JSON.parse(context.request.body); requestBody.custom = true; return { ...context.request, body: JSON.stringify(requestBody) }; } return context.request; }"

		response, err := r.runScript(context, scriptStr, false, false, timeout, true, false)

		// Make sure err is nil
		So(err, ShouldEqual, nil)

		// Validate request
		requestBodyExpected := "{\"data\":\"helloworld\",\"custom\":true}"

		So(response.Request.Body, ShouldEqual, requestBodyExpected)
	})

	Convey("Bulk request modification", t, func() {
		// Body doesn't matter since the script modifies the response
		scriptRequest := ScriptRequest{
			Body: "{\"index\" : { \"_index\" : \"test\", \"_id\" : \"1\" } }\n{ \"query\" : \"value1\" }\n",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}
		scriptResponse := ScriptResponse{
			Code: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: "{\"search\":{\"_shards\":{\"failed\":0,\"skipped\":0,\"successful\":3,\"total\":3},\"hits\":{\"hits\":[{\"_id\":\"UXGArnYBpOdhck8TDcFv\",\"_index\":\"best-buy-dataset\",\"_score\":5.7865844,\"_source\":{\"color\":\"Black\",\"name\":\"Cricut-PremiumRemovableVinyl-Black\"},\"_type\":\"_doc\"},{\"_id\":\"4Od7rnYBR5qBrhW_U3tj\",\"_index\":\"best-buy-dataset\",\"_score\":5.5054154,\"_source\":{\"color\":\"Black\",\"name\":\"UncagedErgonomics-VinylWobbleStool-Black\"},\"_type\":\"_doc\"},{\"_id\":\"cOd_rnYBR5qBrhW_gZB7\",\"_index\":\"best-buy-dataset\",\"_score\":5.476978,\"_source\":{\"color\":\"Black\",\"name\":\"Crosley-VinylRecordCleaningSet-Black\"},\"_type\":\"_doc\"},{\"_id\":\"XedzrnYBR5qBrhW_KlLb\",\"_index\":\"best-buy-dataset\",\"_score\":5.2471347,\"_source\":{\"color\":\"Black\",\"name\":\"OfficeStarProducts-VinylDraftingStool-Black\"},\"_type\":\"_doc\"},{\"_id\":\"z3F0rnYBpOdhck8T-4lM\",\"_index\":\"best-buy-dataset\",\"_score\":5.237612,\"_source\":{\"color\":\"Black\",\"name\":\"Pro-Ject-VinylCleanerVC-S-Black\"},\"_type\":\"_doc\"}],\"max_score\":5.7865844,\"total\":{\"relation\":\"eq\",\"value\":22}},\"status\":200,\"timed_out\":false,\"took\":6},\"settings\":{\"searchRelevancy\":\"best-buy-dataset\",\"took\":6}}",
		}

		// Convert the envs to map
		var scriptEnvs = make(map[string]interface{})
		envsToPass := "{\"acl\":\"bulk\",\"index\":[\"test1\"]}"
		_ = json.Unmarshal([]byte(envsToPass), &scriptEnvs)

		context := ScriptContext{
			Request:      scriptRequest,
			Response:     scriptResponse,
			Environments: scriptEnvs,
		}

		// Script to pass
		scriptStr := "function handleRequest() { if (context.envs.acl === 'bulk') { const jsonRows = context.request.body.split('\\n'); if (jsonRows[jsonRows.length - 1] == '') { jsonRows.pop(); } const data = jsonRows.map((jsonStringRow) => { let bodyEach = JSON.parse(jsonStringRow); if (!('index' in bodyEach)) { bodyEach = { ...bodyEach, queryLength: bodyEach.query ? bodyEach.query.length : 0, }; } return JSON.stringify(bodyEach); }); context.request.body = data.join('\\n') + '\\n'; return context.request; } return context.request; }"

		response, err := r.runScript(context, scriptStr, false, false, timeout, true, false)

		// Make sure err is nil
		So(err, ShouldEqual, nil)

		requestBodyExpected := "{\"index\":{\"_index\":\"test\",\"_id\":\"1\"}}\n{\"query\":\"value1\",\"queryLength\":6}\n"

		So(response.Request.Body, ShouldEqual, requestBodyExpected)
	})
}
