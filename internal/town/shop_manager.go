package town

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/d2go/pkg/nip"

	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
)

var questItems = []item.Name{
	"StaffOfKings",
	"HoradricStaff",
	"AmuletOfTheViper",
	"KhalimsFlail",
	"KhalimsWill",
	"HellforgeHammer",
}

func BuyConsumables(forceRefill bool) {
	ctx := context.Get()

	// --- Track missing consumables ---
	missingHealingPotionBelt := ctx.BeltManager.GetMissingCount(data.HealingPotion)
	missingManaPotionBelt := ctx.BeltManager.GetMissingCount(data.ManaPotion)
	missingHealingPotionInv := ctx.Data.MissingPotionCountInInventory(data.HealingPotion)
	missingManaPotionInv := ctx.Data.MissingPotionCountInInventory(data.ManaPotion)

	// Find best available potions at vendor
	healingPot, healingFound := findFirstMatch("superhealingpotion", "greaterhealingpotion", "healingpotion", "lighthealingpotion", "minorhealingpotion")
	manaPot, manaFound := findFirstMatch("supermanapotion", "greatermanapotion", "manapotion", "lightmanapotion", "minormanapotion")

	ctx.Logger.Debug(fmt.Sprintf("Buying: %d Healing potions and %d Mana potions for belt", missingHealingPotionBelt, missingManaPotionBelt))

	// --- Buy TP Tome if needed ---
	if ShouldBuyTPs() || forceRefill {
		if _, found := ctx.Data.Inventory.Find(item.TomeOfTownPortal, item.LocationInventory); !found && ctx.Data.PlayerUnit.TotalPlayerGold() > 450 {
			ctx.Logger.Info("TP Tome not found, buying one...")
			if itm, itmFound := ctx.Data.Inventory.Find(item.TomeOfTownPortal, item.LocationVendor); itmFound {
				BuyItem(itm, 1)
			}
		}
	}

	// --- Buy potions for belt ---
	if healingFound && missingHealingPotionBelt > 0 {
		BuyItem(healingPot, missingHealingPotionBelt)
	}
	if manaFound && missingManaPotionBelt > 0 {
		BuyItem(manaPot, missingManaPotionBelt)
	}

	ctx.Logger.Debug(fmt.Sprintf("Buying: %d Healing potions and %d Mana potions for inventory", missingHealingPotionInv, missingManaPotionInv))

	// --- Buy potions for inventory ---
	if healingFound && missingHealingPotionInv > 0 {
		BuyItem(healingPot, missingHealingPotionInv)
	}
	if manaFound && missingManaPotionInv > 0 {
		BuyItem(manaPot, missingManaPotionInv)
	}

	// --- Buy Scrolls of TP ---
	if ShouldBuyTPs() || forceRefill {
		ctx.Logger.Debug("Filling TP Tome...")

		// Find the TP tome in inventory
		if tpTome, found := ctx.Data.Inventory.Find(item.TomeOfTownPortal, item.LocationInventory); found {
			// Check current quantity
			qtyStat, found := tpTome.FindStat(stat.Quantity, 0)
			currentQty := 0
			if found {
				currentQty = int(qtyStat.Value)
			}

			// Only buy if below your threshold
			if currentQty < 20 {
				if itm, found := ctx.Data.Inventory.Find(item.ScrollOfTownPortal, item.LocationVendor); found {
					ctx.Logger.Warn("Buying Scrolls of TP to top off Tome...")
					utils.PingSleep(utils.Light, 400)

					if ctx.Data.PlayerUnit.TotalPlayerGold() > 6000 {
						buyFullStack(itm, -1)
					} else {
						BuyItem(itm, 1)
					}
				}
			} else {
				ctx.Logger.Debug("TP Tome already full — skipping purchase")
			}
		} else {
			ctx.Logger.Warn("No TP Tome found in inventory — cannot buy scrolls")
		}
	}

	// --- Determine if IDs are disabled ---
	disableIDs := false
	isLeveling := false
	if ctx.IsLevelingCharacter != nil {
		isLeveling = *ctx.IsLevelingCharacter
	} else {
		isLeveling = ctx.Data.IsLevelingCharacter
	}
	disableIDs = ctx.CharacterCfg.Game.DisableIdentifyTome && !isLeveling

	// --- Buy Scrolls of Identify if IdentifyInField is true OR regular ID purchase ---
	// --- Buy Scrolls of Identify if IdentifyInField is true OR regular ID purchase ---
	if ctx.CharacterCfg.BackToTown.IdentifyInField {
		// Ensure we have a Tome
		if _, found := ctx.Data.Inventory.Find(item.TomeOfIdentify, item.LocationInventory); !found && ctx.Data.PlayerUnit.TotalPlayerGold() > 360 {
			ctx.Logger.Info("ID Tome not found, buying one due to IdentifyInField setting...")
			if itm, itmFound := ctx.Data.Inventory.Find(item.TomeOfIdentify, item.LocationVendor); itmFound {
				BuyItem(itm, 1)
			}
		}

		// Check how many scrolls are in the Tome
		if idTome, found := ctx.Data.Inventory.Find(item.TomeOfIdentify, item.LocationInventory); found {
			qtyStat, found := idTome.FindStat(stat.Quantity, 0)
			currentQty := 0
			if found {
				currentQty = int(qtyStat.Value)
			}

			if currentQty < 20 { // Only buy if we have fewer than 20
				if itm, found := ctx.Data.Inventory.Find(item.ScrollOfIdentify, item.LocationVendor); found {
					ctx.Logger.Warn("Buying Scrolls of Identify to top off Tome...")
					utils.PingSleep(utils.Light, 400)
					buyFullStack(itm, -1)
				}
			} else {
				ctx.Logger.Debug("Tome already has enough Scrolls of Identify — skipping purchase")
			}
		}
	} else if disableIDs {
		ctx.Logger.Debug("DisableIdentifyTome enabled – skipping ID tome/scroll purchases.")
	} else if ShouldBuyIDs() || forceRefill {
		// Regular ID purchase logic
		if _, found := ctx.Data.Inventory.Find(item.TomeOfIdentify, item.LocationInventory); !found && ctx.Data.PlayerUnit.TotalPlayerGold() > 360 {
			ctx.Logger.Info("ID Tome not found, buying one...")
			if itm, itmFound := ctx.Data.Inventory.Find(item.TomeOfIdentify, item.LocationVendor); itmFound {
				BuyItem(itm, 1)
			}
		}

		// Buy Scrolls of Identify based on gold threshold
		if itm, found := ctx.Data.Inventory.Find(item.ScrollOfIdentify, item.LocationVendor); found {
			if ctx.Data.PlayerUnit.TotalPlayerGold() > 16000 {
				buyFullStack(itm, -1)
			} else {
				BuyItem(itm, 1)
			}
		}
	}

	// --- Buy Keys if needed ---
	keyQty, shouldBuyKeys := ShouldBuyKeys()
	if ctx.Data.PlayerUnit.Class != data.Assassin && (shouldBuyKeys || forceRefill) {
		if itm, found := ctx.Data.Inventory.Find(item.Key, item.LocationVendor); found {
			ctx.Logger.Debug("Vendor with keys detected, provisioning...")

			qtyVendor, _ := itm.FindStat(stat.Quantity, 0)
			if qtyVendor.Value > 0 && keyQty < 12 {
				buyFullStack(itm, keyQty)
			}
		}
	}
}

