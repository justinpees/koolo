package run

import (
	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
)

type CaveLevel1 struct {
	ctx *context.Status
}

func NewCaveLevel1() *CaveLevel1 {
	return &CaveLevel1{
		ctx: context.Get(),
	}
}

func (a CaveLevel1) Name() string {
	return string(config.CaveLevel1Run)
}

func (a CaveLevel1) CheckConditions(parameters *RunParameters) SequencerResult {
	// You can add any checks here if needed
	return SequencerOk
}

func (a CaveLevel1) Run(parameters *RunParameters) error {

	// Define a defaut filter
	monsterFilter := data.MonsterAnyFilter()

	// Use the waypoint
	err := action.WayPoint(area.ColdPlains)
	if err != nil {
		return err
	}

	// Move to the BurialGrounds
	if err = action.MoveToArea(area.CaveLevel1); err != nil {
		return err
	}

	// Clear the Blood Moor area
	return action.ClearCurrentLevel(true, monsterFilter)
}
