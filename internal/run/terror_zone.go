package run

import (
	"fmt"
	"slices"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/context"
	terrorzones "github.com/hectorgimenez/koolo/internal/terrorzone"
)

type TerrorZone struct {
	ctx *context.Status
}

func NewTerrorZone() *TerrorZone {
	return &TerrorZone{
		ctx: context.Get(),
	}
}

func (tz TerrorZone) Name() string {
	tzNames := make([]string, 0)
	for _, tzArea := range tz.AvailableTZs() {
		tzNames = append(tzNames, tzArea.Area().Name)
	}

	return fmt.Sprintf("TerrorZone Run: %v", tzNames)
}

func (tz TerrorZone) CheckConditions(parameters *RunParameters) SequencerResult {
	return SequencerError
}

func (tz TerrorZone) Run(parameters *RunParameters) error {

	availableTzs := tz.AvailableTZs()
	if len(availableTzs) == 0 {
		return nil
	}

	// --- Special-case TZs that already have dedicated runs ---
	switch availableTzs[0] {
	case area.PitLevel1, area.PitLevel2:
		return NewPit().Run(parameters)
	case area.Tristram:
		return NewTristram().Run(parameters)
	case area.MooMooFarm:
		return NewCows().Run(parameters)
	case area.TalRashasTomb1:
		return NewTalRashaTombs().Run(parameters)
	case area.AncientTunnels:
		return NewAncientTunnels().Run(parameters)
	case area.ArcaneSanctuary:
		return NewSummonerTZ(tz.customTZEnemyFilter()).Run(parameters)
	case area.Travincal:
		return NewTravincal().Run(parameters)
	case area.DuranceOfHateLevel1:
		return NewMephisto(tz.customTZEnemyFilter()).Run(parameters)
	case area.ChaosSanctuary:
		return NewDiablo().Run(parameters)
	case area.NihlathaksTemple:
		return NewNihlathakTZ(tz.customTZEnemyFilter()).Run(parameters)
	case area.TheWorldStoneKeepLevel1:
		return NewBaal(tz.customTZEnemyFilter()).Run(parameters)
	}

	// --- Generic TZ handling via centralized routes ---
	primary := availableTzs[0]

	routes := terrorzones.RoutesFor(primary)
	if len(routes) == 0 {
		tz.ctx.Logger.Debug(
			"No terror zone route defined for %v — skipping clearing",
			primary.Area().Name,
		)

		// Optional: continue with post-TZ logic, if any
		return nil
	}

	for _, route := range routes {
		for idx, step := range route {
			// Navigation: first step via waypoint, rest via MoveToArea
			if idx == 0 {
				if err := action.WayPoint(step.Area); err != nil {
					return err
				}
			} else {
				if err := action.MoveToArea(step.Area); err != nil {
					return err
				}
			}

			// Clearing: only if the route explicitly says so.
			// We trust routes.go + terrorzones.go to define the correct group.
			if step.Kind == terrorzones.StepClear {
				if err := action.ClearCurrentLevel(
					tz.ctx.CharacterCfg.Game.TerrorZone.OpenChests,
					tz.customTZEnemyFilter(),
				); err != nil {
					return err
				}
			}
		}
	}
	// ------------------------
	// STEP 4: DIE ON PURPOSE IF ABOVE 75% EXPERIENCE
	// ------------------------
	var exp, lastExp, nextExp uint64

	if v, ok := tz.ctx.Data.PlayerUnit.FindStat(stat.Experience, 0); ok {
		exp = uint64(uint32(v.Value))
	}
	if v, ok := tz.ctx.Data.PlayerUnit.FindStat(stat.LastExp, 0); ok {
		lastExp = uint64(uint32(v.Value))
	}
	if v, ok := tz.ctx.Data.PlayerUnit.FindStat(stat.NextExp, 0); ok {
		nextExp = uint64(uint32(v.Value))
	}

	// Compute % towards next level
	expPercent := float64(exp-lastExp) / float64(nextExp-lastExp) * 100

	if tz.ctx.CharacterCfg.Game.TerrorZone.DieOnPurpose && expPercent > 5 {
		tz.ctx.Logger.Warn("DieOnPurpose enabled and exp > 75%, starting intentional death")
		return DieOnPurpose()
	} else {
		tz.ctx.Logger.Warn("DieOnPurpose deferred, exp less than 75%")
		return nil // if DieOnPurpose unchecked
	}
}

func (tz TerrorZone) AvailableTZs() []area.ID {
	tz.ctx.RefreshGameData()
	var availableTZs []area.ID
	for _, tzone := range tz.ctx.Data.TerrorZones {
		for _, tzArea := range tz.ctx.CharacterCfg.Game.TerrorZone.Areas {
			if tzone == tzArea {
				availableTZs = append(availableTZs, tzone)
			}
		}
	}

	return availableTZs
}

func (tz TerrorZone) customTZEnemyFilter() data.MonsterFilter {
	return func(m data.Monsters) []data.Monster {
		var filteredMonsters []data.Monster
		monsterFilter := data.MonsterAnyFilter()
		if tz.ctx.CharacterCfg.Game.TerrorZone.FocusOnElitePacks {
			monsterFilter = data.MonsterEliteFilter()
		}

		for _, mo := range m.Enemies(monsterFilter) {
			isImmune := false
			for _, resist := range tz.ctx.CharacterCfg.Game.TerrorZone.SkipOnImmunities {
				if mo.IsImmune(resist) {
					isImmune = true
				}
			}
			if !isImmune {
				filteredMonsters = append(filteredMonsters, mo)
			}
		}

		return filteredMonsters
	}
}

