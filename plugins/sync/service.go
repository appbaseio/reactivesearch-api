package sync

import (
	"context"

	es7 "github.com/olivere/elastic/v7"
)

type syncService interface {
	saveSyncPreferences(ctx context.Context, record SyncPreferences) (*es7.IndexResponse, error)
	getSyncPreferences(ctx context.Context) (SyncPreferences, error)
}
