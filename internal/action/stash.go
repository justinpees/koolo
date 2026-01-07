package action

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/nip"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/event"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
	"github.com/lxn/win"
)

const (
	maxGoldPerStashTab = 2500000

	// NEW CONSTANTS FOR IMPROVED GOLD STASHING
	minInventoryGoldForStashAggressiveLeveling = 1000   // Stash if inventory gold exceeds 1k during leveling when total gold is low
	maxTotalGoldForAggressiveLevelingStash     = 150000 // Trigger aggressive stashing if total gold (inventory + stashed) is below this
)

var lastSuccessfulStashTab = -1

func Stash(forceStash bool) error {
	ctx := context.Get()
	ctx.SetLastAction("Stash")

	ctx.Logger.Debug("Checking for items to stash...")
	if !isStashingRequired(forceStash) {
		return nil
	}

	ctx.Logger.Info("Stashing items...")

	switch ctx.Data.PlayerUnit.Area {
	case area.KurastDocks:
		MoveToCoords(data.Position{X: 5146, Y: 5067})
	case area.LutGholein:
		MoveToCoords(data.Position{X: 5130, Y: 5086})
	}

	bank, _ := ctx.Data.Objects.FindOne(object.Bank)
	InteractObject(bank,
		func() bool {
			return ctx.Data.OpenMenus.Stash
		},
	)
	// Clear messages like TZ change or public game spam. Prevent bot from clicking on messages
	ClearMessages()
	stashGold()
	stashInventory(forceStash)
	// Add call to dropExcessItems after stashing
	dropExcessItems()
	step.CloseAllMenus()

	return nil
}

func isStashingRequired(firstRun bool) bool {
	ctx := context.Get()
	ctx.SetLastStep("isStashingRequired")

	// Check if the character is currently leveling
	_, isLevelingChar := ctx.Char.(context.LevelingCharacter)

	for _, i := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if i.IsPotion() {
			continue
		}

		stashIt, dropIt, _, _ := shouldStashIt(i, firstRun)

		if stashIt || dropIt {
			return true
		}
	}

	isStashFull := true
	for _, goldInStash := range ctx.Data.Inventory.StashedGold {
		if goldInStash < maxGoldPerStashTab {
			isStashFull = false
			break // Optimization: No need to check further tabs if one has space
		}
	}

	// Calculate total gold (inventory + stashed) for the new aggressive stashing rule
	totalGold := ctx.Data.Inventory.Gold
	for _, stashedGold := range ctx.Data.Inventory.StashedGold {
		totalGold += stashedGold
	}

	// 1. AGGRESSIVE STASHING for leveling characters with LOW TOTAL GOLD
	if isLevelingChar && totalGold < maxTotalGoldForAggressiveLevelingStash && ctx.Data.Inventory.Gold >= minInventoryGoldForStashAggressiveLeveling && !isStashFull {
		ctx.Logger.Debug(fmt.Sprintf("Leveling char with LOW TOTAL GOLD (%.2fk < %.2fk) and INV GOLD (%.2fk) above aggressive threshold (%.2fk). Stashing gold.",
			float64(totalGold)/1000, float64(maxTotalGoldForAggressiveLevelingStash)/1000,
			float64(ctx.Data.Inventory.Gold)/1000, float64(minInventoryGoldForStashAggressiveLeveling)/1000))
		return true
	}

	// 2. STANDARD STASHING for all other cases (non-leveling, or leveling with sufficient total gold)
	if ctx.Data.Inventory.Gold > ctx.Data.PlayerUnit.MaxGold()/3 && !isStashFull {
		ctx.Logger.Debug(fmt.Sprintf("Inventory gold (%.2fk) is above standard threshold (%.2fk). Stashing gold.",
			float64(ctx.Data.Inventory.Gold)/1000, float64(ctx.Data.PlayerUnit.MaxGold())/3/1000))
		return true
	}

	return false
}

func stashGold() {
	ctx := context.Get()
	ctx.SetLastAction("stashGold")

	if ctx.Data.Inventory.Gold == 0 {
		return
	}

	ctx.Logger.Info("Stashing gold...", slog.Int("gold", ctx.Data.Inventory.Gold))

	for tab, goldInStash := range ctx.Data.Inventory.StashedGold {
		ctx.RefreshGameData()
		if ctx.Data.Inventory.Gold == 0 {
			ctx.Logger.Info("All inventory gold stashed.") // Added log for clarity
			return
		}

		if goldInStash < maxGoldPerStashTab {
			SwitchStashTab(tab + 1) // Stash tabs are 0-indexed in data, but 1-indexed for UI interaction
			clickStashGoldBtn()
			utils.PingSleep(utils.Critical, 1000) // Critical operation: Wait for stash UI to process gold deposit
			// After clicking, refresh data again to see if gold is now 0 or less
			ctx.RefreshGameData()             // Crucial: Refresh data to see if gold has been deposited
			if ctx.Data.Inventory.Gold == 0 { // Check if all gold was stashed in this tab
				ctx.Logger.Info("All inventory gold stashed.")
				return
			}
		}
	}

	ctx.Logger.Info("All stash tabs are full of gold :D")
}