func findFirstMatch(itemNames ...string) (data.Item, bool) {
	ctx := context.Get()
	for _, name := range itemNames {
		if itm, found := ctx.Data.Inventory.Find(item.Name(name), item.LocationVendor); found {
			return itm, true
		}
	}

	return data.Item{}, false
}

func ShouldBuyTPs() bool {
	portalTome, found := context.Get().Data.Inventory.Find(item.TomeOfTownPortal, item.LocationInventory)
	if !found {
		return true
	}

	qty, found := portalTome.FindStat(stat.Quantity, 0)

	return qty.Value < 5 || !found
}

func ShouldBuyIDs() bool {
	ctx := context.Get()

	_, isLevelingChar := ctx.Char.(context.LevelingCharacter)

	// Respect end-game setting: completely disable ID tome purchasing
	if ctx.CharacterCfg.Game.DisableIdentifyTome && !isLevelingChar {
		// Do not buy Tome of Identify nor ID scrolls at all
		ctx.Logger.Debug("DisableIdentifyTome enabled – skipping ID tome/scroll purchases.")
		return false
	}

	// Original behaviour: keep at least 10 IDs in the tome
	idTome, found := ctx.Data.Inventory.Find(item.TomeOfIdentify, item.LocationInventory)
	if !found {
		return true
	}

	qty, found := idTome.FindStat(stat.Quantity, 0)
	return !found || qty.Value < 10
}

