package action

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/nip"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/utils"
)

func StashFull() bool {
	ctx := context.Get()
	totalUsedSpace := 0

	// Stash tabs are 1-indexed:
	// Tab 1 = Personal stash
	// Tabs 2-N = Shared stash pages (N = 2 + SharedStashPages - 1)
	// Non-DLC: 3 shared pages (tabs 2-4)
	// DLC: 5 shared pages (tabs 2-6)
	sharedPages := ctx.Data.Inventory.SharedStashPages
	if sharedPages == 0 {
		// Fallback: assume 3 pages if not detected
		sharedPages = 3
	}

	tabsToCheck := make([]int, sharedPages)
	for i := 0; i < sharedPages; i++ {
		tabsToCheck[i] = i + 2 // Tabs start at 2 (first shared page)
	}

	for _, tabIndex := range tabsToCheck {
		SwitchStashTab(tabIndex)
		time.Sleep(time.Millisecond * 500)
		ctx.RefreshGameData()

		sharedItems := ctx.Data.Inventory.ByLocation(item.LocationSharedStash)
		for _, it := range sharedItems {
			totalUsedSpace += it.Desc().InventoryWidth * it.Desc().InventoryHeight
		}
	}

	// Each page has 100 spaces. 80% threshold for muling.
	// Non-DLC: 3 pages × 100 = 300 spaces, 80% = 240
	// DLC: 5 pages × 100 = 500 spaces, 80% = 400
	maxSpace := sharedPages * 100
	threshold := int(float64(maxSpace) * 0.8)
	return totalUsedSpace > threshold
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

	if ctx.CharacterCfg.Game.Leveling.AutoEquip && isLevelingChar {
		AutoEquip()
	}

	if isLevelingChar {
		OptimizeInventory(item.LocationInventory)
	}

	// Leveling related checks
	if ctx.CharacterCfg.Game.Leveling.EnsurePointsAllocation && isLevelingChar {
		ResetStats()
		EnsureStatPoints()
		EnsureSkillPoints()
	} else if !isLevelingChar && ctx.CharacterCfg.Character.AutoStatSkill.Enabled {
		AutoRespecIfNeeded()
		EnsureStatPoints()
		if !shouldDeferAutoSkillsForStats() {
			EnsureSkillPoints()
			EnsureSkillBindings()
		} else {
			ctx.Logger.Debug("Auto stat targets pending; skipping skill allocation for now.")
		}
	}

	if ctx.CharacterCfg.Game.Leveling.EnsureKeyBinding {
		EnsureSkillBindings()
	}

	HealAtNPC()
	ReviveMerc()
	HireMerc()

	// if specific item in stash does not match nip and is not fingerprint match, make it the fingerprint
	var matcheditem data.Item
	var matchedrareitem data.Item
	itemsInStash := ctx.Data.Inventory.ByLocation(item.LocationStash, item.LocationSharedStash)
	ctx.Logger.Debug("Checking for stale magic fingerprint...")
	ctx.Logger.Debug("Checking for stale rare fingerprint...")
	for _, stashitem := range itemsInStash {

		if ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint != "" && slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Magic Item") {

			if stashitem.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) {
				if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(stashitem); result != nip.RuleResultFullMatch {
					matcheditem = stashitem
					fp := SpecificFingerprint(matcheditem)

					if fp != ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint {
						ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint = fp
						ctx.Logger.Warn("Magic fingerprint mismatch found, updating the stale fingerprint")
					}
				}

			}
		}

		if ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint != "" && slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Rare Item") {

			if stashitem.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) {
				if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(stashitem); result != nip.RuleResultFullMatch {
					matchedrareitem = stashitem
					fpr := SpecificRareFingerprint(matchedrareitem)

					if fpr != ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint {
						ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint = fpr
						ctx.Logger.Warn("Rare fingerprint mismatch found, updating the stale fingerprint")
					}
				}

			}
		}

	}

	if !slices.Contains(ctx.CharacterCfg.Game.Runs, "Cows") {
		itemsInInventory := ctx.Data.Inventory.ByLocation(item.LocationInventory)
		//ctx.Logger.Warn("Checking for stale magic fingerprint...")
		for _, invitem := range itemsInInventory {
			if invitem.Name == "WirtsLeg" && invitem.Quality == item.QualityNormal && !invitem.HasSockets {
				ctx.Logger.Debug("dropping wirts leg that has no sockets, cows disabled")
				ctx.CurrentGame.BlacklistedItems = append(ctx.CurrentGame.BlacklistedItems, invitem) // blacklist it so bot never tries to pick it back up
				DropItem(invitem)                                                                    // Explicitly drop the normal 0-socket wirts leg
			}
		}
	}

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

	if ctx.CharacterCfg.Game.Leveling.EnsurePointsAllocation && isLevelingChar {
		EnsureStatPoints()
		ctx.PauseIfNotPriority()

		ctx.Logger.Info("Ensuring skill points allocation...")
		EnsureSkillPoints()
		ctx.PauseIfNotPriority() // Check after EnsureSkillPoints
	} else if !isLevelingChar && ctx.CharacterCfg.Character.AutoStatSkill.Enabled {
		AutoRespecIfNeeded()
		ctx.PauseIfNotPriority() // Check after AutoRespecIfNeeded
		EnsureStatPoints()
		ctx.PauseIfNotPriority() // Check after EnsureStatPoints
		if !shouldDeferAutoSkillsForStats() {
			EnsureSkillPoints()
			ctx.PauseIfNotPriority() // Check after EnsureSkillPoints
			EnsureSkillBindings()
			ctx.PauseIfNotPriority() // Check after EnsureSkillBindings
		} else {
			ctx.Logger.Debug("Auto stat targets pending; skipping skill allocation for now.")
		}
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

	// if specific item in stash does not match nip and is not fingerprint match, make it the fingerprint
	var matcheditem data.Item
	var matchedrareitem data.Item
	itemsInStash := ctx.Data.Inventory.ByLocation(item.LocationStash, item.LocationSharedStash)
	ctx.Logger.Debug("Checking for stale magic fingerprint...")
	ctx.Logger.Debug("Checking for stale rare fingerprint...")
	for _, stashitem := range itemsInStash {

		if ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint != "" && slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Magic Item") {

			if stashitem.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) {
				if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(stashitem); result != nip.RuleResultFullMatch {
					matcheditem = stashitem
					fp := SpecificFingerprint(matcheditem)

					if fp != ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint {
						ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint = fp
						ctx.Logger.Warn("Magic fingerprint mismatch found, updating the stale fingerprint")
					}
				}

			}
		}

		if ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint != "" && slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Rare Item") {

			if stashitem.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) {
				if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(stashitem); result != nip.RuleResultFullMatch {
					matchedrareitem = stashitem
					fpr := SpecificRareFingerprint(matchedrareitem)

					if fpr != ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint {
						ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint = fpr
						ctx.Logger.Warn("Rare fingerprint mismatch found, updating the stale fingerprint")
					}
				}

			}
		}

	}

	if !slices.Contains(ctx.CharacterCfg.Game.Runs, "Cows") {
		itemsInInventory := ctx.Data.Inventory.ByLocation(item.LocationInventory)
		//ctx.Logger.Warn("Checking for stale magic fingerprint...")
		for _, invitem := range itemsInInventory {
			if invitem.Name == "WirtsLeg" && invitem.Quality == item.QualityNormal && !invitem.HasSockets {
				ctx.Logger.Debug("dropping wirts leg that has no sockets, cows disabled")
				ctx.CurrentGame.BlacklistedItems = append(ctx.CurrentGame.BlacklistedItems, invitem) // blacklist it so bot never tries to pick it back up
				DropItem(invitem)                                                                    // Explicitly drop the normal 0-socket wirts leg
			}
		}
	}

	// Portal / town exit
	if ctx.CharacterCfg.Companion.Leader {
		ctx.Logger.Info("Using portal as town leader...")
		UsePortalInTown()
		utils.Sleep(500)
		return OpenTPIfLeader()
	}

	ctx.Logger.Info("Using portal to exit town...")

	err := UsePortalInTown() // Try to use the portal
	if err != nil {
		return err // If portal fails, return the error
	}

	// Only now is the town routine fully complete
	ctx.JustDidTownRoutine = false

	return nil // Success

}
