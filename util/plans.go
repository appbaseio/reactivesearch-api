package util

import (
	"encoding/json"
	"fmt"
)

// An enum having a list of valid plans
type Plan int

const (
	ArcBasic Plan = iota
	ArcStandard
	ArcEnterprise
	HostedArcBasic
	HostedArcStandard
	HostedArcEnterprise
	Sandbox
	Hobby
	Starter
	ProductionFirst
	ProductionSecond
	ProductionThird
	Sandbox2019
	Hobby2019
	Starter2019
	ProductionFirst2019
	ProductionSecond2019
	ProductionThird2019
)

// String is the implementation of Stringer interface that returns the string representation of Plan type.
func (o Plan) String() string {
	return [...]string{
		"arc-basic",
		"arc-standard",
		"arc-enterprise",
		"hosted-arc-basic",
		"hosted-arc-standard",
		"hosted-arc-enterprise",
		"sandbox",
		"hobby",
		"starter",
		"production-1",
		"production-2",
		"production-3",
		"2019-sandbox",
		"2019-hobby",
		"2019-starter",
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
	case ArcStandard.String():
		*o = ArcStandard
	case ArcEnterprise.String():
		*o = ArcEnterprise
	case HostedArcBasic.String():
		*o = HostedArcBasic
	case HostedArcStandard.String():
		*o = HostedArcStandard
	case HostedArcEnterprise.String():
		*o = HostedArcEnterprise
	case Sandbox.String():
		*o = Sandbox
	case Hobby.String():
		*o = Hobby
	case Starter.String():
		*o = Starter
	case ProductionFirst.String():
		*o = ProductionFirst
	case ProductionSecond.String():
		*o = ProductionSecond
	case ProductionThird.String():
		*o = ProductionThird
	case Sandbox2019.String():
		*o = Sandbox2019
	case Hobby2019.String():
		*o = Hobby2019
	case Starter2019.String():
		*o = Starter2019
	case ProductionFirst2019.String():
		*o = ProductionFirst2019
	case ProductionSecond2019.String():
		*o = ProductionSecond2019
	case ProductionThird2019.String():
		*o = ProductionThird2019
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
	case ArcStandard:
		plan = ArcStandard.String()
	case ArcEnterprise:
		plan = ArcEnterprise.String()
	case HostedArcBasic:
		plan = HostedArcBasic.String()
	case HostedArcStandard:
		plan = HostedArcStandard.String()
	case HostedArcEnterprise:
		plan = HostedArcEnterprise.String()
	case Sandbox:
		plan = Sandbox.String()
	case Hobby:
		plan = Hobby.String()
	case Starter:
		plan = Starter.String()
	case ProductionFirst:
		plan = ProductionFirst.String()
	case ProductionSecond:
		plan = ProductionSecond.String()
	case ProductionThird:
		plan = ProductionThird.String()
	case Sandbox2019:
		plan = Sandbox2019.String()
	case Hobby2019:
		plan = Hobby2019.String()
	case Starter2019:
		plan = Starter2019.String()
	case ProductionFirst2019:
		plan = ProductionFirst2019.String()
	case ProductionSecond2019:
		plan = ProductionSecond2019.String()
	case ProductionThird2019:
		plan = ProductionThird2019.String()
	default:
		return nil, fmt.Errorf("invalid plan encountered: %v", o)
	}
	return json.Marshal(plan)
}

// ValidatePlans validates the user's plan against the valid plans
func ValidatePlans(validPlans []Plan, byPassValidation bool) bool {
	if byPassValidation {
		return true
	}
	if Tier == nil {
		return false
	}
	for _, validPlan := range validPlans {
		if Tier.String() == validPlan.String() {
			return true
		}
	}
	return false
}
