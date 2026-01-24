package run

import (
	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"

	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	/* "github.com/hectorgimenez/koolo/internal/utils"

	"log/slog" */)

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
	//filter to kill all
	monsterFilter := data.MonsterAnyFilter()

	championAndUniqueAndSuperUniqueFilter := func(m data.Monsters) []data.Monster {
		var filtered []data.Monster
		for _, mo := range m {
			if mo.Type == data.MonsterTypeChampion || mo.Type == data.MonsterTypeUnique || mo.Type == data.MonsterTypeSuperUnique {
				filtered = append(filtered, mo)
			}
		}
		return filtered
	}

	uniqueAndSuperUniqueFilter := func(m data.Monsters) []data.Monster {
		var filtered []data.Monster
		for _, mo := range m {
			if mo.Type == data.MonsterTypeChampion || mo.Type == data.MonsterTypeUnique || mo.Type == data.MonsterTypeSuperUnique {
				filtered = append(filtered, mo)
			}
		}
		return filtered
	}

	// =====================
	// ACT 2 – HAREM / PALACE
	// =====================

	//WAYPOINT TO LUTGHOLEIN
	if err := action.WayPoint(area.LutGholein); err != nil {
		return err
	}
	//GO IN TEMPLE
	if err := action.MoveToArea(area.HaremLevel1); err != nil {
		return err
	}
	//MOVE TO HAREM LEVEL 2
	if err := action.MoveToArea(area.HaremLevel2); err != nil {
		return err
	}
	//CLEAR ALL MONSTERS HAREM LEVEL 2
	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}
	//MOVE TO PALACE CELLAR LEVEL 1
	if err := action.MoveToArea(area.PalaceCellarLevel1); err != nil {
		return err
	}
	//CLEAR ALL MONSTERS PALACE CELLAR LEVEL 1
	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}
	//MOVE TO PALACE CELLAR LEVEL 2
	if err := action.MoveToArea(area.PalaceCellarLevel2); err != nil {
		return err
	}
	//CLEAR ALL MONSTERS IN PALACE CELLAR LEVEL 2
	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}
	//MOVE TO PALACE CELLAR LEVEL 3
	if err := action.MoveToArea(area.PalaceCellarLevel3); err != nil {
		return err
	}
	//CLEAR ALL MONSTERS IN PALACE CELLAR LEVEL 3
	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}
	//RETURN TO TOWN
	if err := action.ReturnTown(); err != nil {
		return err
	}
	//WAYPOINT TO DRY HILLS
	if err := action.WayPoint(area.DryHills); err != nil {
		return err
	}
	//CLEAR ONLY CHAMPION AND UNIQUE MONSTERS IN DRY HILLS
	if err := action.ClearCurrentLevel(false, championAndUniqueAndSuperUniqueFilter); err != nil {
		return err
	}
	//RETURN TO TOWN
	if err := action.ReturnTown(); err != nil {
		return err
	}

	// =====================
	// ACT 1 – CAVE
	// =====================

	//WAYPOINT TO COLD PLAINS
	if err := action.WayPoint(area.ColdPlains); err != nil {
		return err
	}
	//MOVE TO CAVE LEVEL 1
	if err := action.MoveToArea(area.CaveLevel1); err != nil {
		return err
	}
	//MOVE TO CAVE LEVEL 2
	if err := action.MoveToArea(area.CaveLevel2); err != nil {
		return err
	}
	//CLEAR ALL MONSTERS IN CAVE LEVEL 2
	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}
	//RETURN TO TOWN
	if err := action.ReturnTown(); err != nil {
		return err
	}

	// =====================
	// ACT 1 – TOWER
	// =====================

	//WAYPOINT TO BLACK MARSH
	if err := action.WayPoint(area.BlackMarsh); err != nil {
		return err
	}
	//MOVE TO FORGOTTEN TOWER
	if err := action.MoveToArea(area.ForgottenTower); err != nil {
		return err
	}
	//MOVE TO TOWER CELLAR LEVEL 1
	if err := action.MoveToArea(area.TowerCellarLevel1); err != nil {
		return err
	}
	//MOVE TO TOWER CELLAR LEVEL 2
	if err := action.MoveToArea(area.TowerCellarLevel2); err != nil {
		return err
	}
	//CLEAR ONLY CHAMPION AND UNIQUE MONSTERS IN TOWER CELLAR LEVEL 2
	if err := action.ClearCurrentLevel(false, championAndUniqueAndSuperUniqueFilter); err != nil {
		return err
	}
	//MOVE TO TOWER CELLAR LEVEL 3
	if err := action.MoveToArea(area.TowerCellarLevel3); err != nil {
		return err
	}
	//CLEAR ONLY CHAMPION AND UNIQUE MONSTERS IN TOWER CELLAR LEVEL 3
	if err := action.ClearCurrentLevel(false, championAndUniqueAndSuperUniqueFilter); err != nil {
		return err
	}
	//MOVE TO TOWER CELLAR LEVEL 4
	if err := action.MoveToArea(area.TowerCellarLevel4); err != nil {
		return err
	}
	//CLEAR ALL MONSTER IN TOWER CELLAR LEVEL 4
	if err := action.ClearCurrentLevel(true, monsterFilter); err != nil {
		return err
	}
	//MOVE TO TOWER CELLAR LEVEL 5
	if err := action.MoveToArea(area.TowerCellarLevel5); err != nil {
		return err
	}

	//CLEAR ALL MONSTER IN TOWER CELLAR LEVEL 5
	if err := action.ClearCurrentLevel(true, championAndUniqueAndSuperUniqueFilter); err != nil {
		return err
	}

	//RETURN TO TOWN
	if err := action.ReturnTown(); err != nil {
		return err
	}

	//WAYPOINT TO CATACOMBS LEVEL 2
	if err := action.WayPoint(area.CatacombsLevel2); err != nil {
		return err
	}

	//CLEAR ALL MONSTER IN CATACOMBS LEVEL 2
	if err := action.ClearCurrentLevel(false, uniqueAndSuperUniqueFilter); err != nil {
		return err
	}

	//MOVE TO CATACOMBS LEVEL 3
	if err := action.MoveToArea(area.CatacombsLevel3); err != nil {
		return err
	}

	//CLEAR ALL MONSTER IN CATACOMBS LEVEL 3
	if err := action.ClearCurrentLevel(false, uniqueAndSuperUniqueFilter); err != nil {
		return err
	}

	//MOVE TO CATACOMBS LEVEL 4
	if err := action.MoveToArea(area.CatacombsLevel4); err != nil {
		return err
	}

	//CLEAR ALL MONSTER IN CATACOMBS LEVEL 4
	if err := action.ClearCurrentLevel(false, uniqueAndSuperUniqueFilter); err != nil {
		return err
	}
	/* //MOVE TO COUNTESS
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

	// KILL COUNTESS
	if err := a.ctx.Char.KillCountess(); err != nil {
		return err
	}

	action.ItemPickup(30)

	// OPEN ALL CHESTS IN TOWER CELLAR LEVEL 5
	if err := OpenAllChestsInArea(area.TowerCellarLevel5); err != nil {
		a.ctx.Logger.Warn("Failed opening all chests", slog.Any("error", err))
	} */

	return nil
}