func stashInventory(firstRun bool) {
	ctx := context.Get()
	ctx.SetLastAction("stashInventory")

	// Determine starting tab based on configuration
	startTab := 1 // Personal stash by default (tab 1)
	if ctx.CharacterCfg.Character.StashToShared {
		startTab = 2 // Start with first shared stash tab if configured (tabs 2-4 are shared)
	}

	currentTab := startTab
	SwitchStashTab(currentTab)

	// ------------------------------------------------------------------
	// ENSURE GEM TO UPGRADE EXISTS IN INVENTORY
	// ------------------------------------------------------------------
	if ctx.CharacterCfg.Inventory.GemToUpgrade != "None" {

		// Safety: stash must be open
		if !ctx.Data.OpenMenus.Stash {
			ctx.Logger.Warn("Stash not open while trying to pull gem for upgrade")
		} else {

			gemName := item.Name(ctx.CharacterCfg.Inventory.GemToUpgrade)

			// Refresh before checking inventory / stash
			ctx.RefreshGameData()

			// Count gem in inventory
			invCount := 0
			for _, it := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
				if it.Name == gemName {
					invCount++
				}
			}

			// If none in inventory, pull ONE from stash (prefer shared stash)
			if invCount == 0 {

				ctx.Logger.Debug(
					"No gem to upgrade in inventory, searching stash",
					slog.String("gem", string(gemName)),
				)

				stashedItems := ctx.Data.Inventory.ByLocation(
					item.LocationSharedStash,
					item.LocationStash,
				)

				for _, it := range stashedItems {
					if it.Name != gemName {
						continue
					}

					// Switch to correct stash tab
					SwitchStashTab(it.Location.Page + 1)
					utils.PingSleep(utils.Medium, 200)

					// Move gem to inventory
					screenPos := ui.GetScreenCoordsForItem(it)
					ctx.HID.MovePointer(screenPos.X, screenPos.Y)
					ctx.HID.ClickWithModifier(game.LeftButton, screenPos.X, screenPos.Y, game.CtrlKey)
					utils.PingSleep(utils.Medium, 500)

					// Refresh and verify move
					ctx.RefreshGameData()

					found := false
					for _, it2 := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
						if it2.Name == gemName {
							found = true
							break
						}
					}

					if !found {
						ctx.Logger.Warn("FAILED TO MOVE " + ctx.CharacterCfg.Inventory.GemToUpgrade + " FROM STASH TO INVENTORY")
						continue
					}

					ctx.Logger.Debug("MOVED " + ctx.CharacterCfg.Inventory.GemToUpgrade + " FROM STASH TO INVENTORY FOR SHRINE UPGRADE")

					break // ONLY take one
				}
			}
		}
	}

	// Make a copy of inventory items to avoid issues if the slice changes during iteration
	itemsToProcess := make([]data.Item, 0)
	for _, i := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if i.IsPotion() {
			continue
		}

		itemsToProcess = append(itemsToProcess, i)
	}

	for _, i := range itemsToProcess {
		stashIt, dropIt, matchedRule, ruleFile := shouldStashIt(i, firstRun)

		if dropIt {
			ctx.Logger.Debug(fmt.Sprintf("Dropping item %s [%s] due to MaxQuantity rule.", i.Desc().Name, i.Quality.ToString()))
			blacklistItem(i)
			utils.PingSleep(utils.Medium, 500) // Medium operation: Prepare for item drop
			DropItem(i)
			utils.PingSleep(utils.Medium, 500) // Medium operation: Wait for drop to complete
			step.CloseAllMenus()
			continue
		}

		if !stashIt {
			continue
		}

		// Determine target tab for this specific item
		targetStartTab := startTab

		// Always stash unique charms to the shared stash (override personal stash setting)
		if (i.Name == "grandcharm" || i.Name == "smallcharm" || i.Name == "largecharm") && i.Quality == item.QualityUnique {
			targetStartTab = 2 // Force shared stash for unique charms
		}
		if i.Name == "WirtsLeg" {
			targetStartTab = 1 // Force personal stash for Wirt's Leg
		}

		itemStashed := false
		maxTab := 4
		name := i.Desc().Name
		lowerName := strings.ToLower(name)

		// Priority items
		isPriorityItem :=
			strings.Contains(lowerName, "rune") ||
				strings.Contains(lowerName, "jewel") ||
				strings.Contains(lowerName, "ring") ||
				strings.Contains(lowerName, "amulet") ||
				strings.Contains(lowerName, "token of absolution") ||
				strings.Contains(lowerName, "essence") ||
				strings.Contains(lowerName, "amethyst") ||
				strings.Contains(lowerName, "ruby") ||
				strings.Contains(lowerName, "sapphire") ||
				strings.Contains(lowerName, "topaz") ||
				strings.Contains(lowerName, "emerald") ||
				strings.Contains(lowerName, "diamond")

		// 1. Priority items → try tab 2 first
		if isPriorityItem {
			priorityTab := 2
			SwitchStashTab(priorityTab)
			if stashItemAction(i, matchedRule, ruleFile, firstRun) {
				lastSuccessfulStashTab = priorityTab
				itemStashed = true
				ctx.Logger.Info(fmt.Sprintf("Priority item %s stashed to tab %d", name, priorityTab))
			}
		}

		// 2. Try last successful stash tab first (skip tab 2 for non-priority)
		if !itemStashed && lastSuccessfulStashTab != -1 {
			if !isPriorityItem && lastSuccessfulStashTab == 2 {
				// skip
			} else {
				SwitchStashTab(lastSuccessfulStashTab)
				if stashItemAction(i, matchedRule, ruleFile, firstRun) {
					itemStashed = true
				}
			}
		}

		// 3. Normal stash rotation (skip tab 2 for non-priority items)
		if !itemStashed {
			fallbackToTab2 := false

			for tabAttempt := targetStartTab; tabAttempt <= maxTab; tabAttempt++ {
				// Skip tab 2 for non-priority items for now
				if !isPriorityItem && tabAttempt == 2 {
					fallbackToTab2 = true
					continue
				}
				// Skip last successful tab
				if tabAttempt == lastSuccessfulStashTab {
					continue
				}

				SwitchStashTab(tabAttempt)
				if stashItemAction(i, matchedRule, ruleFile, firstRun) {
					if tabAttempt > 1 {
						lastSuccessfulStashTab = tabAttempt
					}
					itemStashed = true
					break
				}
			}

			// Only fallback to tab 2 if no other tab worked
			if !itemStashed && fallbackToTab2 {
				SwitchStashTab(2)
				if stashItemAction(i, matchedRule, ruleFile, firstRun) {
					lastSuccessfulStashTab = 2
					itemStashed = true
				}
			}
		}

		// 4. Fallback to personal stash if nothing else worked
		if !itemStashed {
			SwitchStashTab(1)
			if stashItemAction(i, matchedRule, ruleFile, firstRun) {
				itemStashed = true
			}
		}

		// 5. Final warning

		if !itemStashed {
			stashed := stashItemAcrossTabs(i, matchedRule, ruleFile, firstRun)
			if !stashed {
				ctx.Logger.Warn(fmt.Sprintf(
					"ERROR: Item %s [%s] could not be stashed into any tab. All stash tabs might be full.",
					i.Desc().Name, i.Quality.ToString(),
				))
			}
		}
	}

	step.CloseAllMenus()
}

