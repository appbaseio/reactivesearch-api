GC=go build

PLUGIN_FLAGS=--buildmode=plugin
PLUGIN_BUILD_DIR=build/plugins

PLUGINS=analytics auth elasticsearch logs permissions reindexer rules users
PLUGIN_MAIN_LOC_FUNC=plugins/$(1)/main/$(1).$(2)
PLUGIN_LOC_FUNC=$(foreach PLUGIN,$(PLUGINS),$(call PLUGIN_MAIN_LOC_FUNC,$(PLUGIN),$(1)))

arcmake: $(call PLUGIN_LOC_FUNC,so)
	$(GC) -o build/arc arc/cmd/main.go

$(call PLUGIN_LOC_FUNC,so): %.so: %.go
	$(GC) $(PLUGIN_FLAGS) -o $(PLUGIN_BUILD_DIR)/$(@F) $<
