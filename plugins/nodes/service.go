package nodes

import "context"

type nodeService interface {
	pingES(ctx context.Context, machineID string) error
}
