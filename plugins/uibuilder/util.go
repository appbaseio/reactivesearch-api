package uibuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type PreferenceType int

const (
	Search PreferenceType = iota
	Recommendation
)

// String is the implementation of Stringer interface that returns the string representation of PreferenceType type.
func (o PreferenceType) String() string {
	return [...]string{
		"search",
		"recommendation",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling PreferenceType type.
func (o *PreferenceType) UnmarshalJSON(bytes []byte) error {
	var preferenceType string
	err := json.Unmarshal(bytes, &preferenceType)
	if err != nil {
		return err
	}
	switch preferenceType {
	case Search.String():
		*o = Search
	case Recommendation.String():
		*o = Recommendation
	default:
		return fmt.Errorf("invalid export type encountered: %v", preferenceType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling PreferenceType type.
func (o PreferenceType) MarshalJSON() ([]byte, error) {
	var preferenceType string
	switch o {
	case Search:
		preferenceType = Search.String()
	case Recommendation:
		preferenceType = Recommendation.String()
	default:
		return nil, fmt.Errorf("invalid export type encountered: %v", o)
	}
	return json.Marshal(preferenceType)
}

type ExportType int

const (
	Other ExportType = iota
	Shopify
)

// String is the implementation of Stringer interface that returns the string representation of ExportType type.
func (o ExportType) String() string {
	return [...]string{
		"other",
		"shopify",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling ExportType type.
func (o *ExportType) UnmarshalJSON(bytes []byte) error {
	var exportType string
	err := json.Unmarshal(bytes, &exportType)
	if err != nil {
		return err
	}
	switch exportType {
	case Other.String():
		*o = Other
	case Shopify.String():
		*o = Shopify
	default:
		return fmt.Errorf("invalid export type encountered: %v", exportType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling SortBy type.
func (o ExportType) MarshalJSON() ([]byte, error) {
	var exportType string
	switch o {
	case Other:
		exportType = Other.String()
	case Shopify:
		exportType = Shopify.String()
	default:
		return nil, fmt.Errorf("invalid export type encountered: %v", o)
	}
	return json.Marshal(exportType)
}

type ExportAs int

const (
	Embed ExportAs = iota
	Hackable
)

// String is the implementation of Stringer interface that returns the string representation of ExportAs type.
func (o ExportAs) String() string {
	return [...]string{
		"embed",
		"hackable",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling ExportAs type.
func (o *ExportAs) UnmarshalJSON(bytes []byte) error {
	var exportAs string
	err := json.Unmarshal(bytes, &exportAs)
	if err != nil {
		return err
	}
	switch exportAs {
	case Embed.String():
		*o = Embed
	case Hackable.String():
		*o = Hackable
	default:
		return fmt.Errorf("invalid export as encountered: %v", exportAs)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling ExportAs type.
func (o ExportAs) MarshalJSON() ([]byte, error) {
	var exportAs string
	switch o {
	case Embed:
		exportAs = Embed.String()
	case Hackable:
		exportAs = Hackable.String()
	default:
		return nil, fmt.Errorf("invalid export as encountered: %v", o)
	}
	return json.Marshal(exportAs)
}

type StaticFilterType int

const (
	ProductType StaticFilterType = iota
	Collection
	Color
	Size
	Price
)

// String is the implementation of Stringer interface that returns the string representation of StaticFilterType type.
func (o StaticFilterType) String() string {
	return [...]string{
		"productType",
		"collection",
		"color",
		"size",
		"price",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling StaticFilterType type.
func (o *StaticFilterType) UnmarshalJSON(bytes []byte) error {
	var staticFilterType string
	err := json.Unmarshal(bytes, &staticFilterType)
	if err != nil {
		return err
	}
	switch staticFilterType {
	case ProductType.String():
		*o = ProductType
	case Collection.String():
		*o = Collection
	case Color.String():
		*o = Color
	case Size.String():
		*o = Size
	case Price.String():
		*o = Price
	default:
		return fmt.Errorf("invalid static filter type encountered: %v", staticFilterType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling SortBy type.
func (o StaticFilterType) MarshalJSON() ([]byte, error) {
	var staticFilterType string
	switch o {
	case ProductType:
		staticFilterType = ProductType.String()
	case Collection:
		staticFilterType = Collection.String()
	case Color:
		staticFilterType = Color.String()
	case Size:
		staticFilterType = Size.String()
	case Price:
		staticFilterType = Price.String()
	default:
		return nil, fmt.Errorf("invalid static filter type encountered: %v", o)
	}
	return json.Marshal(staticFilterType)
}

type ThemeColors struct {
	PrimaryColor     string `json:"primaryColor"`
	PrimaryTextColor string `json:"primaryTextColor"`
	TextColor        string `json:"textColor"`
	TitleColor       string `json:"titleColor"`
}

type ThemeTypography struct {
	FontFamily string `json:"fontFamily"`
}

type RSThemeConfig struct {
	Colors     ThemeColors     `json:"colors"`
	Typography ThemeTypography `json:"typography"`
}

type ThemeSettings struct {
	Type      string                 `json:"type"`
	RSConfig  RSThemeConfig          `json:"rsConfig"`
	CustomCSS string                 `json:"customCss"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

type GlobalSettings struct {
	Currency            string                 `json:"currency"`
	ShowSelectedFilters bool                   `json:"showSelectedFilters"`
	Meta                map[string]interface{} `json:"meta,omitempty"`
}

type ResultSettings struct {
	Fields struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Price       string `json:"price"`
		Image       string `json:"image"`
		Handle      string `json:"handle"`
	} `json:"fields"`
	CustomMessages struct {
		ResultStats string `json:"resultStats"`
		NoResults   string `json:"noResults"`
	} `json:"customMessages"`
	RSConfig           map[string]interface{} `json:"rsConfig"`
	ViewSwitcher       bool                   `json:"viewSwitcher"`
	Layout             string                 `json:"layout"`
	Meta               map[string]interface{} `json:"meta,omitempty"`
	ResultHighlight    bool                   `json:"resultHighlight,omitempty"`
	MapLayout          string                 `json:"mapLayout,omitempty"`
	MapComponent       string                 `json:"mapComponent,omitempty"`
	DefaultZoom        int32                  `json:"defaultZoom,omitempty"`
	ShowSearchAsMove   bool                   `json:"showSearchAsMove"`
	ShowMarkerClusters bool                   `json:"showMarkerClusters"`
	MapsAPIKey         string                 `json:"mapsAPIkey,omitempty"`
	LocationDataField  string                 `json:"locationDataField,omitempty"`
}

type SearchSettings struct {
	// To override the fields to display suggestions
	// We don't support this in UI right now
	Fields struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Price       string `json:"price"`
		Image       string `json:"image"`
		Handle      string `json:"handle"`
	} `json:"fields"`
	CustomMessages struct {
		NoResults string `json:"noResults"`
	} `json:"customMessages"`
	RSConfig     map[string]interface{} `json:"rsConfig"`
	SearchButton struct {
		Icon string `json:"icon"`
		Text string `json:"text"`
	} `json:"searchButton"`
	RedirectURLIcon string                 `json:"redirectUrlIcon,omitempty"`
	RedirectURLText string                 `json:"redirectUrlText,omitempty"`
	Meta            map[string]interface{} `json:"meta,omitempty"`
}

type FacetSettings struct {
	StaticFacets []struct {
		Name           *StaticFilterType `json:"name"`
		Enabled        bool              `json:"enabled"`
		IsCollapsible  bool              `json:"isCollapsible"`
		CustomMessages struct {
			Loading   string `json:"loading"`
			NoResults string `json:"noResults"`
		} `json:"customMessages"`
		RSConfig map[string]interface{} `json:"rsConfig"`
	} `json:"staticFacets"`
	DynamicFacets []struct {
		Enabled        bool `json:"enabled"`
		CustomMessages struct {
			Loading   string `json:"loading"`
			NoResults string `json:"noResults"`
		} `json:"customMessages"`
		RSConfig map[string]interface{} `json:"rsConfig"`
	} `json:"dynamicFacets"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

type RecommendationSettings struct {
	CtaTitle        string `json:"ctaTitle"`
	CtaAction       string `json:"ctaAction"`
	Recommendations []struct {
		ID              string    `json:"id"`
		Title           string    `json:"title"`
		Type            string    `json:"type"`
		MaxProducts     int64     `json:"maxProducts"`
		ProductsPageUrl *string   `json:"productsPageUrl,omitempty"`
		DataField       *string   `json:"dataField,omitempty"`
		DocIds          *[]string `json:"docIds,omitempty"`
	} `json:"recommendations"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

type ExportSettings struct {
	ExportType  *ExportType            `json:"type"`
	OpenAsPage  bool                   `json:"openAsPage"`
	Credentials string                 `json:"credentials"`
	ExportAs    *ExportAs              `json:"exportAs"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

type FusionSettings struct {
	App           string                 `json:"app"`
	Profile       string                 `json:"profile"`
	SearchProfile string                 `json:"searchProfile"`
	Meta          map[string]interface{} `json:"meta,omitempty"`
}

type SearchPreference struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Pipeline    string         `json:"pipeline"`
	UpdatedAt   *int64         `json:"updated_at,omitempty"`
	CreatedAt   *int64         `json:"created_at,omitempty"`
	Type        PreferenceType `json:"type"`
	// New fields to store settings as string
	FusionSettings         *string `json:"fusionSettings_string,omitempty"`
	DeploySettings         *string `json:"deploySettings_string,omitempty"`
	AuthenticationSettings *string `json:"authenticationSettings_string,omitempty"`
	PageSettings           *string `json:"pageSettings_string,omitempty"`
	ComponentSettings      *string `json:"componentSettings_string,omitempty"`
	ThemeSettings          *string `json:"themeSettings_string,omitempty"`
	GlobalSettings         *string `json:"globalSettings_string,omitempty"`
	ResultSettings         *string `json:"resultSettings_string,omitempty"`
	ExportSettings         *string `json:"exportSettings_string,omitempty"`
	SearchSettings         *string `json:"searchSettings_string,omitempty"`
	FacetSettings          *string `json:"facetSettings_string,omitempty"`
	SyncSettings           *string `json:"syncSettings_string,omitempty"`
	// Older fields
	FusionSettingsStruct         *FusionSettings         `json:"fusionSettings"`
	DeploySettingsStruct         *map[string]interface{} `json:"deploySettings,omitempty"`
	AuthenticationSettingsStruct *map[string]interface{} `json:"authenticationSettings,omitempty"`
	PageSettingsStruct           *map[string]interface{} `json:"pageSettings,omitempty"`
	ComponentSettingsStruct      *map[string]interface{} `json:"componentSettings,omitempty"`
	ThemeSettingsStruct          *ThemeSettings          `json:"themeSettings,omitempty"`
	GlobalSettingsStruct         *GlobalSettings         `json:"globalSettings,omitempty"`
	ResultSettingsStruct         *ResultSettings         `json:"resultSettings,omitempty"`
	ExportSettingsStruct         *ExportSettings         `json:"exportSettings,omitempty"`
	SearchSettingsStruct         *SearchSettings         `json:"searchSettings,omitempty"`
	FacetSettingsStruct          *FacetSettings          `json:"facetSettings,omitempty"`
	SyncSettingsStruct           *map[string]interface{} `json:"syncSettings"`
	// To track if using new fields
	IsUsingStringFields bool `json:"isUsingStringFields"`
}

func MapToString(m interface{}) *string {
	if m != nil {
		byteMap, err := json.Marshal(m)
		if err != nil {
			log.Errorln(logTag, ":", err)
		} else {
			mapString := string(byteMap)
			return &mapString
		}
	}
	return nil
}

type SearchPreferenceRequest struct {
	ID                     string                 `json:"id"`
	Name                   string                 `json:"name"`
	Description            string                 `json:"description"`
	Pipeline               string                 `json:"pipeline"`
	Type                   PreferenceType         `json:"type"`
	DeploySettings         map[string]interface{} `json:"deploySettings"`
	AuthenticationSettings map[string]interface{} `json:"authenticationSettings,omitempty"`
	ThemeSettings          *ThemeSettings         `json:"themeSettings,omitempty"`
	FusionSettings         *FusionSettings        `json:"fusionSettings,omitempty"`
	PageSettings           map[string]interface{} `json:"pageSettings,omitempty"`
	ComponentSettings      map[string]interface{} `json:"componentSettings,omitempty"`
	GlobalSettings         *GlobalSettings        `json:"globalSettings,omitempty"`
	ResultSettings         *ResultSettings        `json:"resultSettings,omitempty"`
	ExportSettings         *ExportSettings        `json:"exportSettings,omitempty"`
	SearchSettings         *SearchSettings        `json:"searchSettings,omitempty"`
	FacetSettings          *FacetSettings         `json:"facetSettings,omitempty"`
	SyncSettings           map[string]interface{} `json:"syncSettings"`
}

type RecommendationPreference struct {
	ID                     string                  `json:"id"`
	Name                   string                  `json:"name"`
	Description            string                  `json:"description"`
	Pipeline               string                  `json:"pipeline"`
	Type                   PreferenceType          `json:"type"`
	DeploySettings         map[string]interface{}  `json:"deploySettings"`
	ThemeSettings          *ThemeSettings          `json:"themeSettings,omitempty"`
	GlobalSettings         *GlobalSettings         `json:"globalSettings,omitempty"`
	ResultSettings         *ResultSettings         `json:"resultSettings,omitempty"`
	ExportSettings         *ExportSettings         `json:"exportSettings,omitempty"`
	RecommendationSettings *RecommendationSettings `json:"recommendationSettings,omitempty"`
	UpdatedAt              *int64                  `json:"updated_at,omitempty"`
	CreatedAt              *int64                  `json:"created_at,omitempty"`
}

type RecommendationPreferenceRequest struct {
	ID                     string                  `json:"id"`
	Name                   string                  `json:"name"`
	Description            string                  `json:"description"`
	Pipeline               string                  `json:"pipeline"`
	Type                   PreferenceType          `json:"type"`
	DeploySettings         map[string]interface{}  `json:"deploySettings"`
	ThemeSettings          *ThemeSettings          `json:"themeSettings,omitempty"`
	GlobalSettings         *GlobalSettings         `json:"globalSettings,omitempty"`
	ResultSettings         *ResultSettings         `json:"resultSettings,omitempty"`
	ExportSettings         *ExportSettings         `json:"exportSettings,omitempty"`
	RecommendationSettings *RecommendationSettings `json:"recommendationSettings,omitempty"`
}
type SearchPreferences struct {
	ThemeSettings     *ThemeSettings         `json:"themeSettings,omitempty"`
	FusionSettings    *FusionSettings        `json:"fusionSettings,omitempty"`
	PageSettings      map[string]interface{} `json:"pageSettings,omitempty"`
	ComponentSettings map[string]interface{} `json:"componentSettings,omitempty"`
	GlobalSettings    *GlobalSettings        `json:"globalSettings,omitempty"`
	ResultSettings    *ResultSettings        `json:"resultSettings,omitempty"`
	ExportSettings    *ExportSettings        `json:"exportSettings,omitempty"`
	SearchSettings    *SearchSettings        `json:"searchSettings,omitempty"`
	FacetSettings     *FacetSettings         `json:"facetSettings,omitempty"`
	SyncSettings      map[string]interface{} `json:"syncSettings"`
}

type RecommendationsPreferences struct {
	ThemeSettings          *ThemeSettings          `json:"themeSettings,omitempty"`
	GlobalSettings         *GlobalSettings         `json:"globalSettings,omitempty"`
	ResultSettings         *ResultSettings         `json:"resultSettings,omitempty"`
	ExportSettings         *ExportSettings         `json:"exportSettings,omitempty"`
	RecommendationSettings *RecommendationSettings `json:"recommendationSettings,omitempty"`
}

type PutObjectToBucketRequest struct {
	MetaData map[string]string `json:"metadata"`
	Content  map[string]string `json:"content"`
}

func performDeployRequestToACCAPI(uiBuilderID string, method string, path string, urlValues url.Values, requestBody io.Reader) (*http.Response, error) {
	appbaseID, err := util.GetArcID()
	if err != nil {
		return nil, err
	}
	// TODO: Use ACCAPI
	req, err := http.NewRequest(method, util.ACCAPI+"uibuilder/deploy/"+appbaseID+"/"+uiBuilderID+path, requestBody)
	if err != nil {
		return nil, err
	}
	// Apply query params
	q := urlValues
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := util.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func performEnvRequestToACCAPI(uiBuilderID string, method string, path string, urlValues url.Values, requestBody io.Reader) (*http.Response, error) {
	appbaseID, err := util.GetArcID()
	if err != nil {
		return nil, err
	}
	// TODO: Use ACCAPI
	req, err := http.NewRequest(method, util.ACCAPI+"uibuilder/env/"+appbaseID+"/"+uiBuilderID+path, requestBody)
	if err != nil {
		return nil, err
	}
	// Apply query params
	q := urlValues
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := util.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func performDomainRequestToACCAPI(uiBuilderID string, method string, path string, urlValues url.Values, requestBody io.Reader) (*http.Response, error) {
	appbaseID, err := util.GetArcID()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, util.ACCAPI+"uibuilder/domain/"+appbaseID+"/"+uiBuilderID+path, requestBody)
	if err != nil {
		return nil, err
	}
	// Apply query params
	q := urlValues
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := util.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func performHTTPRequestToACCAPI(method string, path string, urlValues url.Values, requestBody io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, util.ACCAPI+path, requestBody)
	if err != nil {
		return nil, err
	}
	// Apply query params
	q := urlValues
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := util.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func setAuthPreferenceDo(e *UIBuilder, ctx context.Context, reqBody map[string]interface{}) ([]byte, error) {
	updatedAt := time.Now().Unix()
	createdAt := updatedAt
	// Use created_at from existing auth preference
	preference, err := e.es.getAuthPreference(context.Background())
	if err == nil {
		if !keyExists(preference, "created_at") {
			created, ok := preference["created_at"].(float64)
			if ok {
				createdAt = int64(created)
			}
		}
	}
	// handle system maintained keys
	preference["updated_at"] = updatedAt
	preference["created_at"] = createdAt
	// override the values in req body with the existing map
	for key := range reqBody {
		if reqBody[key] == "" {
			delete(preference, key)
		} else {
			preference[key] = reqBody[key]
		}
	}
	err1 := e.es.setAuthPreference(ctx, preference)
	if err1 != nil {
		log.Errorln(logTag, ":", err1)
		return nil, err1
	}
	response, _ := json.Marshal(reqBody)
	return response, nil
}

func keyExists(decoded map[string]interface{}, key string) bool {
	val, ok := decoded[key]
	return ok && val != nil
}
