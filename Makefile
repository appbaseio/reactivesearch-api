GC=go build

BUILD_DIR=build
DEFAULT_VERSION=9.2.0
VERSION := $(or $(VERSION),$(DEFAULT_VERSION))

cmd: build

build:
	$(GC) -pgo=auto -ldflags "-w -X main.Billing=$(BILLING) -X main.HostedBilling=$(HOSTED_BILLING) -X main.ClusterBilling=$(CLUSTER_BILLING) -X main.Opensource=$(OPENSOURCE) -X main.PlanRefreshInterval=$(PLAN_REFRESH_INTERVAL) -X main.IgnoreBillingMiddleware=$(IGNORE_BILLING_MIDDLEWARE) -X main.Tier=$(TEST_TIER) -X main.FeatureCustomEvents=$(TEST_FEATURE_CUSTOM_EVENTS) -X main.FeatureSuggestions=$(TEST_FEATURE_SUGGESTIONS) -X main.FeatureOpenAI=$(TEST_FEATURE_OPEN_AI) -X main.Version=$(VERSION)" -o $(BUILD_DIR)/reactivesearch github.com/appbaseio/reactivesearch-api

clean:
	rm -rf $(BUILD_DIR)