func DieOnPurpose() error {
	ctx := context.Get()
	ctx.SetLastAction("DieOnPurpose")
	// ------------------------
	// STEP 0: Disable potions & chicken
	// ------------------------
	// Save original health thresholds
	hp := ctx.CharacterCfg.Health
	origChicken := hp.ChickenAt
	origHealing := hp.HealingPotionAt
	origRejuv := hp.RejuvPotionAtLife
	origMana := hp.ManaPotionAt
	origMercChicken := hp.MercChickenAt
	origMercHealing := hp.MercHealingPotionAt
	origMercRejuv := hp.MercRejuvPotionAt

	// Disable all healing & chicken
	ctx.CharacterCfg.Health.ChickenAt = 0
	ctx.CharacterCfg.Health.HealingPotionAt = 0
	ctx.CharacterCfg.Health.RejuvPotionAtLife = 0
	ctx.CharacterCfg.Health.ManaPotionAt = 0
	ctx.CharacterCfg.Health.MercChickenAt = 0
	ctx.CharacterCfg.Health.MercHealingPotionAt = 0
	ctx.CharacterCfg.Health.MercRejuvPotionAt = 0

	defer func() {
		// Restore original values
		ctx.CharacterCfg.Health.ChickenAt = origChicken
		ctx.CharacterCfg.Health.HealingPotionAt = origHealing
		ctx.CharacterCfg.Health.RejuvPotionAtLife = origRejuv
		ctx.CharacterCfg.Health.ManaPotionAt = origMana
		ctx.CharacterCfg.Health.MercChickenAt = origMercChicken
		ctx.CharacterCfg.Health.MercHealingPotionAt = origMercHealing
		ctx.CharacterCfg.Health.MercRejuvPotionAt = origMercRejuv

		ctx.Logger.Warn("INTENTIONAL DEATH: health thresholds restored")
	}()

	/* // OPTIONAL: Hardcore safety guard
	if ctx.Data.CharacterCfg.Hardcore {
		ctx.Logger.Warn("INTENTIONAL DEATH BLOCKED: hardcore character")
		return nil
	} */

	// ------------------------
	// STEP 1: Travel to pull location
	// ------------------------
	if !slices.Contains(ctx.Data.TerrorZones, area.Travincal) {
		ctx.Logger.Warn("Travincial is not terrorized, go die there")
		if err := action.WayPoint(area.Travincal); err != nil {
			ctx.Logger.Warn("Error using Travincal waypoint", "error", err)
			return err
		}

		// Create a temporary Travincal helper to get the council position
		trav := NewTravincal()
		councilPosition := trav.findCouncilPosition()

		if err := action.MoveToCoords(councilPosition); err != nil {
			ctx.Logger.Warn("Error moving to council area", "error", err)
			return err
		}
	} else {
		ctx.Logger.Warn("Travincial is already terrorized, go to Shenk and die there instead")
		if err := action.WayPoint(area.FrigidHighlands); err != nil {
			ctx.Logger.Warn("Error using Frigid Highlands waypoint", "error", err)
			return err
		}

		// Move into position
		if err := action.MoveToCoords(data.Position{X: 3876, Y: 5130}); err != nil {
			ctx.Logger.Warn("Error moving to shenk location", "error", err)
			return err
		}
	}

	ctx.Logger.Warn("INTENTIONAL DEATH: forcing merc to lag behind")

	// ------------------------
	// STEP 2: Make merc lag behind by temporarily disabling teleport
	// ------------------------

	pos := ctx.Data.PlayerUnit.Position

	// Move in increments
	_ = action.MoveToCoords(data.Position{X: pos.X + 15, Y: pos.Y + 15})
	time.Sleep(150 * time.Millisecond)
	_ = action.MoveToCoords(data.Position{X: pos.X + 30, Y: pos.Y + 30})
	time.Sleep(150 * time.Millisecond)
	_ = action.MoveToCoords(pos)
	time.Sleep(150 * time.Millisecond)

	ctx.Logger.Warn("INTENTIONAL DEATH: pulling mobs")

	// ------------------------
	// STEP 3: Pull mobs
	// ------------------------
	pullDeadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(pullDeadline) {
		ctx.RefreshGameData()

		if len(ctx.Data.Monsters) > 0 {
			break
		}

		pos = ctx.Data.PlayerUnit.Position

		_ = action.MoveToCoords(data.Position{X: pos.X + 6, Y: pos.Y})
		_ = action.MoveToCoords(data.Position{X: pos.X - 6, Y: pos.Y})
		_ = action.MoveToCoords(data.Position{X: pos.X, Y: pos.Y + 6})
		_ = action.MoveToCoords(data.Position{X: pos.X, Y: pos.Y - 6})

		time.Sleep(150 * time.Millisecond)
	}

	ctx.Logger.Warn("INTENTIONAL DEATH: standing still")

	// ------------------------
	// STEP 4: Stand still and die
	// ------------------------

	ctx.Logger.Warn("INTENTIONAL DEATH: hard stalling main loop")

	for {
		// NO RefreshGameData
		// NO action calls
		// NO events
		// NO short sleeps

		time.Sleep(10 * time.Second)

		// Occasionally check death using stale data (safe)
		if ctx.Data.PlayerUnit.IsDead() {
			ctx.Logger.Warn("INTENTIONAL DEATH: player is dead")
			return nil
		}
	}
}