// stashItemAcrossTabs attempts to stash the given item across available tabs, applying the same logic
// used by the main stash routine. It returns true if the item was stashed successfully.
func stashItemAcrossTabs(i data.Item, matchedRule string, ruleFile string, firstRun bool) bool {
	ctx := context.Get()
	displayName := formatItemName(i)

	startTab := 1
	if ctx.CharacterCfg.Character.StashToShared {
		startTab = 2
	}

	targetStartTab := startTab
	if (i.Name == "grandcharm" || i.Name == "smallcharm" || i.Name == "largecharm") && i.Quality == item.QualityUnique {
		targetStartTab = 2
	}

	itemStashed := false
	maxTab := 4

	for tabAttempt := targetStartTab; tabAttempt <= maxTab; tabAttempt++ {
		SwitchStashTab(tabAttempt)

		if stashItemAction(i, matchedRule, ruleFile, firstRun) {
			itemStashed = true
			r, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(i)

			if res != nip.RuleResultFullMatch && firstRun {
				ctx.Logger.Info(
					fmt.Sprintf("Item %s [%s] stashed to tab %d because it was found in the inventory during the first run.", displayName, i.Quality.ToString(), tabAttempt),
				)
			} else {
				ctx.Logger.Info(
					fmt.Sprintf("Item %s [%s] stashed to tab %d", displayName, i.Quality.ToString(), tabAttempt),
					slog.String("nipFile", fmt.Sprintf("%s:%d", r.Filename, r.LineNumber)),
					slog.String("rawRule", r.RawLine),
				)
			}
			break
		}
		ctx.Logger.Debug(fmt.Sprintf("Item %s could not be stashed on tab %d. Trying next.", displayName, tabAttempt))
	}

	if !itemStashed && targetStartTab == 2 {
		ctx.Logger.Debug(fmt.Sprintf("All shared stash tabs full for %s, trying personal stash as fallback", displayName))
		SwitchStashTab(1)
		if stashItemAction(i, matchedRule, ruleFile, firstRun) {
			itemStashed = true
			ctx.Logger.Info(fmt.Sprintf("Item %s [%s] stashed to personal stash (tab 1) as fallback", displayName, i.Quality.ToString()))
		}
	}

	return itemStashed
}