func ShouldBuyKeys() (int, bool) {
	// Re-calculating total keys each time ShouldBuyKeys is called for accuracy
	ctx := context.Get()
	totalKeys := 0
	for _, itm := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if itm.Name == item.Key {
			if qty, found := itm.FindStat(stat.Quantity, 0); found {
				totalKeys += qty.Value
			}
		}
	}

	if totalKeys == 0 {
		return 0, true // No keys found, so we should buy
	}

	// We only need to buy if we have less than 12 keys.
	return totalKeys, totalKeys < 12
}

func SellJunk(lockConfig ...[][]int) {
	ctx := context.Get()
	ctx.Logger.Debug("--- SellJunk() function entered ---")
	ctx.Logger.Debug("Selling junk items and excess keys...")

	// --- OPTIMIZED LOGIC FOR SELLING EXCESS KEYS ---
	var allKeyStacks []data.Item
	totalKeys := 0

	// Iterate through ALL items in the inventory to find all key stacks
	// Make sure to re-fetch inventory data before this loop if it hasn't been refreshed recently
	ctx.RefreshGameData() // Crucial to have up-to-date inventory
	for _, itm := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if itm.Name == item.Key {
			if qty, found := itm.FindStat(stat.Quantity, 0); found {
				allKeyStacks = append(allKeyStacks, itm)
				totalKeys += qty.Value
			}
		}
	}

	ctx.Logger.Debug(fmt.Sprintf("Total keys found across all stacks in inventory: %d", totalKeys))

	if totalKeys > 12 {
		excessCount := totalKeys - 12
		ctx.Logger.Info(fmt.Sprintf("Found %d excess keys (total %d). Selling them.", excessCount, totalKeys))

		keysSold := 0

		// Sort key stacks by quantity in descending order to sell larger stacks first
		slices.SortFunc(allKeyStacks, func(a, b data.Item) int {
			qtyA, _ := a.FindStat(stat.Quantity, 0)
			qtyB, _ := b.FindStat(stat.Quantity, 0)
			return qtyB.Value - qtyA.Value // Descending order
		})

		// 1. Sell full stacks until we are close to the target
		stacksToProcess := make([]data.Item, len(allKeyStacks))
		copy(stacksToProcess, allKeyStacks)

		for _, keyStack := range stacksToProcess {
			if keysSold >= excessCount {
				break // We've sold enough
			}

			qtyInStack, found := keyStack.FindStat(stat.Quantity, 0)
			if !found {
				continue
			}

			// If selling this entire stack still leaves us with at least 12 keys
			// Or if this stack exactly equals the remaining excess to sell
			if (totalKeys-qtyInStack.Value >= 12) || (qtyInStack.Value == excessCount-keysSold) {
				ctx.Logger.Debug(fmt.Sprintf("Selling full stack of %d keys from %v", qtyInStack.Value, keyStack.Position))
				SellItemFullStack(keyStack)
				keysSold += qtyInStack.Value
				totalKeys -= qtyInStack.Value     // Update total keys count
				ctx.RefreshGameData()             // Refresh after selling a full stack
				utils.PingSleep(utils.Light, 200) // Light operation: Short delay for UI update
			}
		}

		// Re-evaluate total keys after selling full stacks
		ctx.RefreshGameData()
		totalKeys = 0
		allKeyStacks = []data.Item{} // Clear and re-populate allKeyStacks
		for _, itm := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
			if itm.Name == item.Key {
				if qty, found := itm.FindStat(stat.Quantity, 0); found {
					allKeyStacks = append(allKeyStacks, itm)
					totalKeys += qty.Value
				}
			}
		}

		// 2. If there's still excess, sell individual keys from one of the remaining stacks
		if totalKeys > 12 {
			excessCount = totalKeys - 12 // Recalculate excess after full stack sales
			ctx.Logger.Info(fmt.Sprintf("Still have %d excess keys. Selling individually from a remaining stack.", excessCount))

			// Find *any* remaining key stack to sell from
			var remainingKeyStack data.Item
			for _, itm := range allKeyStacks {
				if itm.Name == item.Key {
					remainingKeyStack = itm
					break
				}
			}

			if remainingKeyStack.Name != "" { // Check if a stack was found
				for i := 0; i < excessCount; i++ {
					SellItem(remainingKeyStack)
					keysSold++
					ctx.RefreshGameData()
					utils.PingSleep(utils.Light, 100) // Light operation: Individual sell delay
				}
			} else {
				ctx.Logger.Warn("No remaining key stacks found to sell individual keys from, despite excess reported.")
			}
		}

		ctx.Logger.Info(fmt.Sprintf("Finished selling excess keys. Keys sold: %d. Estimated remaining: %d", keysSold, totalKeys-keysSold))
	} else {
		ctx.Logger.Debug("No excess keys to sell (12 or less).")
	}
	// --- END OPTIMIZED LOGIC ---

	for _, i := range ItemsToBeSold(lockConfig...) {

		// =========================================================
		// 🔒 PROTECT SPECIFIC MAGIC ITEM (unless duplicate in stash)
		// =========================================================
		if ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint != "" &&
			i.Quality == item.QualityMagic &&
			i.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) &&
			SpecificFingerprint(i) == ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint {

			foundInSharedStash := false

			for _, stashItem := range ctx.Data.Inventory.ByLocation(item.LocationSharedStash) {
				if stashItem.Quality == item.QualityMagic &&
					stashItem.Name == i.Name &&
					SpecificFingerprint(stashItem) == ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint {

					foundInSharedStash = true
					break
				}
			}

			if !foundInSharedStash {
				ctx.Logger.Warn(
					"HARD SKIP SELL: protected specific magic item (no stash duplicate)",
					"unitID", i.UnitID,
				)
				continue
			}

			ctx.Logger.Warn(
				"ALLOW SELL: duplicate specific magic item already in shared stash",
				"unitID", i.UnitID,
			)
		}

		// =========================================================
		// 🔒 PROTECT SPECIFIC RARE ITEM (unless duplicate in stash)
		// =========================================================
		if ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint != "" &&
			i.Quality == item.QualityRare &&
			i.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) &&
			SpecificRareFingerprint(i) == ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint {

			foundInSharedStash := false

			for _, stashItem := range ctx.Data.Inventory.ByLocation(item.LocationSharedStash) {
				if stashItem.Quality == item.QualityRare &&
					stashItem.Name == i.Name &&
					SpecificRareFingerprint(stashItem) == ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint {

					foundInSharedStash = true
					break
				}
			}

			if !foundInSharedStash {
				ctx.Logger.Warn(
					"HARD SKIP SELL: protected specific rare item (no stash duplicate)",
					"unitID", i.UnitID,
				)
				continue
			}

			ctx.Logger.Warn(
				"ALLOW SELL: duplicate specific rare item already in shared stash",
				"unitID", i.UnitID,
			)
		}

		// ---------------------------------------------------------
		// Sell normally
		// ---------------------------------------------------------
		SellItem(i)
	}
}

