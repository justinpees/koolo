package run

import (
	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
)

type Level78Zones struct {
	ctx *context.Status
}

func NewLevel78Zones() *Level78Zones {
	return &Level78Zones{
		ctx: context.Get(),
	}
}

func (a Level78Zones) Name() string {
	return string(config.Level78ZonesRun)
}

func (a Level78Zones) CheckConditions(parameters *RunParameters) SequencerResult {
	// You can add any checks here if needed
	return SequencerOk
}

func (a Level78Zones) Run(parameters *RunParameters) error {

	monsterFilter := data.MonsterAnyFilter()

	// =====================
	// ACT 2 – HAREM / PALACE
	// =====================

	if err := action.WayPoint(area.LutGholein); err != nil {
		return err
	}

	if err := action.MoveToArea(area.HaremLevel1); err != nil {
		return err
	}

	if err := action.MoveToArea(area.HaremLevel2); err != nil {
		return err
	}

	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}

	if err := action.MoveToArea(area.PalaceCellarLevel1); err != nil {
		return err
	}

	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}

	if err := action.MoveToArea(area.PalaceCellarLevel2); err != nil {
		return err
	}

	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}

	if err := action.MoveToArea(area.PalaceCellarLevel3); err != nil {
		return err
	}

	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}

	if err := action.ReturnTown(); err != nil {
		return err
	}

	// =====================
	// ACT 1 – CAVE
	// =====================

	if err := action.WayPoint(area.ColdPlains); err != nil {
		return err
	}

	if err := action.MoveToArea(area.CaveLevel1); err != nil {
		return err
	}

	if err := action.MoveToArea(area.CaveLevel2); err != nil {
		return err
	}

	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}

	if err := action.ReturnTown(); err != nil {
		return err
	}

	// =====================
	// ACT 1 – TOWER
	// =====================

	if err := action.WayPoint(area.BlackMarsh); err != nil {
		return err
	}

	if err := action.MoveToArea(area.ForgottenTower); err != nil {
		return err
	}

	if err := action.MoveToArea(area.TowerCellarLevel1); err != nil {
		return err
	}

	if err := action.MoveToArea(area.TowerCellarLevel2); err != nil {
		return err
	}

	if err := action.MoveToArea(area.TowerCellarLevel3); err != nil {
		return err
	}

	if err := action.MoveToArea(area.TowerCellarLevel4); err != nil {
		return err
	}

	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}

	if err := action.MoveToArea(area.TowerCellarLevel5); err != nil {
		return err
	}

	/* if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	} */
	// if you only want to kill countess
	err := action.MoveTo(func() (data.Position, bool) {
		areaData := a.ctx.Data.Areas[area.TowerCellarLevel5]
		countessNPC, found := areaData.NPCs.FindOne(740)
		if !found {
			return data.Position{}, false
		}

		return countessNPC.Positions[0], true
	})
	if err != nil {
		return err
	}

	// Kill Countess
	if err := a.ctx.Char.KillCountess(); err != nil {
		return err
	}

	action.ItemPickup(30)

	return nil
}
