GC=go build
GT=go test

BUILD_DIR=build
PLUGIN_FLAGS=--buildmode=plugin
PLUGIN_BUILD_DIR=$(BUILD_DIR)/plugins
DEFAULT_VERSION=7.50.1
VERSION := $(or $(VERSION),$(DEFAULT_VERSION))
 
PLUGINS=$(shell ls -l plugins | grep ^d | awk '{ print $$9 }')
PLUGIN_MAIN_LOC_FUNC=plugins/$(1)/main/$(1).$(2)
PLUGIN_LOC_FUNC=$(foreach PLUGIN,$(PLUGINS),$(call PLUGIN_MAIN_LOC_FUNC,$(PLUGIN),$(1)))

cmd: plugins
	$(GC) -ldflags "-w -X main.Billing=$(BILLING) -X main.HostedBilling=$(HOSTED_BILLING) -X main.ClusterBilling=$(CLUSTER_BILLING) -X main.Opensource=$(OPENSOURCE) -X main.PlanRefreshInterval=$(PLAN_REFRESH_INTERVAL) -X main.IgnoreBillingMiddleware=$(IGNORE_BILLING_MIDDLEWARE) -X main.Tier=$(TEST_TIER) -X main.FeatureCustomEvents=$(TEST_FEATURE_CUSTOM_EVENTS) -X main.FeatureSuggestions=$(TEST_FEATURE_SUGGESTIONS) -X main.Version=$(VERSION)" -o $(BUILD_DIR)/reactivesearch main.go

plugins: $(call PLUGIN_LOC_FUNC,so)

$(call PLUGIN_LOC_FUNC,so): %.so: %.go
	$(GC) $(PLUGIN_FLAGS) -o $(PLUGIN_BUILD_DIR)/$(@F) $<

test: 
	$(GT) -p 1 ./... -tags=unit
clean:
	rm -rf $(BUILD_DIR)