// shouldStashIt now returns stashIt, dropIt, matchedRule, ruleFile
func shouldStashIt(i data.Item, firstRun bool) (bool, bool, string, string) {
	ctx := context.Get()
	ctx.SetLastStep("shouldStashIt")

	// Don't stash items in protected slots (highest priority exclusion)
	if ctx.CharacterCfg.Inventory.InventoryLock[i.Position.Y][i.Position.X] == 0 {
		return false, false, "", ""
	}

	// These items should NEVER be stashed, regardless of quest status, pickit rules, or first run.
	fmt.Printf("DEBUG: Evaluating item '%s' for *absolute* exclusion from stash.\n", i.Name)
	if i.Name == "horadricstaff" { // This is the simplest way given your logs
		fmt.Printf("DEBUG: ABSOLUTELY PREVENTING stash for '%s' (Horadric Staff exclusion).\n", i.Name)
		return false, false, "", "" // Explicitly do NOT stash the Horadric Staff
	}

	if ctx.CharacterCfg.Inventory.GemToUpgrade != "None" {
		// Count flawless skulls currently in inventory
		invCount := 0

		// Loop through all items in the inventory to count the flawless skulls
		for _, itemInInventory := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
			if itemInInventory.Name == item.Name(ctx.CharacterCfg.Inventory.GemToUpgrade) {
				invCount++
			}
		}

		// Check if the current item is a flawless skull
		if i.Name == item.Name(ctx.CharacterCfg.Inventory.GemToUpgrade) {
			if invCount <= 1 {
				// If it's the first flawless skull, KEEP it in inventory
				ctx.Logger.Debug("KEEPING GEM IN INVENTORY TO UPGRADE WITH SHRINE", "gem", ctx.CharacterCfg.Inventory.GemToUpgrade)
				return false, false, "", "" // do NOT stash, do NOT drop
			} else {
				// If it's an extra flawless skull, STASH it, NEVER drop it
				ctx.Logger.Debug("EXTRA " + ctx.CharacterCfg.Inventory.GemToUpgrade + " DETECTED, STASHING THIS ONE")
				return true, false, "", "" // stash=true, drop=false
			}
		}

	}

	if i.Name == "TomeOfTownPortal" || i.Name == "TomeOfIdentify" || i.Name == "Key" {
		fmt.Printf("DEBUG: ABSOLUTELY PREVENTING stash for '%s' (Quest/Special item exclusion).\n", i.Name)
		return false, false, "", ""
	}

	if _, isLevelingChar := ctx.Char.(context.LevelingCharacter); isLevelingChar && i.IsFromQuest() && i.Name != "HoradricCube" || i.Name == "HoradricStaff" {
		return false, false, "", ""
	}

	if firstRun {
		fmt.Printf("DEBUG: Allowing stash for '%s' (first run).\n", i.Name)
		return true, false, "FirstRun", ""
	}

	// DONT KNOW IF I NEED THIS, LOG DOESNT SHOW UP
	// 🔒 Grand Charm handling when rerolling marked GCs
	if ctx.CharacterCfg.CubeRecipes.RerollGrandCharms &&
		i.Name == "GrandCharm" &&
		i.Quality == item.QualityMagic &&
		ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint != "" {

		fp := utils.GrandCharmFingerprint(i)
		if fp == ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint {
			ctx.Logger.Warn("FORCING STASH OF MARKED GRAND CHARM", "fp", fp)
			return true, false, "MarkedGrandCharm", "" // Always stash
		}
	}

	/* // 🔒 Grand Charm handling when rerolling marked GCs
	if ctx.CharacterCfg.CubeRecipes.RerollGrandCharms &&
		i.Name == "GrandCharm" &&
		i.Quality == item.QualityMagic {

		markedFP := ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint
		if markedFP != "" {

			fp := utils.GrandCharmFingerprint(i)

			// 🎯 MARKED GRAND CHARM
			if fp == markedFP {
				// ✅ Check for godly roll: +1 Skill Tab AND MaxHP ≥ 41
				skillTabStat, _ := i.FindStat(stat.AddSkillTab, 0)
				maxHPStat, _ := i.FindStat(stat.MaxLife, 0)

				if skillTabStat.Value == 1 && maxHPStat.Value >= 41 {
					// Godly roll — stop rerolling and stash
					ctx.Logger.Warn("GODLY GRAND CHARM FOUND — STOPPING REROLL")
					// Clear fingerprint to indicate reroll is complete
					ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint = ""
					if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
						ctx.Logger.Error("FAILED TO SAVE CONFIG AFTER GODLY GC", "err", err)
					}
					return true, false, "GodlyGrandCharm", ""
				}

				// Not godly — keep rerolling
				ctx.Logger.Warn("MARKED GRAND CHARM NOT GODLY — KEEP REROLLING")
				return true, false, "MarkedGrandCharm", ""
			}

			// 🚫 UNMARKED GC — do NOT touch fingerprint here
			if fp != markedFP {
				if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(i); res != nip.RuleResultFullMatch {
					ctx.Logger.Warn("DROPPING UNMARKED NON-NIP GRAND CHARM (only reroll >= ilvl91 mode)")
					return false, false, "", ""
				}

				ctx.Logger.Warn("STASHING UNMARKED GRAND CHARM BECAUSE IT MATCHES NIP")
				return true, false, "", ""
			}
		}
	} */

	// Stash items that are part of a recipe which are not covered by the NIP rules
	if shouldKeepRecipeItem(i) {
		return true, false, "Item is part of a enabled recipe", ""
	}

	// Location/position checks
	if i.Position.Y >= len(ctx.CharacterCfg.Inventory.InventoryLock) || i.Position.X >= len(ctx.CharacterCfg.Inventory.InventoryLock[0]) {
		return false, false, "", ""
	}

	if i.Location.LocationType == item.LocationInventory && ctx.CharacterCfg.Inventory.InventoryLock[i.Position.Y][i.Position.X] == 0 || i.IsPotion() {
		return false, false, "", ""
	}

	// NOW, evaluate pickit rules.
	tierRule, mercTierRule := ctx.CharacterCfg.Runtime.Rules.EvaluateTiers(i, ctx.CharacterCfg.Runtime.TierRules)
	if tierRule.Tier() > 0.0 && IsBetterThanEquipped(i, false, PlayerScore) {
		return true, true, tierRule.RawLine, tierRule.Filename + ":" + strconv.Itoa(tierRule.LineNumber)
	}

	if mercTierRule.Tier() > 0.0 && IsBetterThanEquipped(i, true, MercScore) {
		return true, true, mercTierRule.RawLine, mercTierRule.Filename + ":" + strconv.Itoa(mercTierRule.LineNumber)
	}

	// NOW, evaluate pickit rules.
	rule, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAllIgnoreTiers(i)

	if res == nip.RuleResultFullMatch {

		// 🔒 Special case: ONLY when rerolling marked GCs
		if ctx.CharacterCfg.CubeRecipes.RerollGrandCharms {
			// 🔑 CHECK IF THIS IS THE MARKED GRAND CHARM
			if i.Name == "GrandCharm" && i.Quality == item.QualityMagic {
				fp := utils.GrandCharmFingerprint(i)

				if fp == ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint {
					ctx.Logger.Error("REROLLED GRAND CHARM MATCHES NIP — CLEARING MARK")

					// ✅ RESET STATE
					ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint = ""
					ctx.MarkedGrandCharmUnitID = 0

					// Persist config so restart is safe
					config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg)
				}
			}
		}

		if doesExceedQuantity(rule) {
			// If it matches a rule but exceeds quantity, we want to drop it, not stash.
			fmt.Printf("DEBUG: Dropping '%s' because MaxQuantity is exceeded.\n", i.Name)
			return false, true, rule.RawLine, rule.Filename + ":" + strconv.Itoa(rule.LineNumber)
		} else {
			// If it matches a rule and quantity is fine, stash it.
			fmt.Printf("DEBUG: Allowing stash for '%s' (pickit rule match: %s).\n", i.Name, rule.RawLine)
			return true, false, rule.RawLine, rule.Filename + ":" + strconv.Itoa(rule.LineNumber)
		}
	}

	if i.IsRuneword {
		return true, false, "Runeword", ""
	}

	fmt.Printf("DEBUG: Disallowing stash for '%s' (no rule match and not explicitly kept, and not exceeding quantity).\n", i.Name)
	return false, false, "", "" // Default if no other rule matches
}

