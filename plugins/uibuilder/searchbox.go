package uibuilder

import (
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/util"
)

// Featured suggestion document stored in ES
type SearchBoxESFeaturedSuggestionDoc struct {
	Label       string                     `json:"label,omitempty"`
	Value       string                     `json:"value,omitempty"`
	Description *string                    `json:"description,omitempty"`
	Action      *querytranslate.ActionType `json:"action,omitempty"`
	Id          *string                    `json:"id,omitempty"`
	SubAction   *string                    `json:"subAction,omitempty"`
	Icon        *string                    `json:"icon,omitempty"`
	IconURL     *string                    `json:"iconURL,omitempty"`
}

type SectionInfo struct {
	Id          *string                            `json:"id,omitempty"`
	Label       *string                            `json:"label,omitempty"`
	Suggestions []SearchBoxESFeaturedSuggestionDoc `json:"suggestions,omitempty"`
}

type Layout struct {
	Sections                 []SectionInfo `json:"sections,omitempty"`
	MaxSuggestionsPerSection *int          `json:"maxSuggestionsPerSection,omitempty"`
	SectionsOrder            *[]string     `json:"sectionsOrder,omitempty"`
}

type FeaturedPreferences struct {
	SectionLabel *string                 `json:"sectionLabel,omitempty"`
	Design       *map[string]interface{} `json:"design,omitempty"`
	Layout       *Layout                 `json:"layout,omitempty"`
}

type DocumentPreferences struct {
	SectionLabel     *string `json:"sectionLabel,omitempty"`
	RenderSuggestion *string `json:"renderSuggestion,omitempty"`
}

// Endpoint struct
type Endpoint struct {
	URL                 *string            `json:"url,omitempty"`
	Method              *string            `json:"method,omitempty"`
	HeadersMap          *map[string]string `json:"headers,omitempty"`
	BodyMap             *interface{}       `json:"body,omitempty"`
	Headers             *string            `json:"headers_string,omitempty"`
	Body                *string            `json:"body_string,omitempty"`
	IsUsingStringFormat bool               `json:"is_using_string_format,omitempty"`
}

type EndpointPreferences struct {
	SectionLabel                *string   `json:"sectionLabel,omitempty"`
	Endpoint                    *Endpoint `json:"endpoint,omitempty"`
	TransformResponse           *string   `json:"transformResponse,omitempty"`
	ShowDistinctSuggestions     *bool     `json:"showDistinctSuggestions,omitempty"`
	EnablePredictiveSuggestions *bool     `json:"enablePredictiveSuggestions,omitempty"`
	MaxPredictedWords           *int      `json:"maxPredictedWords,omitempty"`
	ApplyStopwords              *bool     `json:"applyStopwords,omitempty"`
	CustomStopwords             *[]string `json:"customStopwords,omitempty"`
	EnableSynonyms              *bool     `json:"enableSynonyms,omitempty"`
	IncludeFields               *[]string `json:"includeFields,omitempty"`
	ExcludeFields               *[]string `json:"excludeFields,omitempty"`
	URLField                    *string   `json:"urlField,omitempty"`
}

type SearchBoxPreference struct {
	Popular  *querytranslate.PopularSuggestionsOptions `json:"popular,omitempty"`
	Recent   *querytranslate.RecentSuggestionsOptions  `json:"recent,omitempty"`
	Endpoint *EndpointPreferences                      `json:"endpoint,omitempty"`
	Featured *FeaturedPreferences                      `json:"featured,omitempty"`
	Document *DocumentPreferences                      `json:"document,omitempty"`
	FAQ      *map[string]string                        `json:"faq,omitempty"`
}

type SearchBoxESModel struct {
	Id          *string              `json:"id,omitempty"`
	Description *string              `json:"description,omitempty"`
	Enabled     *bool                `json:"enabled,omitempty"`
	Hidden      *bool                `json:"hidden,omitempty"`
	SearchBox   *SearchBoxPreference `json:"searchbox,omitempty"`
	CreatedAt   *int64               `json:"created_at,omitempty"`
	UpdatedAt   *int64               `json:"updated_at,omitempty"`
}

