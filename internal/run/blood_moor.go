package run

import (
	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
)

type BloodMoor struct {
	ctx *context.Status
}

func NewBloodMoor() *BloodMoor {
	return &BloodMoor{
		ctx: context.Get(),
	}
}

func (a BloodMoor) Name() string {
	return string(config.BloodMoorRun)
}

func (a BloodMoor) CheckConditions(parameters *RunParameters) SequencerResult {
	// You can add any checks here if needed
	return SequencerOk
}

func (a BloodMoor) Run(parameters *RunParameters) error {

	// Define a defaut filter
	monsterFilter := data.MonsterAnyFilter()

	// Use the waypoint
	err := action.WayPoint(area.RogueEncampment)
	if err != nil {
		return err
	}

	// Move to the BurialGrounds
	if err = action.MoveToArea(area.BloodMoor); err != nil {
		return err
	}

	// Clear the Blood Moor area
	return action.ClearCurrentLevel(true, monsterFilter)
}