// SellItem sells a single item by Control-Clicking it.
func SellItem(i data.Item) {
	ctx := context.Get()
	screenPos := ui.GetScreenCoordsForItem(i)

	ctx.Logger.Debug(fmt.Sprintf("Attempting to sell single item %s at screen coords X:%d Y:%d", i.Desc().Name, screenPos.X, screenPos.Y))

	utils.PingSleep(utils.Light, 200) // Light operation: Pre-click delay
	ctx.HID.ClickWithModifier(game.LeftButton, screenPos.X, screenPos.Y, game.CtrlKey)
	utils.PingSleep(utils.Light, 200) // Light operation: Post-click delay
	ctx.Logger.Debug(fmt.Sprintf("Item %s [%s] sold", i.Desc().Name, i.Quality.ToString()))
}

// SellItemFullStack sells an entire stack of items by Ctrl-Clicking it.
func SellItemFullStack(i data.Item) {
	ctx := context.Get()
	screenPos := ui.GetScreenCoordsForItem(i)

	ctx.Logger.Debug(fmt.Sprintf("Attempting to sell full stack of item %s at screen coords X:%d Y:%d", i.Desc().Name, screenPos.X, screenPos.Y))

	utils.PingSleep(utils.Light, 200) // Light operation: Pre-click delay for stack sell
	ctx.HID.ClickWithModifier(game.LeftButton, screenPos.X, screenPos.Y, game.CtrlKey)
	utils.PingSleep(utils.Medium, 500) // Medium operation: Post-click delay for stack sell (longer for confirmation)
	ctx.Logger.Debug(fmt.Sprintf("Full stack of %s [%s] sold", i.Desc().Name, i.Quality.ToString()))
}