// Returns the featured suggestions
func GetFeaturedSuggestions(query *querytranslate.Query, value string) ([]querytranslate.SuggestionHIT, error) {
	featuredSuggestions := make([]querytranslate.SuggestionHIT, 0)
	if query.Type != querytranslate.Suggestion {
		return featuredSuggestions, nil
	}
	if query.SearchBoxId == nil {
		return featuredSuggestions, nil
	}
	if query.EnableFeaturedSuggestions != nil && !*query.EnableFeaturedSuggestions {
		return featuredSuggestions, nil
	}
	if query.FeaturedSuggestionsConfig != nil && query.FeaturedSuggestionsConfig.MaxSuggestionsPerSection != nil &&
		*query.FeaturedSuggestionsConfig.MaxSuggestionsPerSection == 0 {
		return featuredSuggestions, nil
	}
	featuredSuggestionsESDoc, err := singleton.featuredSuggestionsConfig.SearchFeaturedSuggestions(*query.SearchBoxId, value)
	if err != nil {
		return featuredSuggestions, err
	}
	defaultSectionsOrder := []string{}
	// Group suggestions by section
	var featuredSuggestionsBySections = make(map[string][]ESFeaturedSuggestionDoc)
	for _, featuredSuggestion := range featuredSuggestionsESDoc {
		if featuredSuggestion.SectionId != nil {
			// maintain default sections order
			if !util.Contains(defaultSectionsOrder, *featuredSuggestion.SectionId) {
				defaultSectionsOrder = append(defaultSectionsOrder, *featuredSuggestion.SectionId)
			}

			if featuredSuggestionsBySections[*featuredSuggestion.SectionId] == nil {
				featuredSuggestionsBySections[*featuredSuggestion.SectionId] = []ESFeaturedSuggestionDoc{featuredSuggestion}
			} else {
				featuredSuggestionsBySections[*featuredSuggestion.SectionId] = append(featuredSuggestionsBySections[*featuredSuggestion.SectionId], featuredSuggestion)
			}
		}
	}

	sectionsOrder := []string{}
	if query.FeaturedSuggestionsConfig != nil && query.FeaturedSuggestionsConfig.SectionsOrder != nil {
		sectionsOrder = *query.FeaturedSuggestionsConfig.SectionsOrder
	}
	// populate order for missing sections
	for _, section := range defaultSectionsOrder {
		if !util.Contains(sectionsOrder, section) {
			sectionsOrder = append(sectionsOrder, section)
		}
	}

	for _, section := range sectionsOrder {
		if featuredSuggestionsBySection, ok := featuredSuggestionsBySections[section]; ok {
			featuredSuggestionsPerSection := make([]querytranslate.SuggestionHIT, 0)
			// TODO: Calculate matched tokens for featured suggestions

			for _, featuredSuggestion := range featuredSuggestionsBySection {
				var id string
				if featuredSuggestion.Id != nil {
					id = *featuredSuggestion.Id
				}
				featuredSuggestionsPerSection = append(featuredSuggestionsPerSection, querytranslate.SuggestionHIT{
					Label:        featuredSuggestion.Label,
					Value:        featuredSuggestion.Value,
					SectionLabel: featuredSuggestion.SectionLabel,
					SectionId:    featuredSuggestion.SectionId,
					Description:  featuredSuggestion.Description,
					Action:       featuredSuggestion.Action,
					SubAction:    featuredSuggestion.SubAction,
					Icon:         featuredSuggestion.Icon,
					IconURL:      featuredSuggestion.IconURL,
					Type:         querytranslate.Featured,
					Id:           id,
					// Featured Suggestions properties

					// RSScore       float64        `json:"_rs_score"`
					// MatchedTokens []string       `json:"_matched_tokens"`
					// ES response properties
					Score:  *featuredSuggestion.Score,
					Source: make(map[string]interface{}),
				})
			}

			// Respect max suggestion per section constraint
			if query.FeaturedSuggestionsConfig != nil && query.FeaturedSuggestionsConfig.MaxSuggestionsPerSection != nil {
				if len(featuredSuggestionsPerSection) > *query.FeaturedSuggestionsConfig.MaxSuggestionsPerSection {
					featuredSuggestionsPerSection = featuredSuggestionsPerSection[:*query.FeaturedSuggestionsConfig.MaxSuggestionsPerSection]
				}
			}
			featuredSuggestions = append(featuredSuggestions, featuredSuggestionsPerSection...)
		}
	}

	return featuredSuggestions, nil
}

// Featured suggestion document stored in ES
type ESFeaturedSuggestionDoc struct {
	Label        string                     `json:"label,omitempty"`
	Value        string                     `json:"value,omitempty"`
	Description  *string                    `json:"description,omitempty"`
	Action       *querytranslate.ActionType `json:"action,omitempty"`
	SearchboxId  *string                    `json:"searchboxId,omitempty"`
	Id           *string                    `json:"id,omitempty"`
	SubAction    *string                    `json:"subAction,omitempty"`
	SectionId    *string                    `json:"sectionId,omitempty"`
	SectionLabel *string                    `json:"sectionLabel,omitempty"`
	Icon         *string                    `json:"icon,omitempty"`
	IconURL      *string                    `json:"iconURL,omitempty"`
	Score        *float64                   `json:"_score,omitempty"`
	Order        *int                       `json:"order,omitempty"`
}
