package es

import "github.com/appbaseio-confidential/arc/arc"

func init() {
	arc.RegisterPlugin(arc.NewPlugin("es", arc.NoSetup(), getESRoutes()))
}