// shouldKeepRecipeItem decides whether the bot should stash a low-quality item that is part of an enabled cube recipe.
// It now supports keeping multiple jewels for crafting via maxJewelsKept.
// shouldKeepRecipeItem decides whether the bot should stash a low-quality item that is part of an enabled cube recipe.
// It now supports keeping multiple jewels for crafting via JewelsToKeep.
// shouldKeepRecipeItem decides whether the bot should stash a low-quality item that is part of an enabled cube recipe.
// It now supports keeping multiple jewels (of any quality) for crafting via JewelsToKeep.
func shouldKeepRecipeItem(i data.Item) bool {
	ctx := context.Get()
	ctx.SetLastStep("shouldKeepRecipeItem")

	// For non-jewel items: only normal/magic quality can be part of recipes
	// For jewels: any quality (magic, rare, unique, etc.) can be used in crafting recipes
	if string(i.Name) != "Jewel" && i.Quality > item.QualityMagic {
		return false
	}

	itemInStashNotMatchingRule := false
	jewelCount := 0

	// Count ALL non-NIP jewels in stash (regardless of quality: magic, rare, unique, etc.)
	for _, it := range ctx.Data.Inventory.ByLocation(item.LocationStash, item.LocationSharedStash) {
		if string(it.Name) == "Jewel" {
			if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(it); res != nip.RuleResultFullMatch {
				jewelCount++
			}
		}
		// For OTHER recipe items (not jewels): match on base name and require magic quality
		// so only another magic item of the same base blocks us
		if string(it.Name) != "Jewel" && strings.EqualFold(string(it.Name), string(i.Name)) && it.Quality == item.QualityMagic {
			_, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(it)
			if res != nip.RuleResultFullMatch {
				itemInStashNotMatchingRule = true
			}
		}
	}

	// CRITICAL: Also count ALL non-NIP jewels currently in inventory (any quality, excluding the one we're evaluating)
	// because they will also be stashed in the same run
	for _, it := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if string(it.Name) == "Jewel" && it.UnitID != i.UnitID {
			if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(it); res != nip.RuleResultFullMatch {
				jewelCount++
			}
		}
	}

	recipeMatch := false

	// Check if the item is part of an enabled recipe
	for _, recipe := range Recipes {

		// 🔒 Special case: ignore GrandCharm as a recipe ingredient when rerolling marked GCs
		if ctx.CharacterCfg.CubeRecipes.RerollGrandCharms &&
			recipe.Name == "Reroll GrandCharms" &&
			i.Name == "GrandCharm" {
			continue
		}

		if slices.Contains(recipe.Items, string(i.Name)) &&
			slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, recipe.Name) {
			recipeMatch = true
			break
		}
	}

	// Special-case: For jewels of ANY quality used in crafting recipes, stash up to JewelsToKeep copies.
	if string(i.Name) == "Jewel" {
		if recipeMatch && jewelCount < ctx.CharacterCfg.CubeRecipes.JewelsToKeep {
			ctx.Logger.Debug(fmt.Sprintf("Keeping jewel (quality: %s) for recipe - current count: %d, limit: %d",
				i.Quality.ToString(), jewelCount, ctx.CharacterCfg.CubeRecipes.JewelsToKeep))
			return true
		}
		ctx.Logger.Debug(fmt.Sprintf("NOT keeping jewel (quality: %s) - count: %d, limit: %d, recipeMatch: %v",
			i.Quality.ToString(), jewelCount, ctx.CharacterCfg.CubeRecipes.JewelsToKeep, recipeMatch))
		return false
	}

	// For all other recipe items, keep one copy in the stash if none exists
	if recipeMatch && !itemInStashNotMatchingRule {
		return true
	}

	return false
}

