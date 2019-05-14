GC=go build

BUILD_DIR=build
PLUGIN_FLAGS=--buildmode=plugin
PLUGIN_BUILD_DIR=$(BUILD_DIR)/plugins

PLUGINS=analytics auth elasticsearch logs permissions reindexer rules users
PLUGIN_MAIN_LOC_FUNC=plugins/$(1)/main/$(1).$(2)
PLUGIN_LOC_FUNC=$(foreach PLUGIN,$(PLUGINS),$(call PLUGIN_MAIN_LOC_FUNC,$(PLUGIN),$(1)))

.DEFAULT_GOAL: plugins
	$(GC) -o $(BUILD_DIR)/arc arc/cmd/main.go

plugins: $(call PLUGIN_LOC_FUNC,so)

$(call PLUGIN_LOC_FUNC,so): %.so: %.go
	$(GC) $(PLUGIN_FLAGS) -o $(PLUGIN_BUILD_DIR)/$(@F) $<

clean:
	rm -rf $(BUILD_DIR)
