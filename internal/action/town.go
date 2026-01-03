package action

import (
	"errors"
	"fmt"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/utils"
)

func StashFull() bool {
	ctx := context.Get()
	totalUsedSpace := 0

	// Stash tabs are 1-indexed, so we check tabs 2, 3, and 4.
	// These correspond to the first three shared stash tabs.
	tabsToCheck := []int{2, 3, 4}

	for _, tabIndex := range tabsToCheck {
		SwitchStashTab(tabIndex)
		time.Sleep(time.Millisecond * 500)
		ctx.RefreshGameData()

		sharedItems := ctx.Data.Inventory.ByLocation(item.LocationSharedStash)
		for _, it := range sharedItems {
			totalUsedSpace += it.Desc().InventoryWidth * it.Desc().InventoryHeight
		}
	}

	// 3 tabs, 100 spaces each = 300 total spaces. 80% of 300 is 240.
	return totalUsedSpace > 240
}

func PreRun(firstRun bool) error {
	ctx := context.Get()

	// Muling logic for the main farmer character
	if ctx.CharacterCfg.Muling.Enabled && ctx.CharacterCfg.Muling.ReturnTo == "" {
		isStashFull := StashFull()

		if isStashFull {
			muleProfiles := ctx.CharacterCfg.Muling.MuleProfiles
			muleIndex := ctx.CharacterCfg.MulingState.CurrentMuleIndex

			if muleIndex >= len(muleProfiles) {
				ctx.Logger.Error("All mules are full! Cannot stash more items. Stopping.")
				ctx.StopSupervisor()
				return errors.New("all mules are full")
			}

			nextMule := muleProfiles[muleIndex]
			ctx.Logger.Info("Stash is full, preparing to switch to mule.", "mule", nextMule, "index", muleIndex)

			// Increment the index for the next time we come back
			ctx.CharacterCfg.MulingState.CurrentMuleIndex++

			// CRITICAL: Save the updated index to the config file BEFORE switching
			if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
				ctx.Logger.Error("Failed to save muling state before switching", "error", err)
				return err // Stop if we can't save state
			}

			// Trigger the character switch
			ctx.CurrentGame.SwitchToCharacter = nextMule
			ctx.RestartWithCharacter = nextMule
			ctx.CleanStopRequested = true
			ctx.StopSupervisor()
			return ErrMulingNeeded // Stop current execution
		} else {
			// If stash is NOT full and the index is not 0, it means muling just finished.
			// Reset the index and save.
			if ctx.CharacterCfg.MulingState.CurrentMuleIndex != 0 {
				ctx.Logger.Info("Muling process complete, resetting mule index.")
				ctx.CharacterCfg.MulingState.CurrentMuleIndex = 0
				if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
					ctx.Logger.Error("Failed to reset muling state", "error", err)
				}
			}
		}
	}

	DropAndRecoverCursorItem()
	step.SetSkill(skill.Vigor)
	RecoverCorpse()
	ManageBelt()
	// Just to make sure messages like TZ change or public game spam arent on the way
	ClearMessages()
	RefillBeltFromInventory()
	_, isLevelingChar := ctx.Char.(context.LevelingCharacter)

	if firstRun && !isLevelingChar {
		Stash(false)
	}

	if !isLevelingChar {
		// Store items that need to be left unidentified
		if HaveItemsToStashUnidentified() {
			Stash(false)
		}
	}

	// Identify - either via Cain or Tome
	IdentifyAll(false)

	if ctx.CharacterCfg.Game.Leveling.AutoEquip && isLevelingChar {
		AutoEquip()
	}

	// Stash before vendor
	Stash(false)

	// Refill pots, sell, buy etc
	VendorRefill(false, true)

	// Gamble
	Gamble()

	// Stash again if needed
	Stash(false)


	
	

	
	

	if ctx.CharacterCfg.CubeRecipes.PrioritizeRunewords {
		MakeRunewords()
		if !isLevelingChar {
			RerollRunewords()
		}
		CubeRecipes()
	} else {
		CubeRecipes()
		MakeRunewords()
		if !isLevelingChar {
			RerollRunewords()
		}
	}

// --- New addition: ensure upgrade gem is corner-safe for all characters ---
	ctx.Logger.Info("Ensuring upgrade gem is corner-safe...")
	EnsureUpgradeGemCornerSafe()
	ctx.PauseIfNotPriority()
	ctx.Logger.Info("Upgrade gem placement complete.")


	// After creating or rerolling runewords, stash newly created bases/runewords
	// so we don't carry them out to the next area unnecessarily.
	Stash(false)


	if isLevelingChar {
		OptimizeInventory(item.LocationInventory)
	}

	// Leveling related checks
	if ctx.CharacterCfg.Game.Leveling.EnsurePointsAllocation {
		ResetStats()
		EnsureStatPoints()
		EnsureSkillPoints()
	}

	if ctx.CharacterCfg.Game.Leveling.EnsureKeyBinding {
		EnsureSkillBindings()
	}

	HealAtNPC()
	ReviveMerc()
	HireMerc()

	return Repair()
}