func stashItemAction(i data.Item, rule string, ruleFile string, skipLogging bool) bool {
	ctx := context.Get()
	ctx.SetLastAction("stashItemAction")

	screenPos := ui.GetScreenCoordsForItem(i)
	ctx.HID.MovePointer(screenPos.X, screenPos.Y)
	utils.PingSleep(utils.Medium, 170)        // Medium operation: Move pointer to item
	screenshot := ctx.GameReader.Screenshot() // Take screenshot *before* attempting stash
	utils.PingSleep(utils.Medium, 150)        // Medium operation: Wait for screenshot
	ctx.HID.ClickWithModifier(game.LeftButton, screenPos.X, screenPos.Y, game.CtrlKey)
	utils.PingSleep(utils.Medium, 500) // Medium operation: Give game time to process the stash

	// Verify if the item is no longer in inventory
	ctx.RefreshGameData() // Crucial: Refresh data to see if item moved
	for _, it := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if it.UnitID == i.UnitID {
			ctx.Logger.Debug(fmt.Sprintf("Failed to stash item %s (UnitID: %d), still in inventory.", i.Name, i.UnitID))
			return false // Item is still in inventory, stash failed
		}
	}

	dropLocation := "unknown"

	// Check if the item was picked up in-game
	if areaId, found := ctx.CurrentGame.PickedUpItems[int(i.UnitID)]; found {
		dropLocation = area.ID(areaId).Area().Name
		if slices.Contains(ctx.Data.TerrorZones, area.ID(areaId)) {
			dropLocation += " (terrorized)"
		}
	} else if vendorName, found := ctx.CurrentGame.PickedUpItemsVendor[int(i.UnitID)]; found {
		// Item was bought from a vendor
		dropLocation = vendorName
	}

	// Don't log items that we already have in inventory during first run or that we don't want to notify about (gems, low runes .. etc)
	if !skipLogging && shouldNotifyAboutStashing(i) && ruleFile != "" {
		var message string
		mentions := []string{}
		if config.Koolo != nil {
			for _, id := range config.Koolo.Discord.MentionID {
				if strings.TrimSpace(id) != "" {
					mentions = append(mentions, "<@"+id+">")
				}
			}
		}

		if len(mentions) > 0 {
			// Include Discord mentions
			mentions := []string{}
			for _, id := range config.Koolo.Discord.MentionID {
				if id != "" {
					mentions = append(mentions, "<@"+id+">")
				}
			}
			message = fmt.Sprintf("%s %s [%s] found in \"%s\" (%s)",
				strings.Join(mentions, " "),
				i.Name,
				i.Quality.ToString(),
				dropLocation,
				filepath.Base(ruleFile),
			)
		} else {
			// No Discord mentions
			message = fmt.Sprintf("%s [%s] found in \"%s\" (%s)",
				i.Name,
				i.Quality.ToString(),
				dropLocation,
				filepath.Base(ruleFile),
			)
		}

		event.Send(event.ItemStashed(
			event.WithScreenshot(ctx.Name, message, screenshot),
			data.Drop{
				Item:         i,
				Rule:         rule,
				RuleFile:     ruleFile,
				DropLocation: dropLocation,
			},
		))
	}

	return true // Item successfully stashed
}

func formatItemName(i data.Item) string {
	if i.IsRuneword && i.RunewordName != item.RunewordNone {
		if rwName := string(item.Name(i.RunewordName)); rwName != "" {
			return rwName
		}
	}

	if i.IdentifiedName != "" {
		return i.IdentifiedName
	}

	if desc := i.Desc().Name; desc != "" {
		return desc
	}

	return string(i.Name)
}

// dropExcessItems iterates through inventory and drops items marked for dropping
func dropExcessItems() {
	ctx := context.Get()
	ctx.SetLastAction("dropExcessItems")

	itemsToDrop := make([]data.Item, 0)
	for _, i := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if i.IsPotion() {
			continue
		}

		_, dropIt, _, _ := shouldStashIt(i, false) // Re-evaluate if it should be dropped (not firstRun)
		if dropIt {
			itemsToDrop = append(itemsToDrop, i)
		}
	}

	if len(itemsToDrop) > 0 {
		ctx.Logger.Debug(fmt.Sprintf("Dropping %d excess items from inventory.", len(itemsToDrop)))
		// Ensure we are not in a menu before dropping
		step.CloseAllMenus()

		for _, i := range itemsToDrop {
			DropItem(i)
		}
	}
}

func blacklistItem(i data.Item) {
	ctx := context.Get()
	ctx.CurrentGame.BlacklistedItems = append(ctx.CurrentGame.BlacklistedItems, i)
	ctx.Logger.Info(fmt.Sprintf("Blacklisted item %s (UnitID: %d) to prevent immediate re-pickup.", i.Name, i.UnitID))
}