// OpenAllChestsInArea will systematically walk through all rooms in the specified area
/* func OpenAllChestsInArea(areaID area.ID) error {
	ctx := context.Get()

	for {
		anyOpened := false

		// Refresh area data each pass
		areaData, exists := ctx.Data.Areas[areaID]
		if !exists {
			ctx.Logger.Warn("Area data not found", slog.Int("areaID", int(areaID)))
			return nil
		}

		for _, obj := range areaData.Objects {
			chest := obj // capture for closure
			if !chest.Selectable || !(chest.IsChest() || chest.IsSuperChest()) {
				continue
			}

			ctx.Logger.Debug("Found chest, attempting to move", slog.Any("Name", chest.Desc().Name),
				slog.Any("ID", chest.ID), slog.Any("Pos", chest.Position))

			// Move to chest
			err := action.MoveTo(func() (data.Position, bool) {
				return chest.Position, true
			})
			if err != nil {
				ctx.Logger.Warn("Failed moving to chest", slog.Any("pos", chest.Position), slog.Any("error", err))
				continue
			}

			// Interact with chest
			err = action.InteractObject(chest, func() bool {
				freshChest, _ := ctx.Data.Objects.FindByID(chest.ID)
				return !freshChest.Selectable
			})
			if err != nil {
				ctx.Logger.Warn("Failed interacting with chest", slog.Any("error", err))
				continue
			}

			// Pickup items dropped by chest
			action.ItemPickup(30)

			anyOpened = true
			utils.Sleep(500) // small delay after opening
		}

		// If no new chests opened in this pass, we’re done
		if !anyOpened {
			break
		}
	}

	return nil
} */
