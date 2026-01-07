package run

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/data/quest"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/nip"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
)

var baalThronePosition = data.Position{
	X: 15095,
	Y: 5042,
}

type Baal struct {
	ctx                *context.Status
	clearMonsterFilter data.MonsterFilter // Used to clear area (basically TZ)
	preAtkLast         time.Time
	decoyLast          time.Time
}

func NewBaal(clearMonsterFilter data.MonsterFilter) *Baal {
	return &Baal{
		ctx:                context.Get(),
		clearMonsterFilter: clearMonsterFilter,
	}
}

func (s Baal) Name() string {
	return string(config.BaalRun)
}

func (a Baal) CheckConditions(parameters *RunParameters) SequencerResult {
	farmingRun := IsFarmingRun(parameters)
	if !a.ctx.Data.Quests[quest.Act5RiteOfPassage].Completed() {
		if farmingRun {
			return SequencerSkip
		}
		return SequencerStop
	}
	questCompleted := a.ctx.Data.Quests[quest.Act5EveOfDestruction].Completed()
	if (farmingRun && !questCompleted) || (!farmingRun && questCompleted) {
		return SequencerSkip
	}
	return SequencerOk
}
func (s *Baal) Run(parameters *RunParameters) error {
	// Set filter
	filter := data.MonsterAnyFilter()
	if s.ctx.CharacterCfg.Game.Baal.OnlyElites {
		filter = data.MonsterEliteFilter()
	}
	if s.clearMonsterFilter != nil {
		filter = s.clearMonsterFilter
	}

	err := action.WayPoint(area.TheWorldStoneKeepLevel2)
	if err != nil {
		return err
	}

	if s.ctx.CharacterCfg.Game.Baal.ClearFloors || s.clearMonsterFilter != nil {
		action.ClearCurrentLevel(false, filter)
	}

	err = action.MoveToArea(area.TheWorldStoneKeepLevel3)
	if err != nil {
		return err
	}

	if s.ctx.CharacterCfg.Game.Baal.ClearFloors || s.clearMonsterFilter != nil {
		action.ClearCurrentLevel(false, filter)
	}

	err = action.MoveToArea(area.ThroneOfDestruction)
	if err != nil {
		return err
	}
	err = action.MoveToCoords(baalThronePosition)
	if err != nil {
		return err
	}
	if s.checkForSoulsOrDolls() {
		return errors.New("souls or dolls detected, skipping")
	}

	// Let's move to a safe area and open the portal in companion mode
	if s.ctx.CharacterCfg.Companion.Leader {
		action.MoveToCoords(data.Position{
			X: 15116,
			Y: 5071,
		})
		action.OpenTPIfLeader()
	}

	err = action.ClearAreaAroundPlayer(50, data.MonsterAnyFilter())
	if err != nil {
		return err
	}

	// Force rebuff before waves
	action.Buff()

	// Come back to previous position
	err = action.MoveToCoords(baalThronePosition)
	if err != nil {
		return err
	}

	// Process waves until Baal leaves throne
	s.ctx.Logger.Info("Starting Baal waves...")
	waveTimeout := time.Now().Add(7 * time.Minute)

	lastWaveDetected := false
	isWaitingForPortal := false
	_, isLevelingChar := s.ctx.Char.(context.LevelingCharacter)

	for !s.hasBaalLeftThrone() && time.Now().Before(waveTimeout) {
		s.ctx.PauseIfNotPriority()
		s.ctx.RefreshGameData()

		// Detect last wave for logging
		if _, found := s.ctx.Data.Monsters.FindOne(npc.BaalsMinion, data.MonsterTypeMinion); found {
			if !lastWaveDetected {
				s.ctx.Logger.Info("Last wave (Baal's Minion) detected")
				lastWaveDetected = true
			}
		} else if lastWaveDetected {

			if !s.ctx.CharacterCfg.Game.Baal.KillBaal && !isLevelingChar {
				s.ctx.Logger.Info("Waves cleared, skipping Baal kill (Fast Exit).")
				return nil
			}

			if !isWaitingForPortal {
				s.ctx.Logger.Info("Waves cleared, moving to portal position to wait...")
				action.MoveToCoords(data.Position{X: 15090, Y: 5008})
				isWaitingForPortal = true
			}

			utils.Sleep(500)
			continue
		}

		if !isWaitingForPortal {
			action.ClearAreaAroundPosition(baalThronePosition, 50, data.MonsterAnyFilter())
			action.MoveToCoords(baalThronePosition)
			s.preAttackBaalWaves()
		}

		utils.Sleep(500) // Prevent excessive checking
	}

	if !s.hasBaalLeftThrone() {
		return errors.New("baal waves timeout - portal never appeared")
	}

	// Baal has entered the chamber
	s.ctx.Logger.Info("Baal has entered the Worldstone Chamber")

	// Kill Baal Logic
	if s.ctx.CharacterCfg.Game.Baal.KillBaal || isLevelingChar {
		action.Buff()

		s.ctx.Logger.Info("Waiting for Baal portal...")
		var baalPortal data.Object
		found := false

		for i := 0; i < 15; i++ {
			baalPortal, found = s.ctx.Data.Objects.FindOne(object.BaalsPortal)
			if found {
				break
			}
			utils.Sleep(300)
		}

		if !found {
			return errors.New("baal portal not found after waves completed")
		}

		s.ctx.Logger.Info("Entering Baal portal...")

		// Enter portal
		err = action.InteractObject(baalPortal, func() bool {
			return s.ctx.Data.PlayerUnit.Area == area.TheWorldstoneChamber
		})

		// Verify entry
		if s.ctx.Data.PlayerUnit.Area == area.TheWorldstoneChamber {
			s.ctx.Logger.Info("Successfully entered Worldstone Chamber")
		} else if err != nil {
			return fmt.Errorf("failed to enter baal portal: %w", err)
		}

		// Move to Baal (may fail due to tentacles)
		s.ctx.Logger.Info("Moving to Baal...")
		moveErr := action.MoveToCoords(data.Position{X: 15136, Y: 5943})
		if moveErr != nil {
			if strings.Contains(moveErr.Error(), "path could not be calculated") {
				s.ctx.Logger.Info("Path blocked by tentacles, attacking from current position")
			} else {
				s.ctx.Logger.Warn("Failed to move to Baal", "error", moveErr)
			}
		}

		prevPickup := s.ctx.CurrentGame.PickupItems
		s.ctx.CurrentGame.PickupItems = false
		defer func() {
			s.ctx.CurrentGame.PickupItems = prevPickup
		}()

		// Kill Baal
		if err := s.ctx.Char.KillBaal(); err != nil {
			return err
		}
		// 🔒 Special case: ONLY when rerolling marked GCs
		if s.ctx.CharacterCfg.CubeRecipes.RerollGrandCharms {
			// 🚫 Already have a marked GC — do NOT mark another
			if s.ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint != "" {
				s.ctx.Logger.Warn("GRAND CHARM ALREADY MARKED, PROCEED WITH NORMAL PICKUP")
			} else {

				// Give the game time to spawn loot
				utils.Sleep(500)
				s.ctx.RefreshGameData()

				foundCharm := false

				for _, it := range s.ctx.Data.Inventory.ByLocation(item.LocationGround) {
					if it.Name == "GrandCharm" && it.Quality == item.QualityMagic {

						s.ctx.Logger.Warn(
							"GRAND CHARM DROPPED BY BAAL, MARKING FOR REROLL",
							"unitID", it.UnitID,
						)

						s.ctx.MarkedGrandCharmUnitID = it.UnitID

						// --- Pickup the Grand Charm first ---
						err := action.ItemPickup(40)

						s.ctx.Logger.Warn("Waiting 5 seconds after pickup...")
						utils.Sleep(5000)

						if err != nil {
							s.ctx.Logger.Error("Failed to pick up Grand Charm", "unitID", it.UnitID, "err", err)
							break
						}

						s.ctx.RefreshInventory() // make sure it is in inventory

						// --- Fetch the picked-up charm from inventory manually ---
						var charmInInv data.Item
						found := false
						for _, invItem := range s.ctx.Data.Inventory.ByLocation(item.LocationInventory) {
							if invItem.UnitID == s.ctx.MarkedGrandCharmUnitID {
								charmInInv = invItem
								found = true
								break
							}
						}
						if !found {
							s.ctx.Logger.Error("Picked up Grand Charm but cannot find it in inventory", "unitID", s.ctx.MarkedGrandCharmUnitID)
							break
						}

						// --- Tome Identification Starts Here ---
						idTome, found := s.ctx.Data.Inventory.Find(item.TomeOfIdentify, item.LocationInventory)
						if !found {
							s.ctx.Logger.Warn("Tome of Identify not found, skipping identification")
						} else {
							// Open inventory if not already open
							step.CloseAllMenus()
							for !s.ctx.Data.OpenMenus.Inventory {
								s.ctx.HID.PressKeyBinding(s.ctx.Data.KeyBindings.Inventory)
								utils.PingSleep(utils.Critical, 1000)
							}

							s.ctx.Logger.Warn("Identifying Grand Charm via Tome of Identify...")
							identifyMarkedItem(idTome, charmInInv) // now using the correct inventory item

							step.CloseAllMenus()
							s.ctx.RefreshInventory()
							s.ctx.Logger.Warn("Grand Charm successfully identified")
						}
						// --- End Tome Identification ---

						foundCharm = true
						break
					}
				}

				utils.Sleep(150)

				if !foundCharm {
					s.ctx.Logger.Warn("NO GRAND CHARM DROPPED BY BAAL")
				}
			}
		}
		return nil
	}

	return nil
}