// DropItem handles moving an item from inventory to the ground
func DropItem(i data.Item) {
	ctx := context.Get()
	ctx.SetLastAction("DropItem")
	utils.PingSleep(utils.Medium, 170) // Medium operation: Prepare for drop
	step.CloseAllMenus()
	utils.PingSleep(utils.Medium, 170) // Medium operation: Wait for menus to close
	ctx.HID.PressKeyBinding(ctx.Data.KeyBindings.Inventory)
	utils.PingSleep(utils.Medium, 170) // Medium operation: Wait for inventory to open
	screenPos := ui.GetScreenCoordsForItem(i)
	ctx.HID.MovePointer(screenPos.X, screenPos.Y)
	utils.PingSleep(utils.Medium, 170) // Medium operation: Position pointer on item
	ctx.HID.ClickWithModifier(game.LeftButton, screenPos.X, screenPos.Y, game.CtrlKey)
	utils.PingSleep(utils.Medium, 500) // Medium operation: Wait for item to drop
	step.CloseAllMenus()
	utils.PingSleep(utils.Medium, 170) // Medium operation: Clean up UI
	ctx.RefreshGameData()
	for _, it := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if it.UnitID == i.UnitID {
			ctx.Logger.Warn(fmt.Sprintf("Failed to drop item %s (UnitID: %d), still in inventory. Inventory might be full or area restricted.", i.Name, i.UnitID))
			return
		}
	}
	ctx.Logger.Debug(fmt.Sprintf("Successfully dropped item %s (UnitID: %d).", i.Name, i.UnitID))

	step.CloseAllMenus()
}

func shouldNotifyAboutStashing(i data.Item) bool {
	ctx := context.Get()

	if ctx.IsBossEquipmentActive {
		return false
	}

	ctx.Logger.Debug(fmt.Sprintf("Checking if we should notify about stashing %s %v", i.Name, i.Desc()))
	// Don't notify about gems
	if strings.Contains(i.Desc().Type, "gem") {
		return false
	}
	// Don't notify tokens (NAME-based)
	if strings.Contains(strings.ToLower(string(i.Name)), "tokenofabsolution") {
		return false
	}

	// Don't notify keys (NAME-based: Terror / Hate / Destruction)
	if strings.Contains(strings.ToLower(string(i.Name)), "keyofterror") || strings.Contains(strings.ToLower(string(i.Name)), "keyofdestruction") || strings.Contains(strings.ToLower(string(i.Name)), "keyofhate") {
		return false
	}
	// Skip low runes (below lem)
	lowRunes := []string{"elrune", "eldrune", "tirrune", "nefrune", "ethrune", "ithrune", "talrune", "ralrune", "ortrune", "thulrune", "amnrune", "solrune", "shaelrune", "dolrune", "helrune", "iorune", "lumrune", "korune", "falrune", "pulrune", "lemrune"}
	if i.Desc().Type == item.TypeRune {
		itemName := strings.ToLower(string(i.Name))
		for _, runeName := range lowRunes {
			if itemName == runeName {
				if !(i.Name == "tirrune" || i.Name == "talrune" || i.Name == "ralrune" || i.Name == "ortrune" || i.Name == "thulrune" || i.Name == "amnrune" || i.Name == "solrune" || i.Name == "lumrune" || i.Name == "nefrune") { // Exclude specific runes from low rune skip logic if they are part of a recipe you want to keep
					return false
				}
			}
		}
	}

	return true
}

func clickStashGoldBtn() {
	ctx := context.Get()
	ctx.SetLastStep("clickStashGoldBtn")

	utils.PingSleep(utils.Medium, 170) // Medium operation: Prepare for gold button click
	if ctx.GameReader.LegacyGraphics() {
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnXClassic, ui.StashGoldBtnYClassic)
		utils.PingSleep(utils.Critical, 1000) // Critical operation: Wait for confirm dialog
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnConfirmXClassic, ui.StashGoldBtnConfirmYClassic)
	} else {
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnX, ui.StashGoldBtnY)
		utils.PingSleep(utils.Critical, 1000) // Critical operation: Wait for confirm dialog
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnConfirmX, ui.StashGoldBtnConfirmY)
	}
}

func SwitchStashTab(tab int) {
	// Ensure any chat messages that could prevent clicking on the tab are cleared
	ClearMessages()
	utils.Sleep(200)

	ctx := context.Get()
	ctx.SetLastStep("switchTab")

	if ctx.GameReader.LegacyGraphics() {
		x := ui.SwitchStashTabBtnXClassic
		y := ui.SwitchStashTabBtnYClassic

		tabSize := ui.SwitchStashTabBtnTabSizeClassic
		x = x + tabSize*tab - tabSize/2
		ctx.HID.Click(game.LeftButton, x, y)
		utils.PingSleep(utils.Medium, 500) // Medium operation: Wait for tab switch
	} else {
		x := ui.SwitchStashTabBtnX
		y := ui.SwitchStashTabBtnY

		tabSize := ui.SwitchStashTabBtnTabSize
		x = x + tabSize*tab - tabSize/2
		ctx.HID.Click(game.LeftButton, x, y)
		utils.PingSleep(utils.Medium, 500) // Medium operation: Wait for tab switch
	}

}

func OpenStash() error {
	ctx := context.Get()
	ctx.SetLastAction("OpenStash")

	bank, found := ctx.Data.Objects.FindOne(object.Bank)
	if !found {
		return errors.New("stash not found")
	}
	InteractObject(bank,
		func() bool {
			return ctx.Data.OpenMenus.Stash
		},
	)

	return nil
}

func CloseStash() error {
	ctx := context.Get()
	ctx.SetLastAction("CloseStash")

	if ctx.Data.OpenMenus.Stash {
		ctx.HID.PressKey(win.VK_ESCAPE)

	} else {
		return errors.New("stash is not open")
	}

	return nil
}

