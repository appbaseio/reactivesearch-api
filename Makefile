GC=go build

BUILD_DIR=build
PLUGIN_FLAGS=--buildmode=plugin
PLUGIN_BUILD_DIR=$(BUILD_DIR)/plugins

PLUGINS=$(shell ls -l plugins | grep ^d | awk '{ print $$9 }')
PLUGIN_MAIN_LOC_FUNC=plugins/$(1)/main/$(1).$(2)
PLUGIN_LOC_FUNC=$(foreach PLUGIN,$(PLUGINS),$(call PLUGIN_MAIN_LOC_FUNC,$(PLUGIN),$(1)))

cmd: plugins
	$(GC) -ldflags "-X main.Billing=$(BILLING) -X main.HostedBilling=$(HOSTED_BILLING) -X main.ClusterBilling=$(CLUSTER_BILLING) -X main.PlanRefreshInterval=$(PLAN_REFRESH_INTERVAL) -X main.IgnoreBillingMiddleware=$(IGNORE_BILLING_MIDDLEWARE)" -o $(BUILD_DIR)/arc main.go

plugins: $(call PLUGIN_LOC_FUNC,so)

$(call PLUGIN_LOC_FUNC,so): %.so: %.go
	$(GC) $(PLUGIN_FLAGS) -o $(PLUGIN_BUILD_DIR)/$(@F) $<

clean:
	rm -rf $(BUILD_DIR)
