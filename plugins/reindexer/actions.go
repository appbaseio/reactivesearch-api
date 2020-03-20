package reindexer

type Action int

const (
	Mappings Action = iota
	Settings
	Data
)

func (o Action) String() string {
	return [...]string{"mappings", "settings", "data"}[o]
}