func TakeItemsFromStash(stashedItems []data.Item) error {
	ctx := context.Get()
	ctx.SetLastAction("TakeItemsFromStash")

	if !ctx.Data.OpenMenus.Stash {
		err := OpenStash()
		if err != nil {
			return err
		}
	}

	utils.PingSleep(utils.Medium, 250) // Medium operation: Wait for stash to open

	for _, i := range stashedItems {

		if i.Location.LocationType != item.LocationStash && i.Location.LocationType != item.LocationSharedStash {
			continue
		}

		// Make sure we're on the correct tab
		SwitchStashTab(i.Location.Page + 1)

		// Move the item to the inventory
		screenPos := ui.GetScreenCoordsForItem(i)
		ctx.HID.MovePointer(screenPos.X, screenPos.Y)
		ctx.HID.ClickWithModifier(game.LeftButton, screenPos.X, screenPos.Y, game.CtrlKey)
		utils.PingSleep(utils.Medium, 500) // Medium operation: Wait for item to move to inventory
	}

	return nil
}

// New function dedicated to upgrading gem corner safety
func EnsureUpgradeGemCornerSafe() {
	ctx := context.Get()
	ctx.Logger.Debug("EnsureUpgradeGemCornerSafe called")

	gemName := ctx.CharacterCfg.Inventory.GemToUpgrade
	if gemName == "None" {
		ctx.Logger.Debug("No gem to upgrade configured, skipping")
		return
	}

	items := ctx.Data.Inventory.ByLocation(item.LocationInventory)
	ctx.Logger.Debug("Scanning inventory for upgrade gem", "gem", gemName, "itemCount", len(items))

	var gem *data.Item
	for i := range items {
		if items[i].Name == item.Name(gemName) && !IsInLockedInventorySlot(items[i]) {
			gem = &items[i]
			ctx.Logger.Debug("Upgrade gem found", "x", gem.Position.X, "y", gem.Position.Y)
			break
		}
	}

	if gem == nil {
		ctx.Logger.Debug("No upgrade gem found or gem in locked slot")
		return
	}

	inv := NewInventoryMask(10, 4)

	// Mark protected slots
	for y := 0; y < 4; y++ {
		for x := 0; x < 10; x++ {
			if ctx.CharacterCfg.Inventory.InventoryLock[y][x] == 0 {
				inv.Grid[y][x] = true
			}
		}
	}
	ctx.Logger.Debug("Protected inventory slots marked in mask")

	// Mark all other items
	for _, it := range items {
		if it.ID == gem.ID {
			continue
		}
		w, h := it.Desc().InventoryWidth, it.Desc().InventoryHeight
		if it.Position.X >= 0 && it.Position.Y >= 0 {
			inv.Place(it.Position.X, it.Position.Y, w, h)
		}
	}
	ctx.Logger.Debug("Inventory mask populated with existing items (excluding gem)")

	// Check if gem is already corner-safe
	if isCornerSafeInMask(gem.Position.X, gem.Position.Y, inv) {
		ctx.Logger.Debug("Upgrade gem already corner-safe", "x", gem.Position.X, "y", gem.Position.Y)
		return
	}

	// Preferred corners
	corners := []data.Position{
		{X: 0, Y: 0}, {X: 9, Y: 0},
		{X: 0, Y: 3}, {X: 9, Y: 3},
	}

	var target *data.Position
	ctx.Logger.Debug("Searching for available corner for upgrade gem")
	for _, c := range corners {
		if inv.CanPlace(c.X, c.Y, gem.Desc().InventoryWidth, gem.Desc().InventoryHeight) {
			ctx.Logger.Debug("Found valid corner for gem", "targetX", c.X, "targetY", c.Y)
			target = &c
			break
		}
	}

	if target == nil {
		ctx.Logger.Debug("No valid corner available for upgrade gem, skipping move")
		return
	}

	// Move gem
	if !ctx.Data.OpenMenus.Inventory {
		ctx.HID.PressKeyBinding(ctx.Data.KeyBindings.Inventory)
		utils.PingSleep(utils.Light, 200)
	}

	ctx.Logger.Debug("Moving upgrade gem to corner",
		"fromX", gem.Position.X, "fromY", gem.Position.Y,
		"toX", target.X, "toY", target.Y,
	)

	screenPos := ui.GetScreenCoordsForItem(*gem)
	ctx.HID.Click(game.LeftButton, screenPos.X, screenPos.Y)
	utils.PingSleep(utils.Light, 200)

	newPos := ui.GetScreenCoordsForInventoryPosition(*target, item.LocationInventory)
	ctx.HID.Click(game.LeftButton, newPos.X, newPos.Y)
	utils.PingSleep(utils.Light, 200)
}

func isCornerSafeInMask(x, y int, inv *InventoryMask) bool {
	blocked := 0

	// Left
	if x == 0 || inv.Grid[y][x-1] {
		blocked++
	}
	// Right
	if x == inv.Width-1 || inv.Grid[y][x+1] {
		blocked++
	}
	// Up
	if y == 0 || inv.Grid[y-1][x] {
		blocked++
	}
	// Down
	if y == inv.Height-1 || inv.Grid[y+1][x] {
		blocked++
	}

	return blocked >= 2
}
