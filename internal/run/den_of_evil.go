package run

import (
	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
)

type DenOfEvil struct {
	ctx *context.Status
}

func NewDenOfEvil() *DenOfEvil {
	return &DenOfEvil{
		ctx: context.Get(),
	}
}

func (a DenOfEvil) Name() string {
	return string(config.DenOfEvilRun)
}

func (a DenOfEvil) CheckConditions(parameters *RunParameters) SequencerResult {
	// You can add any checks here if needed
	return SequencerOk
}

func (a DenOfEvil) Run(parameters *RunParameters) error {

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

	action.MoveToArea(area.DenOfEvil)

	// Clear the Den of Evil area
	return action.ClearCurrentLevel(true, monsterFilter)
}