func BuyItem(i data.Item, quantity int) {
	ctx := context.Get()
	screenPos := ui.GetScreenCoordsForItem(i)

	utils.PingSleep(utils.Medium, 250) // Medium operation: Pre-buy delay
	for k := 0; k < quantity; k++ {
		ctx.HID.Click(game.RightButton, screenPos.X, screenPos.Y)
		utils.PingSleep(utils.Medium, 600) // Medium operation: Wait for purchase to process
		ctx.Logger.Debug(fmt.Sprintf("Purchased %s [X:%d Y:%d]", i.Desc().Name, i.Position.X, i.Position.Y))
	}
}

// buyFullStack is for buying full stacks of items from a vendor (e.g., potions, scrolls, keys)
// For keys, currentKeysInInventory determines if a special double-click behavior is needed.
func buyFullStack(i data.Item, currentKeysInInventory int) {
	ctx := context.Get()
	screenPos := ui.GetScreenCoordsForItem(i)

	ctx.Logger.Debug(fmt.Sprintf("Attempting to buy full stack of %s from vendor at screen coords X:%d Y:%d", i.Desc().Name, screenPos.X, screenPos.Y))

	// First click: Standard Shift + Right Click for buying a stack from a vendor.
	// As per user's observation:
	// - If 0 keys: this buys 1 key.
	// - If >0 keys: this fills the current stack.
	ctx.HID.ClickWithModifier(game.RightButton, screenPos.X, screenPos.Y, game.ShiftKey)
	utils.PingSleep(utils.Light, 200) // Light operation: Wait for first purchase

	// Special handling for keys: only perform a second click if starting from 0 keys.
	if i.Name == item.Key {
		if currentKeysInInventory == 0 {
			// As per user: if 0 keys, first click buys 1, second click fills the stack.
			ctx.Logger.Debug("Initial keys were 0. Performing second Shift+Right Click to fill key stack.")
			ctx.HID.ClickWithModifier(game.RightButton, screenPos.X, screenPos.Y, game.ShiftKey)
			utils.PingSleep(utils.Light, 200) // Light operation: Wait for second purchase
		} else {
			// As per user: if > 0 keys, the first click should have already filled the stack.
			// No second click is needed to avoid buying an unnecessary extra key/stack.
			ctx.Logger.Debug("Initial keys were > 0. Single Shift+Right Click should have filled stack. No second click needed.")
		}
	}

	ctx.Logger.Debug(fmt.Sprintf("Finished full stack purchase attempt for %s", i.Desc().Name))
}

