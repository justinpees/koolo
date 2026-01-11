package run

import (
	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
)

type ColdPlains struct {
	ctx *context.Status
}

func NewColdPlains() *ColdPlains {
	return &ColdPlains{
		ctx: context.Get(),
	}
}

func (a ColdPlains) Name() string {
	return string(config.ColdPlainsRun)
}

func (a ColdPlains) CheckConditions(parameters *RunParameters) SequencerResult {
	// You can add any checks here if needed
	return SequencerOk
}

func (a ColdPlains) Run(parameters *RunParameters) error {

	// Define a defaut filter
	monsterFilter := data.MonsterAnyFilter()

	// Use the waypoint
	err := action.WayPoint(area.ColdPlains)
	if err != nil {
		return err
	}

	// Clear the Blood Moor area
	return action.ClearCurrentLevel(true, monsterFilter)
}
