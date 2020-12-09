package user

import (
	"encoding/json"
	"fmt"

	"github.com/appbaseio/arc/model/category"
)

type UserAction int

const (
	Develop UserAction = iota
	Analytics
	CuratedInsights
	SearchRelevancy
	AccessControl
	UserManagement
	Billing
	DowntimeAlerts
	UIIntegrations
)

// String is the implementation of Stringer interface that returns the string representation of UserAction type.
func (o UserAction) String() string {
	return [...]string{
		"develop",
		"analytics",
		"curated-insights",
		"search-relevancy",
		"access-control",
		"user-management",
		"billing",
		"downtime-alerts",
		"uiintegrations",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling UserAction type.
func (o *UserAction) UnmarshalJSON(bytes []byte) error {
	var userAction string
	err := json.Unmarshal(bytes, &userAction)
	if err != nil {
		return err
	}
	switch userAction {
	case Develop.String():
		*o = Develop
	case Analytics.String():
		*o = Analytics
	case CuratedInsights.String():
		*o = CuratedInsights
	case SearchRelevancy.String():
		*o = SearchRelevancy
	case AccessControl.String():
		*o = AccessControl
	case UserManagement.String():
		*o = UserManagement
	case Billing.String():
		*o = Billing
	case DowntimeAlerts.String():
		*o = DowntimeAlerts
	case UIIntegrations.String():
		*o = UIIntegrations
	default:
		return fmt.Errorf("invalid user action encountered: %v", userAction)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling UserAction type.
func (o UserAction) MarshalJSON() ([]byte, error) {
	var userAction string
	switch o {
	case Develop:
		userAction = Develop.String()
	case Analytics:
		userAction = Analytics.String()
	case CuratedInsights:
		userAction = CuratedInsights.String()
	case SearchRelevancy:
		userAction = SearchRelevancy.String()
	case AccessControl:
		userAction = AccessControl.String()
	case UserManagement:
		userAction = UserManagement.String()
	case Billing:
		userAction = Billing.String()
	case DowntimeAlerts:
		userAction = DowntimeAlerts.String()
	case UIIntegrations:
		userAction = UIIntegrations.String()
	default:
		return nil, fmt.Errorf("invalid user action encountered: %v", o)
	}
	return json.Marshal(userAction)
}

var developCategories = []category.Category{
	category.Docs,
	category.Search,
	category.Indices,
	category.Cat,
	category.Clusters,
	category.Misc,
	category.Streams,
	category.Logs,
}

var ActionToCategories = map[UserAction][]category.Category{
	Develop: developCategories,
	Analytics: {
		category.Analytics,
		category.Logs,
		category.Cat,
	},
	CuratedInsights: {},
	SearchRelevancy: append([]category.Category{
		category.Rules,
		category.Templates,
		category.Suggestions,
		category.Functions,
		category.ReactiveSearch,
		category.SearchRelevancy,
		category.Synonyms,
		category.SearchGrader,
	}, developCategories...),
	AccessControl: {
		category.Auth,
		category.Permission,
	},
	UserManagement: {
		category.User,
	},
	Billing:        {},
	DowntimeAlerts: {},
	UIIntegrations: {
		category.Cat,
		category.UIIntegrations,
		category.Search,
		category.ReactiveSearch,
		category.SearchRelevancy,
	},
}
