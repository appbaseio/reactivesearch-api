package util

import (
	"encoding/json"
	"fmt"
)

// An enum having a list of valid plans
type Plan int

const (
	ArcBasic Plan = iota
	ArcEnterprise
	HostedArcEnterprise
	ProductionFirst
	ProductionSecond
	ProductionThird
)

// String is the implementation of Stringer interface that returns the string representation of Plan type.
func (o Plan) String() string {
	return [...]string{
		"arc-basic",
		"arc-enterprise",
		"hosted-arc-enterprise",
		"2019-production-1",
		"2019-production-2",
		"2019-production-3",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling Plan type.
func (o *Plan) UnmarshalJSON(bytes []byte) error {
	var plan string
	err := json.Unmarshal(bytes, &plan)
	if err != nil {
		return err
	}
	switch plan {
	case ArcBasic.String():
		*o = ArcBasic
	case ArcEnterprise.String():
		*o = ArcEnterprise
	case HostedArcEnterprise.String():
		*o = HostedArcEnterprise
	case ProductionFirst.String():
		*o = ProductionFirst
	case ProductionSecond.String():
		*o = ProductionSecond
	case ProductionThird.String():
		*o = ProductionThird
	default:
		return fmt.Errorf("invalid plan encountered: %v", plan)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling Plan type.
func (o Plan) MarshalJSON() ([]byte, error) {
	var plan string
	switch o {
	case ArcBasic:
		plan = ArcBasic.String()
	case ArcEnterprise:
		plan = ArcEnterprise.String()
	case HostedArcEnterprise:
		plan = HostedArcEnterprise.String()
	case ProductionFirst:
		plan = ProductionFirst.String()
	case ProductionSecond:
		plan = ProductionSecond.String()
	case ProductionThird:
		plan = ProductionThird.String()
	default:
		return nil, fmt.Errorf("invalid plan encountered: %v", o)
	}
	return json.Marshal(plan)
}

// A util function to validate the user's plan against the restricted plans
func ValidatedPlans(restrictedPlans []Plan) bool {
	for _, restrictedPlan := range restrictedPlans {
		if Billing == "true" && Tier.String() == restrictedPlan.String() {
			return false
		}
	}
	return true
}
