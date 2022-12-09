package util

import (
	"context"
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
	HostedArcBasicV2
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
	Sandbox2020
	Hobby2020
	Starter2020
	ProductionFirst2019
	ProductionSecond2019
	ProductionThird2019
	ProductionFourth2019
	Starter2021
	ProductionFirst2021
	ProductionSecond2021
	ProductionThird2021
	HostedArcStandard2021
	HostedArcEnterprise2021
)

// String is the implementation of Stringer interface that returns the string representation of Plan type.
func (o Plan) String() string {
	return [...]string{
		"arc-basic",
		"arc-standard",
		"arc-enterprise",
		"hosted-arc-basic",
		"hosted-arc-basic-v2",
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
		"2020-sandbox",
		"2020-hobby",
		"2020-starter",
		"2019-production-1",
		"2019-production-2",
		"2019-production-3",
		"2019-production-4",
		"2021-starter",
		"2021-production-1",
		"2021-production-2",
		"2021-production-3",
		"2021-hosted-arc-standard",
		"2021-hosted-arc-enterprise",
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
	case HostedArcBasicV2.String():
		*o = HostedArcBasicV2
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
	case Sandbox2020.String():
		*o = Sandbox2020
	case Hobby2020.String():
		*o = Hobby2020
	case Starter2020.String():
		*o = Starter2020
	case ProductionFirst2019.String():
		*o = ProductionFirst2019
	case ProductionSecond2019.String():
		*o = ProductionSecond2019
	case ProductionThird2019.String():
		*o = ProductionThird2019
	case ProductionFourth2019.String():
		*o = ProductionFourth2019
	case Starter2021.String():
		*o = Starter2021
	case ProductionFirst2021.String():
		*o = ProductionFirst2021
	case ProductionSecond2021.String():
		*o = ProductionSecond2021
	case ProductionThird2021.String():
		*o = ProductionThird2021
	case HostedArcStandard2021.String():
		*o = HostedArcStandard2021
	case HostedArcEnterprise2021.String():
		*o = HostedArcEnterprise2021
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
	case HostedArcBasicV2:
		plan = HostedArcBasicV2.String()
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
	case Sandbox2020:
		plan = Sandbox2020.String()
	case Hobby2020:
		plan = Hobby2020.String()
	case Starter2020:
		plan = Starter2020.String()
	case ProductionFirst2019:
		plan = ProductionFirst2019.String()
	case ProductionSecond2019:
		plan = ProductionSecond2019.String()
	case ProductionThird2019:
		plan = ProductionThird2019.String()
	case ProductionFourth2019:
		plan = ProductionFourth2019.String()
	case Starter2021:
		plan = Starter2021.String()
	case ProductionFirst2021:
		plan = ProductionFirst2021.String()
	case ProductionSecond2021:
		plan = ProductionSecond2021.String()
	case ProductionThird2021:
		plan = ProductionThird2021.String()
	case HostedArcStandard2021:
		plan = HostedArcStandard2021.String()
	case HostedArcEnterprise2021:
		plan = HostedArcEnterprise2021.String()
	default:
		return nil, fmt.Errorf("invalid plan encountered: %v", o)
	}
	return json.Marshal(plan)
}

// ValidatePlans validates the user's plan against the valid plans
func ValidatePlans(ctx context.Context, validPlans []Plan, byPassValidation bool) bool {
	if byPassValidation {
		return true
	}
	if GetTier(ctx) == nil {
		return false
	}
	for _, validPlan := range validPlans {
		if GetTier(ctx).String() == validPlan.String() {
			return true
		}
	}
	return false
}

// IsProductionPlan validates if the user's plan is a production plan
func IsProductionPlan(ctx context.Context) bool {
	tier := GetTier(ctx)
	if tier == nil {
		return false
	}
	switch tier.String() {
	case ArcEnterprise.String(),
		HostedArcEnterprise.String(),
		ProductionFirst2019.String(),
		ProductionSecond2019.String(),
		ProductionThird2019.String(),
		ProductionFourth2019.String(),
		ProductionFirst2021.String(),
		ProductionSecond2021.String(),
		ProductionThird2021.String(),
		HostedArcEnterprise2021.String():
		return true
	default:
		return false
	}
}
