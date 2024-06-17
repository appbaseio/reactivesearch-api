package uibuilder

// Note: it doesn't store featured suggestions because they get stored in Zinc index
var cachedSearchBoxes = make(map[string]SearchBoxESModel)

// To retrieve the cached searchboxes
func GetCachedSearchBoxes(filterHidden bool, filterEnabled bool) map[string]SearchBoxESModel {
	tempSearchBoxes := make(map[string]SearchBoxESModel)
	for _, searchBox := range cachedSearchBoxes {
		if searchBox.Id != nil {
			if filterEnabled {
				if searchBox.Enabled != nil && !*searchBox.Enabled {
					continue
				}
			}
			if filterHidden {
				if searchBox.Hidden != nil && !*searchBox.Hidden {
					continue
				}
			}
			tempSearchBoxes[*searchBox.Id] = searchBox
		}
	}
	return tempSearchBoxes
}

// To retrieve the cached searchbox
func GetCachedSearchBox(id string) *SearchBoxESModel {
	for _, searchbox := range cachedSearchBoxes {
		if searchbox.Id != nil && *searchbox.Id == id {
			return &searchbox
		}
	}
	return nil
}

func SetCachedSearchBoxes(searchBoxes []SearchBoxESModel) {
	temp := make(map[string]SearchBoxESModel)
	for _, v := range searchBoxes {
		if v.Id != nil {
			if v.SearchBox != nil &&
				v.SearchBox.Featured != nil &&
				v.SearchBox.Featured.Layout != nil {
				// Avoid storing sections
				v.SearchBox.Featured.Layout.Sections = make([]SectionInfo, 0)
			}
			temp[*v.Id] = v
		}
	}
	cachedSearchBoxes = temp
}

func AddSearchBoxToCache(searchbox SearchBoxESModel) {
	tempSearchBox := searchbox
	if tempSearchBox.Id != nil {
		if tempSearchBox.SearchBox != nil &&
			tempSearchBox.SearchBox.Featured != nil &&
			tempSearchBox.SearchBox.Featured.Layout != nil {
			// Avoid storing sections
			tempSearchBox.SearchBox.Featured.Layout.Sections = make([]SectionInfo, 0)
		}
		cachedSearchBoxes[*tempSearchBox.Id] = tempSearchBox
	}
}

func DeleteSearchBoxToCache(id string) {
	delete(cachedSearchBoxes, id)
}