func ItemsToBeSold(lockConfig ...[][]int) (items []data.Item) {
	ctx := context.Get()
	_, portalTomeFound := ctx.Data.Inventory.Find(item.TomeOfTownPortal, item.LocationInventory)
	healingPotionCountToKeep := ctx.Data.ConfiguredInventoryPotionCount(data.HealingPotion)
	manaPotionCountToKeep := ctx.Data.ConfiguredInventoryPotionCount(data.ManaPotion)
	rejuvPotionCountToKeep := ctx.Data.ConfiguredInventoryPotionCount(data.RejuvenationPotion)

	var currentLockConfig [][]int
	if len(lockConfig) > 0 {
		currentLockConfig = lockConfig[0]
	} else {
		currentLockConfig = ctx.CharacterCfg.Inventory.InventoryLock
	}

	// Count ALL non-NIP jewels (stash + inventory) to determine how many we can keep
	totalNonNIPJewels := 0

	// Count in stash
	for _, stashed := range ctx.Data.Inventory.ByLocation(item.LocationStash, item.LocationSharedStash) {
		if string(stashed.Name) == "Jewel" {
			if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(stashed); res != nip.RuleResultFullMatch {
				totalNonNIPJewels++
			}
		}
	}

	// Count in inventory
	for _, invItem := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if string(invItem.Name) == "Jewel" {
			if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(invItem); res != nip.RuleResultFullMatch {
				totalNonNIPJewels++
			}
		}
	}

	ctx.Logger.Debug(fmt.Sprintf("Total non-NIP jewels (stash + inventory): %d, Configured limit: %d",
		totalNonNIPJewels, ctx.CharacterCfg.CubeRecipes.JewelsToKeep))

	// Determine whether any jewel-using recipes are enabled
	maxJewelsToKeep := ctx.CharacterCfg.CubeRecipes.JewelsToKeep
	craftingEnabled := false
	for _, r := range ctx.CharacterCfg.CubeRecipes.EnabledRecipes {
		if strings.HasPrefix(r, "Caster ") ||
			strings.HasPrefix(r, "Blood ") ||
			strings.HasPrefix(r, "Safety ") ||
			strings.HasPrefix(r, "Hitpower ") {
			craftingEnabled = true
			break
		}
	}

	// Track how many jewels we've decided to keep so far (starting with those in stash)
	jewelsKeptCount := totalNonNIPJewels
	// Now subtract inventory jewels as we'll re-evaluate them below
	for _, invItem := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if string(invItem.Name) == "Jewel" {
			if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(invItem); res != nip.RuleResultFullMatch {
				jewelsKeptCount-- // We'll re-count them as we process
			}
		}
	}

	for _, itm := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		// Check if the item is in a locked slot, and if so, skip it.
		if len(currentLockConfig) > itm.Position.Y && len(currentLockConfig[itm.Position.Y]) > itm.Position.X {
			if currentLockConfig[itm.Position.Y][itm.Position.X] == 0 {
				continue
			}
		}

		isQuestItem := slices.Contains(questItems, itm.Name)
		if itm.IsFromQuest() || isQuestItem {
			continue
		}

		if itm.Name == item.TomeOfTownPortal || itm.Name == item.TomeOfIdentify || itm.Name == item.Key || itm.Name == "WirtsLeg" {
			continue
		}

		//Don't sell scroll of town portal if tome isn't found
		if !portalTomeFound && itm.Name == item.ScrollOfTownPortal {
			continue
		}

		if itm.IsRuneword {
			continue
		}

		if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAllIgnoreTiers(itm); result == nip.RuleResultFullMatch && !itm.IsPotion() {
			continue
		}

		if slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Magic Item") && itm.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) && ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint != "" {
			fp := SpecificFingerprint(itm)
			if fp == ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint {
				for _, it := range ctx.Data.Inventory.ByLocation(item.LocationSharedStash) {
					if it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) &&
						it.Quality == item.QualityMagic &&
						SpecificFingerprint(it) != fp {
						ctx.Logger.Warn("ABSOLUTELY NOT SELLING SPECIFIC MAGIC ITEM BECAUSE IT MATCHES FINGERPRINT")
						continue
					}
				}
			}
		}

		if slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Rare Item") && itm.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) && ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint != "" {
			fp := SpecificRareFingerprint(itm)
			if fp == ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint {
				for _, it := range ctx.Data.Inventory.ByLocation(item.LocationSharedStash) {
					if it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) &&
						it.Quality == item.QualityRare &&
						SpecificRareFingerprint(it) != fp {
						ctx.Logger.Warn("ABSOLUTELY NOT SELLING RARE SPECIFIC MAGIC ITEM BECAUSE IT MATCHES FINGERPRINT")
						continue
					}
				}
			}
		}

		// Handle jewels: keep up to the configured limit of non-NIP jewels
		if craftingEnabled && string(itm.Name) == "Jewel" {
			// Only consider jewels that are not covered by a NIP rule
			if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(itm); res != nip.RuleResultFullMatch {
				if jewelsKeptCount < maxJewelsToKeep {
					jewelsKeptCount++ // Keep this jewel
					ctx.Logger.Debug(fmt.Sprintf("Keeping jewel #%d (under limit of %d)", jewelsKeptCount, maxJewelsToKeep))
					continue
				} else {
					ctx.Logger.Debug(fmt.Sprintf("Selling jewel - already at limit (%d/%d)", jewelsKeptCount, maxJewelsToKeep))
					// This jewel exceeds the limit, so it will be added to items to sell below
				}
			}
		}

		if itm.IsHealingPotion() {
			if healingPotionCountToKeep > 0 {
				healingPotionCountToKeep--
				continue
			}
		}

		if itm.IsManaPotion() {
			if manaPotionCountToKeep > 0 {
				manaPotionCountToKeep--
				continue
			}
		}

		if itm.IsRejuvPotion() {
			if rejuvPotionCountToKeep > 0 {
				rejuvPotionCountToKeep--
				continue
			}
		}

		if itm.Name == "StaminaPotion" && ctx.HealthManager.ShouldKeepStaminaPot() {
			continue
		}

		items = append(items, itm)
	}

	return
}