// hasBaalLeftThrone checks if Baal has left the throne and entered the Worldstone Chamber
func (s *Baal) hasBaalLeftThrone() bool {
	_, found := s.ctx.Data.Monsters.FindOne(npc.BaalThrone, data.MonsterTypeNone)
	return !found
}

func (s Baal) checkForSoulsOrDolls() bool {
	var npcIds []npc.ID

	if s.ctx.CharacterCfg.Game.Baal.DollQuit {
		npcIds = append(npcIds, npc.UndeadStygianDoll2, npc.UndeadSoulKiller2)
	}
	if s.ctx.CharacterCfg.Game.Baal.SoulQuit {
		npcIds = append(npcIds, npc.BlackSoul2, npc.BurningSoul2)
	}

	for _, id := range npcIds {
		if _, found := s.ctx.Data.Monsters.FindOne(id, data.MonsterTypeNone); found {
			return true
		}
	}

	return false
}

func (s *Baal) preAttackBaalWaves() {
	// Positions adapted from kolbot baal.js preattack
	blizzPos := data.Position{X: 15094, Y: 5027}
	hammerPos := data.Position{X: 15094, Y: 5029}
	throneCenter := data.Position{X: 15093, Y: 5029}
	forwardPos := data.Position{X: 15116, Y: 5026}

	// Simple global cooldown between preattacks to avoid spam
	const preAtkCooldown = 1500 * time.Millisecond
	if !s.preAtkLast.IsZero() && time.Since(s.preAtkLast) < preAtkCooldown {
		return
	}

	if s.ctx.Data.PlayerUnit.Skills[skill.Blizzard].Level > 0 {
		step.CastAtPosition(skill.Blizzard, true, blizzPos)
		s.preAtkLast = time.Now()
		return
	}

	if s.ctx.Data.PlayerUnit.Skills[skill.Meteor].Level > 0 {
		step.CastAtPosition(skill.Meteor, true, blizzPos)
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.FrozenOrb].Level > 0 {
		step.CastAtPosition(skill.FrozenOrb, true, blizzPos)
		s.preAtkLast = time.Now()
		return
	}

	if s.ctx.Data.PlayerUnit.Skills[skill.BlessedHammer].Level > 0 {
		if kb, found := s.ctx.Data.KeyBindings.KeyBindingForSkill(skill.Concentration); found {
			s.ctx.HID.PressKeyBinding(kb)
		}
		step.CastAtPosition(skill.BlessedHammer, true, hammerPos)
		s.preAtkLast = time.Now()
		return
	}

	if s.ctx.Data.PlayerUnit.Skills[skill.Decoy].Level > 0 {
		const decoyCooldown = 10 * time.Second
		if s.decoyLast.IsZero() || time.Since(s.decoyLast) > decoyCooldown {
			decoyPos := data.Position{X: 15092, Y: 5028}
			step.CastAtPosition(skill.Decoy, false, decoyPos)
			s.decoyLast = time.Now()
			s.preAtkLast = time.Now()
			return
		}
	}

	if s.ctx.Data.PlayerUnit.Skills[skill.PoisonNova].Level > 0 {
		step.CastAtPosition(skill.PoisonNova, true, s.ctx.Data.PlayerUnit.Position)
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.DimVision].Level > 0 {
		step.CastAtPosition(skill.DimVision, true, blizzPos)
		s.preAtkLast = time.Now()
		return
	}

	// Druid:
	if s.ctx.Data.PlayerUnit.Skills[skill.Tornado].Level > 0 {
		step.CastAtPosition(skill.Tornado, true, throneCenter)
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.Fissure].Level > 0 {
		step.CastAtPosition(skill.Fissure, true, forwardPos)
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.Volcano].Level > 0 {
		step.CastAtPosition(skill.Volcano, true, forwardPos)
		s.preAtkLast = time.Now()
		return
	}

	// Assassin:
	if s.ctx.Data.PlayerUnit.Skills[skill.LightningSentry].Level > 0 {
		for i := 0; i < 3; i++ {
			step.CastAtPosition(skill.LightningSentry, true, throneCenter)
			utils.Sleep(80)
		}
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.DeathSentry].Level > 0 {
		for i := 0; i < 2; i++ {
			step.CastAtPosition(skill.DeathSentry, true, throneCenter)
			utils.Sleep(80)
		}
		s.preAtkLast = time.Now()
		return
	}
	if s.ctx.Data.PlayerUnit.Skills[skill.ShockWeb].Level > 0 {
		step.CastAtPosition(skill.ShockWeb, true, throneCenter)
		s.preAtkLast = time.Now()
		return
	}
}
func identifyMarkedItem(idTome data.Item, i data.Item) {
	ctx := context.Get()

	// Right-click Tome of Identify
	screenPos := ui.GetScreenCoordsForItem(idTome)
	utils.PingSleep(utils.Medium, 500)
	ctx.HID.Click(game.RightButton, screenPos.X, screenPos.Y)
	utils.PingSleep(utils.Critical, 1000)
	ctx.Logger.Warn("Right-clicked Tome of Identify", "unitID", idTome.UnitID)

	// Left-click the item
	screenPos = ui.GetScreenCoordsForItem(i)
	ctx.HID.Click(game.LeftButton, screenPos.X, screenPos.Y)
	ctx.Logger.Warn("Left-clicked item to identify", "unitID", i.UnitID)

	// 🔎 Poll until the item is identified or timeout occurs
	var identified data.Item
	found := false
	pollCount := 0
	itemSeen := false

	timeout := time.Now().Add(5 * time.Second) // Max 5 seconds to identify
	for time.Now().Before(timeout) {
		ctx.RefreshGameData()
		for _, it := range ctx.Data.Inventory.ByLocation(
			item.LocationInventory,
			item.LocationStash,
			item.LocationSharedStash,
		) {
			if it.UnitID == i.UnitID {
				itemSeen = true
				if it.Identified {
					identified = it
					found = true
					break
				}
			}
		}

		if found {
			ctx.Logger.Warn("Item successfully identified", "unitID", i.UnitID, "polls", pollCount)
			break
		}

		pollCount++
		if pollCount%5 == 0 { // Log every 5 polls (~0.5s)
			ctx.Logger.Warn("Waiting for item to be identified...", "unitID", i.UnitID, "polls", pollCount)
		}

		utils.PingSleep(utils.Light, 100) // Poll every 100ms
	}

	if !itemSeen {
		ctx.Logger.Warn("Item may never have been left-clicked; left-click might have failed", "unitID", i.UnitID)
		ctx.MarkedGrandCharmUnitID = 0 // reset
	}

	if !found {
		ctx.Logger.Error("FAILED TO IDENTIFY ITEM AFTER TIMEOUT", "unitID", i.UnitID)
		ctx.MarkedGrandCharmUnitID = 0 // reset
		return
	}

	// ✅ Fingerprint logic for marked Grand Charm (NOW SAFE)
	if identified.Name == "GrandCharm" &&
		identified.Quality == item.QualityMagic &&
		ctx.MarkedGrandCharmUnitID == identified.UnitID {

		if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(identified); res != nip.RuleResultFullMatch {
			fp := utils.GrandCharmFingerprint(identified)

			ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint = fp
			ctx.Logger.Warn("SAVED MARKED GRAND CHARM FINGERPRINT", "fp", fp)

			if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
				ctx.Logger.Error("FAILED TO SAVE CharacterCfg WITH FINGERPRINT", "err", err)
			}
		} else {
			ctx.Logger.Warn("GRAND CHARM THAT I WAS GOING TO MARK TURNED OUT TO BE A KEEPER, NOT MARKING IT")
		}

		// Clear temporary UnitID tracking (runtime-only)
		ctx.MarkedGrandCharmUnitID = 0
	}
}
