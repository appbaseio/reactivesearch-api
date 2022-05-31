package nodes

import "context"

type nodeService interface {
	pingES(ctx context.Context, machineID string) error
	deleteOlderRecords(ctx context.Context) error
	activeNodesInTenMins(ctx context.Context) (int64, error)
}