func SpecificFingerprint(it data.Item) string {
	ctx := context.Get()
	// Defensive guard
	if it.Name != item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) || it.Quality != item.QualityMagic {
		return ""
	}

	var parts []string

	// Base identity: include actual item name for uniqueness
	parts = append(parts, string(it.Name))

	// Identified name (stable across games)
	if it.IdentifiedName != "" {
		parts = append(parts, it.IdentifiedName)
	}

	// Magic affixes (prefixes & suffixes)
	for _, p := range it.Affixes.Magic.Prefixes {
		if p != 0 {
			parts = append(parts, fmt.Sprintf("P%d", p))
		}
	}
	for _, s := range it.Affixes.Magic.Suffixes {
		if s != 0 {
			parts = append(parts, fmt.Sprintf("S%d", s))
		}
	}

	// Serialize stats deterministically
	stats := make([]string, 0, len(it.Stats))
	for _, st := range it.Stats {
		stats = append(stats, fmt.Sprintf(
			"%d:%d:%d",
			st.ID,    // stat ID
			st.Layer, // layer
			st.Value, // value
		))
	}

	// Order-independent
	sort.Strings(stats)

	for _, s := range stats {
		parts = append(parts, s)
	}

	return strings.Join(parts, "|")
}

func SpecificRareFingerprint(it data.Item) string {
	ctx := context.Get()
	// Defensive guard
	if it.Name != item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) || it.Quality != item.QualityRare {
		return ""
	}

	var parts []string

	// Base identity: include actual item name for uniqueness
	parts = append(parts, string(it.Name))

	// Identified name (stable across games)
	if it.IdentifiedName != "" {
		parts = append(parts, it.IdentifiedName)
	}

	// Magic affixes (prefixes & suffixes)
	for _, p := range it.Affixes.Magic.Prefixes {
		if p != 0 {
			parts = append(parts, fmt.Sprintf("P%d", p))
		}
	}
	for _, s := range it.Affixes.Magic.Suffixes {
		if s != 0 {
			parts = append(parts, fmt.Sprintf("S%d", s))
		}
	}

	// Serialize stats deterministically
	stats := make([]string, 0, len(it.Stats))
	for _, st := range it.Stats {
		stats = append(stats, fmt.Sprintf(
			"%d:%d:%d",
			st.ID,    // stat ID
			st.Layer, // layer
			st.Value, // value
		))
	}

	// Order-independent
	sort.Strings(stats)

	for _, s := range stats {
		parts = append(parts, s)
	}

	return strings.Join(parts, "|")
}
