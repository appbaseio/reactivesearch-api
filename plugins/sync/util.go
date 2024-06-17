package sync

import "github.com/appbaseio-confidential/reactivesearch/util"

type SyncPreferences struct {
	Interval *int `json:"interval,omitempty"`
}

func setPreferences(preferences SyncPreferences) error {
	if preferences.Interval != nil {
		return util.SetSyncInterval(*preferences.Interval)
	}
	return nil
}