func InRunReturnTownRoutine() error {
	ctx := context.Get()
	
_, isLevelingChar := ctx.Char.(context.LevelingCharacter)

	ctx.Logger.Info("Pausing if not priority at start of town routine...")
	ctx.PauseIfNotPriority()

	// Return to town
	ctx.Logger.Info("Returning to town...")
	if err := ReturnTown(); err != nil {
		return fmt.Errorf("failed to return to town: %w", err)
	}

	// Validate we're actually in town
	if !ctx.Data.PlayerUnit.Area.IsTown() {
		return fmt.Errorf("failed to verify town location after portal")
	}
	ctx.Logger.Info("Confirmed character is in town.")

	// Core town preparations
	step.SetSkill(skill.Vigor)
	ctx.Logger.Info("Recovering corpse...")
	RecoverCorpse()
	ctx.PauseIfNotPriority()

	ctx.Logger.Info("Managing belt...")
	ManageBelt()
	ctx.PauseIfNotPriority()

	ctx.Logger.Info("Refilling belt from inventory...")
	RefillBeltFromInventory()
	ctx.PauseIfNotPriority()

	// Stash items that need to remain unidentified
	if ctx.CharacterCfg.Game.UseCainIdentify && HaveItemsToStashUnidentified() {
		ctx.Logger.Info("Stashing items to leave unidentified...")
		Stash(false)
		ctx.PauseIfNotPriority()
	}

	ctx.Logger.Info("Identifying all items...")
	IdentifyAll(false)

	// Auto-equip for leveling characters
	if _, isLevelingChar := ctx.Char.(context.LevelingCharacter); ctx.CharacterCfg.Game.Leveling.AutoEquip && isLevelingChar {
		ctx.Logger.Info("Auto-equipping items for leveling character...")
		AutoEquip()
		ctx.PauseIfNotPriority()
	}

	// Vendor interactions and stash management
	ctx.Logger.Info("Refilling vendor and selling items...")
	VendorRefill(false, true)

	ctx.PauseIfNotPriority() // Check after VendorRefill
	Stash(false)
	ctx.PauseIfNotPriority() // Check after Stash
	Gamble()
	ctx.PauseIfNotPriority() // Check after Gamble
	Stash(false)
	ctx.PauseIfNotPriority() // Check after Stash
	if ctx.CharacterCfg.CubeRecipes.PrioritizeRunewords {
		MakeRunewords()
		// Do not reroll runewords while running the leveling sequences.
		// Leveling characters rely on simpler runeword behavior and base
		// selection, and rerolling could consume resources unexpectedly.
		if !isLevelingChar {
			RerollRunewords()
		}
		CubeRecipes()
		ctx.PauseIfNotPriority() // Check after CubeRecipes
	} else {
		CubeRecipes()
		ctx.PauseIfNotPriority() // Check after CubeRecipes
		MakeRunewords()

		// Do not reroll runewords while running the leveling sequences.
		// Leveling characters rely on simpler runeword behavior and base
		// selection, and rerolling could consume resources unexpectedly.
		if !isLevelingChar {
			RerollRunewords()
		}
	}

	// Ensure any newly created or rerolled runewords/bases are stashed
	// before leaving town.
	Stash(false)
	ctx.PauseIfNotPriority() // Check after post-reroll Stash


	ctx.Logger.Info("Stashing items after vendor...")
	Stash(false)
	ctx.PauseIfNotPriority()

	ctx.Logger.Info("Gambling...")
	Gamble()
	ctx.PauseIfNotPriority()

	ctx.Logger.Info("Stashing items after gambling...")
	Stash(false)
	ctx.PauseIfNotPriority()

	ctx.Logger.Info("Performing cube recipes...")
	CubeRecipes()
	ctx.PauseIfNotPriority()

	ctx.Logger.Info("Making runewords...")
	MakeRunewords()
	ctx.PauseIfNotPriority()

	// Leveling related checks
	if ctx.CharacterCfg.Game.Leveling.EnsurePointsAllocation {
		ctx.Logger.Info("Ensuring stat points allocation...")
		EnsureStatPoints()
		ctx.PauseIfNotPriority()

		ctx.Logger.Info("Ensuring skill points allocation...")
		EnsureSkillPoints()
		ctx.PauseIfNotPriority()
	}

	if ctx.CharacterCfg.Game.Leveling.EnsureKeyBinding {
		ctx.Logger.Info("Ensuring skill key bindings...")
		EnsureSkillBindings()
		ctx.PauseIfNotPriority()
	}

	// NPC interactions
	ctx.Logger.Info("Healing at NPC...")
	HealAtNPC()
	ctx.PauseIfNotPriority()

	ctx.Logger.Info("Reviving mercenary if needed...")
	ReviveMerc()
	ctx.PauseIfNotPriority()

	ctx.Logger.Info("Hiring mercenary if needed...")
	HireMerc()
	ctx.PauseIfNotPriority()

	ctx.Logger.Info("Repairing equipment...")
	Repair()
	ctx.PauseIfNotPriority()

	// --- New addition: ensure upgrade gem is corner-safe for all characters ---
	ctx.Logger.Info("Ensuring upgrade gem is corner-safe...")
	EnsureUpgradeGemCornerSafe()
	ctx.PauseIfNotPriority()
	ctx.Logger.Info("Upgrade gem placement complete.")

	// Portal / town exit
	if ctx.CharacterCfg.Companion.Leader {
		ctx.Logger.Info("Using portal as town leader...")
		UsePortalInTown()
		utils.Sleep(500)
		return OpenTPIfLeader()
	}

	ctx.Logger.Info("Using portal to exit town...")
	return UsePortalInTown()
}
