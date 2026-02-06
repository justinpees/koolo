package action

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/difficulty"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/d2go/pkg/nip"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/event"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/pather"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
)

func itemFitsInventory(i data.Item) bool {
	invMatrix := context.Get().Data.Inventory.Matrix()

	for y := 0; y <= len(invMatrix)-i.Desc().InventoryHeight; y++ {
		for x := 0; x <= len(invMatrix[0])-i.Desc().InventoryWidth; x++ {
			freeSpace := true
			for dy := 0; dy < i.Desc().InventoryHeight; dy++ {
				for dx := 0; dx < i.Desc().InventoryWidth; dx++ {
					if invMatrix[y+dy][x+dx] {
						freeSpace = false
						break
					}
				}
				if !freeSpace {
					break
				}
			}

			if freeSpace {
				return true
			}
		}
	}

	return false
}

func itemNeedsInventorySpace(i data.Item) bool {
	// Gold does not occupy grid slots.
	if i.Name == "Gold" {
		return false
	}
	// Potions can go to belt, and we don't want "no grid slot" to trigger town trips/blacklists for them.
	if i.IsPotion() {
		return false
	}
	return true
}

// HasTPsAvailable checks if the player has at least one Town Portal in their tome.
func HasTPsAvailable() bool {
	ctx := context.Get()

	// Check for Tome of Town Portal
	portalTome, found := ctx.Data.Inventory.Find(item.TomeOfTownPortal, item.LocationInventory)
	if !found {
		_, foundScroll := ctx.Data.Inventory.Find(item.ScrollOfTownPortal)
		if foundScroll {
			return true
		}
		return false // No portal tome found at all
	}

	qty, found := portalTome.FindStat(stat.Quantity, 0)
	// Return true only if the quantity stat was found and the value is greater than 0
	return found && qty.Value > 0
}

func ItemPickup(maxDistance int) error {
	ctx := context.Get()
	ctx.SetLastAction("ItemPickup")

	const maxRetries = 5                                        // Base retries for various issues
	const maxItemTooFarAttempts = 5                             // Additional retries specifically for "item too far"
	const totalMaxAttempts = maxRetries + maxItemTooFarAttempts // Combined total attempts
	const debugPickit = false

	// If we're already picking items, skip it
	if ctx.CurrentGame.IsPickingItems {
		return nil
	}

	// Lock items pickup from other sources during the execution of the function
	ctx.SetPickingItems(true)
	defer func() {
		ctx.SetPickingItems(false)
	}()

	// Track how many times we tried to "clean inventory in town" for a specific ground UnitID
	// to avoid infinite town-loops when an item will never fit due to charm layout, etc.
	townCleanupByUnitID := map[data.UnitID]int{}
	consecutiveNoFitTownTrips := 0

outer:
	for {
		ctx.PauseIfNotPriority()

		// 🔄 Refresh full game state (monsters, corpses, objects)
		ctx.RefreshGameData()

		// 🧠 Track monsters for shatter detection
		context.TrackRecentMonsters()

		// Inventory state can drift while moving/clearing. Refresh before deciding what "fits".
		ctx.RefreshInventory()

		itemsToPickup := GetItemsToPickup(maxDistance)
		if len(itemsToPickup) == 0 {
			return nil
		}

		var itemToPickup data.Item

		for _, i := range itemsToPickup {
			// Prefer items that we can actually place.
			if !itemNeedsInventorySpace(i) || itemFitsInventory(i) {
				itemToPickup = i

				if slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Magic Item") {
					MarkGroundSpecificItemIfEligible(itemToPickup)
				}
				if slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Rare Item") {
					MarkGroundSpecificItemIfEligible(itemToPickup)
				}
				break
			}
		}

		if itemToPickup.UnitID == 0 {
			if debugPickit {
				ctx.Logger.Debug("No fitting items found for pickup after filtering.")
			}
			if HasTPsAvailable() {
				consecutiveNoFitTownTrips++
				if consecutiveNoFitTownTrips > 1 {
					// Prevent endless TP-town-TP loops when an item can never fit.
					ctx.Logger.Warn("No fitting items after a town cleanup; stopping pickup cycle to avoid loops.")
					return nil
				}

				if debugPickit {
					ctx.Logger.Debug("TPs available, returning to town to sell junk and stash items.")
				}
				if err := InRunReturnTownRoutine(); err != nil {
					ctx.Logger.Warn("Failed returning to town from ItemPickup", "error", err)
				}
				continue
			}

			ctx.Logger.Warn("Inventory is full and NO Town Portals found. Skipping return to town and continuing current run (no more item pickups this cycle).")
			return nil
		}

		consecutiveNoFitTownTrips = 0

		if debugPickit {
			ctx.Logger.Info(fmt.Sprintf(
				"Attempting to pickup item: %s [%d] at X:%d Y:%d",
				itemToPickup.Name,
				itemToPickup.Quality,
				itemToPickup.Position.X,
				itemToPickup.Position.Y,
			))
		}

		// Try to pick up the item with retries
		var lastError error
		attempt := 1
		itemTooFarRetryCount := 0     // Tracks retries specifically for "item too far"
		totalAttemptCounter := 0      // Overall attempts
		var consecutiveMoveErrors int // Track consecutive ErrCastingMoving errors
		pickedUp := false

		for totalAttemptCounter < totalMaxAttempts {
			totalAttemptCounter++
			if debugPickit {
				ctx.Logger.Debug(fmt.Sprintf("Item Pickup: Starting attempt %d (total: %d)", attempt, totalAttemptCounter))
			}

			// If inventory changed and item no longer fits, do NOT grind attempts and then blacklist.
			// Instead: go to town (stash/sell), come back and retry.
			if itemNeedsInventorySpace(itemToPickup) {
				ctx.RefreshInventory()
				if !itemFitsInventory(itemToPickup) {
					if HasTPsAvailable() {
						townCleanupByUnitID[itemToPickup.UnitID]++
						if townCleanupByUnitID[itemToPickup.UnitID] <= 1 {
							ctx.Logger.Debug("Item doesn't fit in inventory right now; returning to town to stash/sell and retry.",
								slog.String("itemName", string(itemToPickup.Desc().Name)),
								slog.Int("unitID", int(itemToPickup.UnitID)),
							)
							if err := InRunReturnTownRoutine(); err != nil {
								ctx.Logger.Warn("Failed returning to town from ItemPickup", "error", err)
							}
							continue outer
						}
						// Already tried town once and it still doesn't fit: blacklist this ground instance to stop thrashing.
						lastError = fmt.Errorf("item does not fit in inventory even after town cleanup")
						break
					}
					ctx.Logger.Warn("Inventory full and NO Town Portals found. Skipping further item pickups this cycle.")
					return nil
				}
			}

			pickupStartTime := time.Now()

			// Clear monsters on each attempt
			if debugPickit {
				ctx.Logger.Debug(fmt.Sprintf("Item Pickup: Clearing area around item. Attempt %d", attempt))
			}
			ClearAreaAroundPlayer(4, data.MonsterAnyFilter())
			ClearAreaAroundPosition(itemToPickup.Position, 4, data.MonsterAnyFilter())
			if debugPickit {
				ctx.Logger.Debug(fmt.Sprintf("Item Pickup: Area cleared in %v. Attempt %d", time.Since(pickupStartTime), attempt))
			}

			// Calculate position to move to based on attempt number
			pickupPosition := itemToPickup.Position
			moveDistance := 3
			if attempt > 1 {
				switch attempt {
				case 2:
					pickupPosition = data.Position{X: itemToPickup.Position.X + moveDistance, Y: itemToPickup.Position.Y - 1}
				case 3:
					pickupPosition = data.Position{X: itemToPickup.Position.X - moveDistance, Y: itemToPickup.Position.Y + 1}
				case 4:
					pickupPosition = data.Position{X: itemToPickup.Position.X + moveDistance + 2, Y: itemToPickup.Position.Y - 3}
				case 5:
					MoveToCoords(ctx.PathFinder.BeyondPosition(ctx.Data.PlayerUnit.Position, itemToPickup.Position, 4), step.WithIgnoreItems())
				}
			}

			distance := ctx.PathFinder.DistanceFromMe(itemToPickup.Position)
			if distance >= 7 || attempt > 1 {
				distanceToFinish := max(4-attempt, 2)
				if debugPickit {
					ctx.Logger.Debug(fmt.Sprintf("Item Pickup: Moving to coordinates X:%d Y:%d (distance: %d, distToFinish: %d). Attempt %d", pickupPosition.X, pickupPosition.Y, distance, distanceToFinish, attempt))
				}
				if err := MoveToCoords(pickupPosition, step.WithDistanceToFinish(distanceToFinish), step.WithIgnoreItems()); err != nil {
					lastError = err
					continue
				}
				if debugPickit {
					ctx.Logger.Debug(fmt.Sprintf("Item Pickup: Move completed in %v. Attempt %d", time.Since(pickupStartTime), attempt))
				}
			}

			// Try to pick up the item
			pickupActionStartTime := time.Now()
			if debugPickit {
				ctx.Logger.Debug(fmt.Sprintf("Item Pickup: Initiating PickupItem action. Attempt %d", attempt))
			}

			err := step.PickupItem(itemToPickup, attempt)
			if err == nil {
				pickedUp = true
				lastError = nil

				if debugPickit {
					ctx.Logger.Info(fmt.Sprintf("Successfully picked up item: %s [%d] in %v. Total attempts: %d", itemToPickup.Name, itemToPickup.Quality, time.Since(pickupActionStartTime), totalAttemptCounter))
				}

				onAnniPickedUp(itemToPickup)

				// ✅ If we marked the specific item before pickup, identify it now
				if ctx.MarkedSpecificItemUnitID != 0 && ctx.MarkedSpecificItemUnitID == itemToPickup.UnitID {

					ctx.RefreshInventory() // make sure item is in inventory

					// Find the item in inventory
					var specificItemInInv data.Item
					found := false
					for _, invItem := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
						if invItem.UnitID == ctx.MarkedSpecificItemUnitID {
							specificItemInInv = invItem
							found = true
							break
						}
					}

					if !found {
						ctx.Logger.Error("Picked up Specific Item but cannot find it in inventory", "unitID", ctx.MarkedSpecificItemUnitID)
					} else {
						// Find Tome of Identify
						idTome, found := ctx.Data.Inventory.Find(item.TomeOfIdentify, item.LocationInventory)
						if !found {
							ctx.Logger.Warn("Tome of Identify not found, skipping identification")
						} else {
							step.CloseAllMenus() // make sure nothing is in the way
							for !ctx.Data.OpenMenus.Inventory {
								ctx.HID.PressKeyBinding(ctx.Data.KeyBindings.Inventory)
								utils.PingSleep(utils.Critical, 1000)
							}
							identifySpecificMarkedItem(idTome, specificItemInInv)
							step.CloseAllMenus()
							ctx.RefreshInventory()
							ctx.Logger.Warn("Specific Item successfully identified, closed all menus")

						}
					}
				}

				if ctx.MarkedRareSpecificItemUnitID != 0 && ctx.MarkedRareSpecificItemUnitID == itemToPickup.UnitID {

					ctx.RefreshInventory() // make sure item is in inventory

					// Find the item in inventory
					var specificItemInInv data.Item
					found := false
					for _, invItem := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
						if invItem.UnitID == ctx.MarkedRareSpecificItemUnitID {
							specificItemInInv = invItem
							found = true
							break
						}
					}

					if !found {
						ctx.Logger.Error("Picked up Rare Specific Item but cannot find it in inventory", "unitID", ctx.MarkedRareSpecificItemUnitID)
					} else {
						// Find Tome of Identify
						idTome, found := ctx.Data.Inventory.Find(item.TomeOfIdentify, item.LocationInventory)
						if !found {
							ctx.Logger.Warn("Tome of Identify not found, skipping identification")
						} else {
							step.CloseAllMenus() // make sure nothing is in the way
							for !ctx.Data.OpenMenus.Inventory {
								ctx.HID.PressKeyBinding(ctx.Data.KeyBindings.Inventory)
								utils.PingSleep(utils.Critical, 1000)
							}
							identifyRareSpecificMarkedItem(idTome, specificItemInInv)
							step.CloseAllMenus()
							ctx.RefreshInventory()
							ctx.Logger.Warn("Specific RARE Item successfully identified, closed all menus")

						}
					}
				}

				break
			}

			lastError = err
			if debugPickit {
				ctx.Logger.Warn(fmt.Sprintf("Item Pickup: Pickup attempt %d failed: %v", attempt, err), slog.String("itemName", string(itemToPickup.Name)))
			}

			// If the pickup failed and the item doesn't fit *right now*, don't blacklist it.
			// This is the exact scenario where we should go stash/sell and retry.
			if itemNeedsInventorySpace(itemToPickup) {
				ctx.RefreshInventory()
				if !itemFitsInventory(itemToPickup) {
					if HasTPsAvailable() {
						townCleanupByUnitID[itemToPickup.UnitID]++
						if townCleanupByUnitID[itemToPickup.UnitID] <= 1 {
							ctx.Logger.Debug("Pickup failed and item no longer fits; returning to town to stash/sell and retry.",
								slog.String("itemName", string(itemToPickup.Desc().Name)),
								slog.Int("unitID", int(itemToPickup.UnitID)),
							)
							if errTown := InRunReturnTownRoutine(); errTown != nil {
								ctx.Logger.Warn("Failed returning to town from ItemPickup", "error", errTown)
							}
							continue outer
						}
						lastError = fmt.Errorf("item does not fit in inventory even after town cleanup: %w", err)
						break
					}
					ctx.Logger.Warn("Inventory full and NO Town Portals found. Skipping further item pickups this cycle.")
					return nil
				}
			}

			// Movement-state handling
			if errors.Is(err, step.ErrCastingMoving) {
				consecutiveMoveErrors++
				if consecutiveMoveErrors > 3 {
					lastError = fmt.Errorf("failed to pick up item after multiple attempts due to movement state: %w", err)
					break
				}
				time.Sleep(time.Millisecond * time.Duration(utils.PingMultiplier(utils.Light, 100)))
				continue
			}

			if errors.Is(err, step.ErrMonsterAroundItem) {
				continue
			}

			// Item too far retry logic
			if errors.Is(err, step.ErrItemTooFar) {
				itemTooFarRetryCount++
				if debugPickit {
					ctx.Logger.Debug(fmt.Sprintf("Item Pickup: Item too far detected. ItemTooFar specific retry %d/%d.", itemTooFarRetryCount, maxItemTooFarAttempts))
				}
				if itemTooFarRetryCount < maxItemTooFarAttempts {
					ctx.PathFinder.RandomMovement()
					continue
				}
			}

			if errors.Is(err, step.ErrNoLOSToItem) {
				if debugPickit {
					ctx.Logger.Debug("Item Pickup: No line of sight to item, moving closer",
						slog.String("item", string(itemToPickup.Desc().Name)))
				}
				beyondPos := ctx.PathFinder.BeyondPosition(ctx.Data.PlayerUnit.Position, itemToPickup.Position, 2+attempt)
				if mvErr := MoveToCoords(beyondPos, step.WithIgnoreItems()); mvErr == nil {
					err = step.PickupItem(itemToPickup, attempt)
					if err == nil {
						pickedUp = true
						lastError = nil

						onAnniPickedUp(itemToPickup)

						if debugPickit {
							ctx.Logger.Info(fmt.Sprintf("Successfully picked up item after LOS correction: %s [%d] in %v. Total attempts: %d", itemToPickup.Name, itemToPickup.Quality, time.Since(pickupActionStartTime), totalAttemptCounter))
						}
						break
					}
					lastError = err
				} else {
					lastError = mvErr
				}
			}

			attempt++
		}

		if pickedUp {
			continue
		}

		// Final guard: if it doesn't fit at the end, prefer a town cleanup over blacklisting.
		if lastError != nil && itemNeedsInventorySpace(itemToPickup) {
			ctx.RefreshInventory()
			if !itemFitsInventory(itemToPickup) {
				if HasTPsAvailable() {
					townCleanupByUnitID[itemToPickup.UnitID]++
					if townCleanupByUnitID[itemToPickup.UnitID] <= 1 {
						if err := InRunReturnTownRoutine(); err != nil {
							ctx.Logger.Warn("Failed returning to town from ItemPickup", "error", err)
						}
						continue
					}
					// Still doesn't fit after town: fall through to blacklist this UnitID.
				} else {
					return nil
				}
			}
		}

		// If all attempts failed, blacklist *this specific ground instance* (UnitID), not the whole base item ID.
		if totalAttemptCounter >= totalMaxAttempts && lastError != nil {
			if !IsBlacklisted(itemToPickup) {
				ctx.CurrentGame.BlacklistedItems = append(ctx.CurrentGame.BlacklistedItems, itemToPickup)
			}

			// Screenshot with show items on
			ctx.HID.KeyDown(ctx.Data.KeyBindings.ShowItems)
			time.Sleep(time.Millisecond * time.Duration(utils.PingMultiplier(utils.Light, 200)))
			screenshot := ctx.GameReader.Screenshot()
			event.Send(event.ItemBlackListed(event.WithScreenshot(ctx.Name, fmt.Sprintf("Item %s [%s] BlackListed in Area:%s", itemToPickup.Name, itemToPickup.Quality.ToString(), ctx.Data.PlayerUnit.Area.Area().Name), screenshot), data.Drop{Item: itemToPickup}))
			ctx.HID.KeyUp(ctx.Data.KeyBindings.ShowItems)

			ctx.Logger.Warn(
				"Failed picking up item after all attempts, blacklisting it",
				slog.String("itemName", string(itemToPickup.Desc().Name)),
				slog.Int("unitID", int(itemToPickup.UnitID)),
				slog.String("lastError", lastError.Error()),
				slog.Int("totalAttempts", totalAttemptCounter),
			)
		}
	}
}

func GetItemsToPickup(maxDistance int) []data.Item {
	ctx := context.Get()
	ctx.SetLastAction("GetItemsToPickup")

	missingHealingPotions := ctx.BeltManager.GetMissingCount(data.HealingPotion) + ctx.Data.MissingPotionCountInInventory(data.HealingPotion)
	missingManaPotions := ctx.BeltManager.GetMissingCount(data.ManaPotion) + ctx.Data.MissingPotionCountInInventory(data.ManaPotion)
	missingRejuvenationPotions := ctx.BeltManager.GetMissingCount(data.RejuvenationPotion) + ctx.Data.MissingPotionCountInInventory(data.RejuvenationPotion)

	var itemsToPickup []data.Item
	_, isLevelingChar := ctx.Char.(context.LevelingCharacter)

	for _, itm := range ctx.Data.Inventory.ByLocation(item.LocationGround) {
		// Skip itempickup on party leveling Maggot Lair, is too narrow and causes characters to get stuck
		if isLevelingChar && itm.Name != "StaffOfKings" && (ctx.Data.PlayerUnit.Area == area.MaggotLairLevel1 ||
			ctx.Data.PlayerUnit.Area == area.MaggotLairLevel2 ||
			ctx.Data.PlayerUnit.Area == area.MaggotLairLevel3 ||
			ctx.Data.PlayerUnit.Area == area.ArcaneSanctuary) {
			continue
		}

		if ctx.Data.PlayerUnit.Area != area.Tristram && itm.Name == "WirtsLeg" {
			//ctx.Logger.Warn("Not picking up that trash wirts leg")
			continue
		}

		if itm.Quality == item.QualityUnique && itm.Name == "SmallCharm" {
			if !ctx.CharacterCfg.Inventory.AllowAnniPickup {
				ctx.Logger.Warn("Unique small charm detected (prep phase)")
				if err := HandleSmallCharmOnFloor(itm); err != nil {
					ctx.Logger.Error("Failed Anni prep", "err", err)
				}
				continue
			}

			// Allow normal pickup
			ctx.Logger.Warn("Unique small charm pickup allowed")
		}

		// Skip potion pickup for Berserker Barb in Travincal if configured
		if ctx.CharacterCfg.Character.Class == "berserker" &&
			ctx.CharacterCfg.Character.BerserkerBarb.SkipPotionPickupInTravincal &&
			ctx.Data.PlayerUnit.Area == area.Travincal &&
			itm.IsPotion() {
			continue
		}

		// Skip potion pickup for Warcry Barb in Travincal if configured
		if ctx.CharacterCfg.Character.Class == "warcry_barb" &&
			ctx.CharacterCfg.Character.WarcryBarb.SkipPotionPickupInTravincal &&
			ctx.Data.PlayerUnit.Area == area.Travincal &&
			itm.IsPotion() {
			continue
		}

		// Skip items that are outside pickup radius, this is useful when clearing big areas to prevent
		// character going back to pickup potions all the time after using them
		itemDistance := ctx.PathFinder.DistanceFromMe(itm.Position)
		if maxDistance > 0 && itemDistance > maxDistance && itm.IsPotion() {
			continue
		}

		if itm.IsPotion() {
			if (itm.IsHealingPotion() && missingHealingPotions > 0) ||
				(itm.IsManaPotion() && missingManaPotions > 0) ||
				(itm.IsRejuvPotion() && missingRejuvenationPotions > 0) {
				if shouldBePickedUp(itm) {
					itemsToPickup = append(itemsToPickup, itm)
					switch {
					case itm.IsHealingPotion():
						missingHealingPotions--
					case itm.IsManaPotion():
						missingManaPotions--
					case itm.IsRejuvPotion():
						missingRejuvenationPotions--
					}
				}
			}
		} else if shouldBePickedUp(itm) {
			itemsToPickup = append(itemsToPickup, itm)
		}
	}

	// Remove blacklisted items from the list, we don't want to pick them up
	filteredItems := make([]data.Item, 0, len(itemsToPickup))
	for _, itm := range itemsToPickup {
		isBlacklisted := IsBlacklisted(itm)
		if !isBlacklisted {
			filteredItems = append(filteredItems, itm)
		}
	}

	return filteredItems
}

func shouldBePickedUp(i data.Item) bool {
	ctx := context.Get()
	ctx.SetLastAction("shouldBePickedUp")

	// Always pick up runewords and Wirt's Leg.
	if i.IsRuneword || i.Name == "WirtsLeg" {
		return true
	}

	// Pick up quest items if in a leveling or questing run.
	specialRuns := slices.Contains(ctx.CharacterCfg.Game.Runs, "quests") || slices.Contains(ctx.CharacterCfg.Game.Runs, "leveling") || slices.Contains(ctx.CharacterCfg.Game.Runs, "leveling_sequence")
	if specialRuns {
		switch i.Name {
		case "Scroll of Inifuss", "ScrollOfInifuss", "LamEsensTome", "HoradricCube", "HoradricMalus",
			"AmuletoftheViper", "StaffofKings", "HoradricStaff",
			"AJadeFigurine", "KhalimsEye", "KhalimsBrain", "KhalimsHeart", "KhalimsFlail", "HellforgeHammer", "TheGidbinn":
			return true
		}
	}

	// Specific ID checks (e.g. Book of Skill and Scroll of Inifuss).
	if i.ID == 552 || i.ID == 524 {
		return true
	}

	// Skip picking up gold if inventory is full of gold.
	gold, _ := ctx.Data.PlayerUnit.FindStat(stat.Gold, 0)
	if gold.Value >= ctx.Data.PlayerUnit.MaxGold() && i.Name == "Gold" {
		ctx.Logger.Debug("Skipping gold pickup, inventory full")
		return false
	}

	// In leveling runs, pick up any non‑gold item if very low on gold.
	_, isLevelingChar := ctx.Char.(context.LevelingCharacter)
	if isLevelingChar && IsLowGold() && i.Name != "Gold" {
		return true
	}

	// Pick up stamina potions only when needed in leveling runs.
	if isLevelingChar && i.Name == "StaminaPotion" {
		if ctx.HealthManager.ShouldPickStaminaPot() {
			return true
		}
	}

	// If total gold is below the minimum threshold, pick up magic and better items for selling.
	minGoldPickupThreshold := ctx.CharacterCfg.Game.MinGoldPickupThreshold
	if ctx.Data.PlayerUnit.TotalPlayerGold() < minGoldPickupThreshold && i.Quality >= item.QualityMagic {
		return true
	}

	// After all heuristics, defer to strict pickit/tier evaluation.
	// This function encapsulates the final rule logic (tiers and NIP) and
	// handles quantity blacklisting without re‑implementing it here.
	return shouldMatchRulesOnly(i)
}

func IsBlacklisted(itm data.Item) bool {
	for _, blacklisted := range context.Get().CurrentGame.BlacklistedItems {
		// Blacklist is per-game. UnitID is the safest key: it targets only the problematic ground instance.
		if itm.UnitID == blacklisted.UnitID {
			return true
		}
	}
	return false
}

func IsChestOrContainer(name object.Name) bool {
	switch name {
	// Basic chests
	case object.JungleChest,
		object.MediumChestLeft,
		object.LargeChestLeft2,
		object.LargeChestRight,
		object.LargeChestLeft,
		object.TallChestLeft,

		// Act‑specific chests
		object.Act1LargeChestRight,
		object.Act1TallChestRight,
		object.Act1MediumChestRight,
		object.Act2MediumChestRight,
		object.Act2LargeChestRight,
		object.Act2LargeChestLeft,
		object.MafistoLargeChestLeft,
		object.MafistoLargeChestRight,
		object.MafistoMediumChestLeft,
		object.MafistoMediumChestRight,
		object.SpiderLairLargeChestLeft,
		object.SpiderLairTallChestLeft,
		object.SpiderLairMediumChestRight,
		object.SpiderLairTallChestRight,

		// Special & quest chests
		object.HoradricCubeChest,
		object.HoradricScrollChest,
		object.StaffOfKingsChest,
		object.SparklyChest,
		object.KhalimChest1,
		object.KhalimChest2,
		object.KhalimChest3,
		object.GLchest3L,
		object.Gchest1L,
		object.Gchest2R,
		object.Gchest3R,

		// “Stash” typed objects
		object.JungleStashObject1,
		object.JungleStashObject2,
		object.JungleStashObject3,
		object.JungleStashObject4,
		object.StashBox,
		object.StashAltar,

		// Loose containers and small loot spawners
		object.Crate,
		object.Barrel,
		object.BarrelExploding:

		return true
	default:
		return false
	}
}
func ClosestCorpseID(itemPos data.Position, corpses []data.Monster) npc.ID {
	var nearestCorpse *data.Monster
	minDistance := 9999 // int

	for i := range corpses {
		corpse := &corpses[i]
		dist := pather.DistanceFromPoint(corpse.Position, itemPos)
		if dist < minDistance {
			minDistance = dist
			nearestCorpse = corpse
		}
	}

	if nearestCorpse != nil {
		return nearestCorpse.Name // this is npc.ID
	}
	return 0 // or npc.None if you have that constant
}

// markSpecificItemIfEligible checks if an item should be marked for rerolling
// according to the configured SpecificItemToReroll and the area monster level range.
func MarkGroundSpecificItemIfEligible(i data.Item) {
	ctx := context.Get()

	monsterTypeName := func(t data.MonsterType) string {
		switch t {
		case data.MonsterTypeMinion:
			return "Minion"
		case data.MonsterTypeNone:
			return "Normal"
		case data.MonsterTypeChampion:
			return "Champion"
		case data.MonsterTypeUnique:
			return "Unique"
		case data.MonsterTypeSuperUnique:
			return "SuperUnique"
		default:
			return fmt.Sprintf("Unknown(%d)", t)
		}
	}

	// Already tracking one specific item — do not overwrite
	if ctx.MarkedSpecificItemUnitID != 0 ||
		ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint != "" {
		return
	}

	// Only pick up configured specific item
	if i.Name != item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) ||
		i.Quality != item.QualityMagic {
		return
	}

	areaID := ctx.Data.PlayerUnit.Area
	isTerror := slices.Contains(ctx.Data.TerrorZones, areaID)
	var areaMLvl int

	// --- Find nearest corpse ---
	const maxCorpseDistance = 50
	nearestCorpseID := ClosestCorpseID(i.Position, ctx.Data.Corpses)
	var nearestCorpse *data.Monster
	minCorpseDistance := 9999

	for j := range ctx.Data.Corpses {
		corpse := &ctx.Data.Corpses[j]
		dist := pather.DistanceFromPoint(corpse.Position, i.Position)
		if dist > maxCorpseDistance {
			continue
		}
		if corpse.Name == nearestCorpseID && dist < minCorpseDistance {
			minCorpseDistance = dist
			nearestCorpse = corpse
		}
	}

	// --- Find nearest shattered monster ---
	const maxShatterDistance = 50
	var nearestShattered *context.RecentlySeenMonster
	minShatterDistance := 9999

	for idx := range context.RecentMonsters {
		rm := &context.RecentMonsters[idx]
		dist := pather.DistanceFromPoint(rm.Position, i.Position)
		if dist <= maxShatterDistance && dist < minShatterDistance {
			minShatterDistance = dist
			nearestShattered = rm
		}
	}

	// --- Find nearest chest ---
	var nearestChest *data.Object
	minChestDistance := 9999
	for k := range ctx.Data.Objects {
		obj := &ctx.Data.Objects[k]
		if !IsChestOrContainer(obj.Name) {
			continue
		}
		dist := pather.DistanceFromPoint(obj.Position, i.Position)
		if dist <= 50 && dist < minChestDistance {
			minChestDistance = dist
			nearestChest = obj
		}
	}

	// --- LOG distances ---
	if nearestCorpse != nil {
		ctx.Logger.Warn("Nearest corpse found",
			"corpseID", nearestCorpse.Name,
			"corpseType", monsterTypeName(nearestCorpse.Type),
			"distance", minCorpseDistance,
		)
	}
	if nearestShattered != nil {
		ctx.Logger.Warn("Nearest shattered monster found",
			"monsterID", nearestShattered.Name,
			"monsterType", monsterTypeName(nearestShattered.Type),
			"distance", minShatterDistance,
		)
	}
	if nearestChest != nil {
		ctx.Logger.Warn("Nearest chest found within 50 units",
			"chestName", nearestChest.Name,
			"distance", minChestDistance,
		)
	}

	// --- Tie-break logic for corpse vs shattered ---
	if nearestCorpse != nil && nearestShattered != nil &&
		minCorpseDistance == minShatterDistance {

		var corpseMLvl, shatterMLvl int
		var ok1, ok2 bool

		if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestCorpse.Name)]; ok {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				corpseMLvl = table[0]
			case difficulty.Nightmare:
				corpseMLvl = table[1]
			case difficulty.Hell:
				corpseMLvl = table[2]
			}
			switch nearestCorpse.Type {
			case data.MonsterTypeChampion:
				corpseMLvl += 2
			case data.MonsterTypeUnique:
				corpseMLvl += 3
			}
			ok1 = true
		}

		if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestShattered.Name)]; ok {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				shatterMLvl = table[0]
			case difficulty.Nightmare:
				shatterMLvl = table[1]
			case difficulty.Hell:
				shatterMLvl = table[2]
			}
			switch nearestShattered.Type {
			case data.MonsterTypeChampion:
				shatterMLvl += 2
			case data.MonsterTypeUnique:
				shatterMLvl += 3
			}
			ok2 = true
		}

		if ok1 && ok2 && corpseMLvl != shatterMLvl {
			minMLvl := ctx.CharacterCfg.CubeRecipes.MinMonsterLevel
			maxMLvl := ctx.CharacterCfg.CubeRecipes.MaxMonsterLevel

			corpseValid := corpseMLvl >= minMLvl && corpseMLvl <= maxMLvl
			shatterValid := shatterMLvl >= minMLvl && shatterMLvl <= maxMLvl

			if !corpseValid || !shatterValid {
				return
			}
		}
	}

	// --- Tie-break logic: corpse vs chest ---
	if nearestCorpse != nil && nearestChest != nil &&
		minCorpseDistance == minChestDistance {

		var corpseMLvl, chestMLvl int
		var okCorpse, okChest bool

		if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestCorpse.Name)]; ok {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				corpseMLvl = table[0]
			case difficulty.Nightmare:
				corpseMLvl = table[1]
			case difficulty.Hell:
				corpseMLvl = table[2]
			}
			switch nearestCorpse.Type {
			case data.MonsterTypeChampion:
				corpseMLvl += 2
			case data.MonsterTypeUnique:
				corpseMLvl += 3
			}
			okCorpse = true
		}

		if mlvls, exists := game.AreaLevelTable[areaID]; exists {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				chestMLvl = mlvls[0]
			case difficulty.Nightmare:
				chestMLvl = mlvls[1]
			case difficulty.Hell:
				chestMLvl = mlvls[2]
			}
			okChest = true
		}

		if okCorpse && okChest && corpseMLvl != chestMLvl {
			minMLvl := ctx.CharacterCfg.CubeRecipes.MinMonsterLevel
			maxMLvl := ctx.CharacterCfg.CubeRecipes.MaxMonsterLevel

			if corpseMLvl < minMLvl || corpseMLvl > maxMLvl ||
				chestMLvl < minMLvl || chestMLvl > maxMLvl {
				return
			}
		}
	}

	// --- Tie-break logic: shattered vs chest ---
	if nearestShattered != nil && nearestChest != nil &&
		minShatterDistance == minChestDistance {

		var shatterMLvl, chestMLvl int
		var okShatter, okChest bool

		if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestShattered.Name)]; ok {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				shatterMLvl = table[0]
			case difficulty.Nightmare:
				shatterMLvl = table[1]
			case difficulty.Hell:
				shatterMLvl = table[2]
			}
			switch nearestShattered.Type {
			case data.MonsterTypeChampion:
				shatterMLvl += 2
			case data.MonsterTypeUnique:
				shatterMLvl += 3
			}
			okShatter = true
		}

		if mlvls, exists := game.AreaLevelTable[areaID]; exists {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				chestMLvl = mlvls[0]
			case difficulty.Nightmare:
				chestMLvl = mlvls[1]
			case difficulty.Hell:
				chestMLvl = mlvls[2]
			}
			okChest = true
		}

		if okShatter && okChest && shatterMLvl != chestMLvl {
			minMLvl := ctx.CharacterCfg.CubeRecipes.MinMonsterLevel
			maxMLvl := ctx.CharacterCfg.CubeRecipes.MaxMonsterLevel

			if shatterMLvl < minMLvl || shatterMLvl > maxMLvl ||
				chestMLvl < minMLvl || chestMLvl > maxMLvl {
				return
			}
		}
	}

	// --- Decide closest source ---
	type sourceKind int
	const (
		sourceNone sourceKind = iota
		sourceCorpse
		sourceShattered
		sourceChest
	)

	// 👇 ADD THIS RIGHT HERE
	sourceToString := func(s sourceKind) string {
		switch s {
		case sourceCorpse:
			return "corpse"
		case sourceShattered:
			return "shattered"
		case sourceChest:
			return "chest"
		default:
			return "none"
		}
	}

	chosenSource := sourceNone
	minDist := 9999

	if nearestCorpse != nil && minCorpseDistance < minDist {
		chosenSource = sourceCorpse
		minDist = minCorpseDistance
	}
	if nearestShattered != nil && minShatterDistance < minDist {
		chosenSource = sourceShattered
		minDist = minShatterDistance
	}
	if nearestChest != nil && minChestDistance < minDist {
		chosenSource = sourceChest
		minDist = minChestDistance
	}

	// --- Terror zone MLVL helper ---
	calcTerrorMLvl := func(clvl int, mType data.MonsterType, diff difficulty.Difficulty) int {
		var base int
		switch mType {
		case data.MonsterTypeNone:
			base = clvl + 2
		case data.MonsterTypeChampion:
			base = clvl + 4
		case data.MonsterTypeUnique, data.MonsterTypeSuperUnique:
			base = clvl + 5
		default:
			base = clvl + 2
		}

		switch diff {
		case difficulty.Normal:
			if base > 45 {
				base = 45
			}
		case difficulty.Nightmare:
			if base > 71 {
				base = 71
			}
		case difficulty.Hell:
			if base > 96 {
				base = 96
			}
		}
		return base
	}

	// --- Apply MLVL ---
	if isTerror {
		if clvlStat, ok := ctx.Data.PlayerUnit.FindStat(stat.Level, 0); ok {
			clvl := clvlStat.Value
			switch chosenSource {
			case sourceCorpse:
				areaMLvl = calcTerrorMLvl(clvl, nearestCorpse.Type, ctx.CharacterCfg.Game.Difficulty)
			case sourceShattered:
				areaMLvl = calcTerrorMLvl(clvl, nearestShattered.Type, ctx.CharacterCfg.Game.Difficulty)
			case sourceChest:
				areaMLvl = calcTerrorMLvl(clvl, data.MonsterTypeNone, ctx.CharacterCfg.Game.Difficulty)
			default:
				areaMLvl = calcTerrorMLvl(clvl, data.MonsterTypeNone, ctx.CharacterCfg.Game.Difficulty)
			}
		} else {
			return
		}
	} else {
		switch chosenSource {
		case sourceCorpse:
			if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestCorpse.Name)]; ok {
				switch ctx.CharacterCfg.Game.Difficulty {
				case difficulty.Normal:
					areaMLvl = table[0]
				case difficulty.Nightmare:
					areaMLvl = table[1]
				case difficulty.Hell:
					areaMLvl = table[2]
				}
				switch nearestCorpse.Type {
				case data.MonsterTypeChampion:
					areaMLvl += 2
				case data.MonsterTypeUnique:
					areaMLvl += 3
				}
			}
		case sourceShattered:
			if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestShattered.Name)]; ok {
				switch ctx.CharacterCfg.Game.Difficulty {
				case difficulty.Normal:
					areaMLvl = table[0]
				case difficulty.Nightmare:
					areaMLvl = table[1]
				case difficulty.Hell:
					areaMLvl = table[2]
				}
				switch nearestShattered.Type {
				case data.MonsterTypeChampion:
					areaMLvl += 2
				case data.MonsterTypeUnique:
					areaMLvl += 3
				}
			}
		case sourceChest:
			if mlvls, exists := game.AreaLevelTable[areaID]; exists {
				switch ctx.CharacterCfg.Game.Difficulty {
				case difficulty.Normal:
					areaMLvl = mlvls[0]
				case difficulty.Nightmare:
					areaMLvl = mlvls[1]
				case difficulty.Hell:
					areaMLvl = mlvls[2]
				}
			}
		}
	}

	// --- Validate MLVL ---
	minMLvl := ctx.CharacterCfg.CubeRecipes.MinMonsterLevel
	maxMLvl := ctx.CharacterCfg.CubeRecipes.MaxMonsterLevel
	if areaMLvl < minMLvl || areaMLvl > maxMLvl {
		ctx.Logger.Warn(
			"Magic specific item NOT marked: monster level out of range",
			"unitID", i.UnitID,
			"itemName", i.Name,
			"areaID", areaID,
			"mlvl", areaMLvl,
			"minMLvl", minMLvl,
			"maxMLvl", maxMLvl,
			"source", sourceToString(chosenSource),
		)
		return
	}

	// --- Mark item ---
	ctx.MarkedSpecificItemUnitID = i.UnitID
	ctx.Logger.Warn(
		"Marked rare specific item on ground",
		"unitID", i.UnitID,
		"areaID", areaID,
		"monsterLevel", areaMLvl,
		"source", sourceToString(chosenSource),
	)
}

func MarkGroundRareSpecificItemIfEligible(i data.Item) {
	ctx := context.Get()

	monsterTypeName := func(t data.MonsterType) string {
		switch t {
		case data.MonsterTypeMinion:
			return "Minion"
		case data.MonsterTypeNone:
			return "Normal"
		case data.MonsterTypeChampion:
			return "Champion"
		case data.MonsterTypeUnique:
			return "Unique"
		case data.MonsterTypeSuperUnique:
			return "SuperUnique"
		default:
			return fmt.Sprintf("Unknown(%d)", t)
		}
	}

	// Already tracking one rare specific item — do not overwrite
	if ctx.MarkedRareSpecificItemUnitID != 0 ||
		ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint != "" {
		return
	}

	// Only pick up configured rare item
	if i.Name != item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) ||
		i.Quality != item.QualityRare {
		return
	}

	areaID := ctx.Data.PlayerUnit.Area
	isTerror := slices.Contains(ctx.Data.TerrorZones, areaID)
	var areaMLvl int

	// --- Find nearest corpse ---
	const maxCorpseDistance = 50
	nearestCorpseID := ClosestCorpseID(i.Position, ctx.Data.Corpses)
	var nearestCorpse *data.Monster
	minCorpseDistance := 9999

	for j := range ctx.Data.Corpses {
		corpse := &ctx.Data.Corpses[j]
		dist := pather.DistanceFromPoint(corpse.Position, i.Position)
		if dist > maxCorpseDistance {
			continue
		}
		if corpse.Name == nearestCorpseID && dist < minCorpseDistance {
			minCorpseDistance = dist
			nearestCorpse = corpse
		}
	}

	// --- Find nearest shattered monster ---
	const maxShatterDistance = 50
	var nearestShattered *context.RecentlySeenMonster
	minShatterDistance := 9999

	for idx := range context.RecentMonsters {
		rm := &context.RecentMonsters[idx]
		dist := pather.DistanceFromPoint(rm.Position, i.Position)
		if dist <= maxShatterDistance && dist < minShatterDistance {
			minShatterDistance = dist
			nearestShattered = rm
		}
	}

	// --- Find nearest chest ---
	var nearestChest *data.Object
	minChestDistance := 9999
	for k := range ctx.Data.Objects {
		obj := &ctx.Data.Objects[k]
		if !IsChestOrContainer(obj.Name) {
			continue
		}
		dist := pather.DistanceFromPoint(obj.Position, i.Position)
		if dist <= 50 && dist < minChestDistance {
			minChestDistance = dist
			nearestChest = obj
		}
	}

	// --- LOG distances ---
	if nearestCorpse != nil {
		ctx.Logger.Warn("Nearest corpse found",
			"corpseID", nearestCorpse.Name,
			"corpseType", monsterTypeName(nearestCorpse.Type),
			"distance", minCorpseDistance,
		)
	}
	if nearestShattered != nil {
		ctx.Logger.Warn("Nearest shattered monster found",
			"monsterID", nearestShattered.Name,
			"monsterType", monsterTypeName(nearestShattered.Type),
			"distance", minShatterDistance,
		)
	}
	if nearestChest != nil {
		ctx.Logger.Warn("Nearest chest found within 50 units",
			"chestName", nearestChest.Name,
			"distance", minChestDistance,
		)
	}

	// --- Tie-break logic for corpse vs shattered ---
	if nearestCorpse != nil && nearestShattered != nil &&
		minCorpseDistance == minShatterDistance {

		var corpseMLvl, shatterMLvl int
		var ok1, ok2 bool

		if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestCorpse.Name)]; ok {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				corpseMLvl = table[0]
			case difficulty.Nightmare:
				corpseMLvl = table[1]
			case difficulty.Hell:
				corpseMLvl = table[2]
			}
			switch nearestCorpse.Type {
			case data.MonsterTypeChampion:
				corpseMLvl += 2
			case data.MonsterTypeUnique:
				corpseMLvl += 3
			}
			ok1 = true
		}

		if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestShattered.Name)]; ok {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				shatterMLvl = table[0]
			case difficulty.Nightmare:
				shatterMLvl = table[1]
			case difficulty.Hell:
				shatterMLvl = table[2]
			}
			switch nearestShattered.Type {
			case data.MonsterTypeChampion:
				shatterMLvl += 2
			case data.MonsterTypeUnique:
				shatterMLvl += 3
			}
			ok2 = true
		}

		if ok1 && ok2 && corpseMLvl != shatterMLvl {
			minMLvl := ctx.CharacterCfg.CubeRecipes.RareMinMonsterLevel
			maxMLvl := ctx.CharacterCfg.CubeRecipes.RareMaxMonsterLevel

			corpseValid := corpseMLvl >= minMLvl && corpseMLvl <= maxMLvl
			shatterValid := shatterMLvl >= minMLvl && shatterMLvl <= maxMLvl

			if !corpseValid || !shatterValid {
				return
			}
		}
	}

	// --- Tie-break logic: corpse vs chest ---
	if nearestCorpse != nil && nearestChest != nil &&
		minCorpseDistance == minChestDistance {

		var corpseMLvl, chestMLvl int
		var okCorpse, okChest bool

		if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestCorpse.Name)]; ok {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				corpseMLvl = table[0]
			case difficulty.Nightmare:
				corpseMLvl = table[1]
			case difficulty.Hell:
				corpseMLvl = table[2]
			}
			switch nearestCorpse.Type {
			case data.MonsterTypeChampion:
				corpseMLvl += 2
			case data.MonsterTypeUnique:
				corpseMLvl += 3
			}
			okCorpse = true
		}

		if mlvls, exists := game.AreaLevelTable[areaID]; exists {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				chestMLvl = mlvls[0]
			case difficulty.Nightmare:
				chestMLvl = mlvls[1]
			case difficulty.Hell:
				chestMLvl = mlvls[2]
			}
			okChest = true
		}

		if okCorpse && okChest && corpseMLvl != chestMLvl {
			minMLvl := ctx.CharacterCfg.CubeRecipes.RareMinMonsterLevel
			maxMLvl := ctx.CharacterCfg.CubeRecipes.RareMaxMonsterLevel

			if corpseMLvl < minMLvl || corpseMLvl > maxMLvl ||
				chestMLvl < minMLvl || chestMLvl > maxMLvl {
				return
			}
		}
	}

	// --- Tie-break logic: shattered vs chest ---
	if nearestShattered != nil && nearestChest != nil &&
		minShatterDistance == minChestDistance {

		var shatterMLvl, chestMLvl int
		var okShatter, okChest bool

		if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestShattered.Name)]; ok {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				shatterMLvl = table[0]
			case difficulty.Nightmare:
				shatterMLvl = table[1]
			case difficulty.Hell:
				shatterMLvl = table[2]
			}
			switch nearestShattered.Type {
			case data.MonsterTypeChampion:
				shatterMLvl += 2
			case data.MonsterTypeUnique:
				shatterMLvl += 3
			}
			okShatter = true
		}

		if mlvls, exists := game.AreaLevelTable[areaID]; exists {
			switch ctx.CharacterCfg.Game.Difficulty {
			case difficulty.Normal:
				chestMLvl = mlvls[0]
			case difficulty.Nightmare:
				chestMLvl = mlvls[1]
			case difficulty.Hell:
				chestMLvl = mlvls[2]
			}
			okChest = true
		}

		if okShatter && okChest && shatterMLvl != chestMLvl {
			minMLvl := ctx.CharacterCfg.CubeRecipes.RareMinMonsterLevel
			maxMLvl := ctx.CharacterCfg.CubeRecipes.RareMaxMonsterLevel

			if shatterMLvl < minMLvl || shatterMLvl > maxMLvl ||
				chestMLvl < minMLvl || chestMLvl > maxMLvl {
				return
			}
		}
	}

	// --- Decide closest source ---
	type sourceKind int
	const (
		sourceNone sourceKind = iota
		sourceCorpse
		sourceShattered
		sourceChest
	)

	// 👇 ADD THIS RIGHT HERE
	sourceToString := func(s sourceKind) string {
		switch s {
		case sourceCorpse:
			return "corpse"
		case sourceShattered:
			return "shattered"
		case sourceChest:
			return "chest"
		default:
			return "none"
		}
	}

	chosenSource := sourceNone
	minDist := 9999

	if nearestCorpse != nil && minCorpseDistance < minDist {
		chosenSource = sourceCorpse
		minDist = minCorpseDistance
	}
	if nearestShattered != nil && minShatterDistance < minDist {
		chosenSource = sourceShattered
		minDist = minShatterDistance
	}
	if nearestChest != nil && minChestDistance < minDist {
		chosenSource = sourceChest
		minDist = minChestDistance
	}

	// --- Terror zone MLVL helper ---
	calcTerrorMLvl := func(clvl int, mType data.MonsterType, diff difficulty.Difficulty) int {
		var base int
		switch mType {
		case data.MonsterTypeNone:
			base = clvl + 2
		case data.MonsterTypeChampion:
			base = clvl + 4
		case data.MonsterTypeUnique, data.MonsterTypeSuperUnique:
			base = clvl + 5
		default:
			base = clvl + 2
		}

		switch diff {
		case difficulty.Normal:
			if base > 45 {
				base = 45
			}
		case difficulty.Nightmare:
			if base > 71 {
				base = 71
			}
		case difficulty.Hell:
			if base > 96 {
				base = 96
			}
		}
		return base
	}

	// --- Apply MLVL ---
	if isTerror {
		if clvlStat, ok := ctx.Data.PlayerUnit.FindStat(stat.Level, 0); ok {
			clvl := clvlStat.Value
			switch chosenSource {
			case sourceCorpse:
				areaMLvl = calcTerrorMLvl(clvl, nearestCorpse.Type, ctx.CharacterCfg.Game.Difficulty)
			case sourceShattered:
				areaMLvl = calcTerrorMLvl(clvl, nearestShattered.Type, ctx.CharacterCfg.Game.Difficulty)
			case sourceChest:
				areaMLvl = calcTerrorMLvl(clvl, data.MonsterTypeNone, ctx.CharacterCfg.Game.Difficulty)
			default:
				areaMLvl = calcTerrorMLvl(clvl, data.MonsterTypeNone, ctx.CharacterCfg.Game.Difficulty)
			}
		} else {
			return
		}
	} else {
		switch chosenSource {
		case sourceCorpse:
			if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestCorpse.Name)]; ok {
				switch ctx.CharacterCfg.Game.Difficulty {
				case difficulty.Normal:
					areaMLvl = table[0]
				case difficulty.Nightmare:
					areaMLvl = table[1]
				case difficulty.Hell:
					areaMLvl = table[2]
				}
				switch nearestCorpse.Type {
				case data.MonsterTypeChampion:
					areaMLvl += 2
				case data.MonsterTypeUnique:
					areaMLvl += 3
				}
			}
		case sourceShattered:
			if table, ok := game.MonsterLevelTable[fmt.Sprint(nearestShattered.Name)]; ok {
				switch ctx.CharacterCfg.Game.Difficulty {
				case difficulty.Normal:
					areaMLvl = table[0]
				case difficulty.Nightmare:
					areaMLvl = table[1]
				case difficulty.Hell:
					areaMLvl = table[2]
				}
				switch nearestShattered.Type {
				case data.MonsterTypeChampion:
					areaMLvl += 2
				case data.MonsterTypeUnique:
					areaMLvl += 3
				}
			}
		case sourceChest:
			if mlvls, exists := game.AreaLevelTable[areaID]; exists {
				switch ctx.CharacterCfg.Game.Difficulty {
				case difficulty.Normal:
					areaMLvl = mlvls[0]
				case difficulty.Nightmare:
					areaMLvl = mlvls[1]
				case difficulty.Hell:
					areaMLvl = mlvls[2]
				}
			}
		}
	}

	// --- Validate MLVL ---
	minMLvl := ctx.CharacterCfg.CubeRecipes.RareMinMonsterLevel
	maxMLvl := ctx.CharacterCfg.CubeRecipes.RareMaxMonsterLevel
	if areaMLvl < minMLvl || areaMLvl > maxMLvl {
		ctx.Logger.Warn(
			"Rare specific item NOT marked: monster level out of range",
			"unitID", i.UnitID,
			"itemName", i.Name,
			"areaID", areaID,
			"mlvl", areaMLvl,
			"minMLvl", minMLvl,
			"maxMLvl", maxMLvl,
			"source", sourceToString(chosenSource),
		)
		return
	}

	// --- Mark item ---
	ctx.MarkedRareSpecificItemUnitID = i.UnitID
	ctx.Logger.Warn(
		"Marked rare specific item on ground",
		"unitID", i.UnitID,
		"areaID", areaID,
		"monsterLevel", areaMLvl,
		"source", sourceToString(chosenSource),
	)
}

func identifySpecificMarkedItem(idTome data.Item, i data.Item) {
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
		ctx.MarkedSpecificItemUnitID = 0 // reset
	}

	if !found {
		ctx.Logger.Error("FAILED TO IDENTIFY ITEM AFTER TIMEOUT", "unitID", i.UnitID)
		ctx.MarkedSpecificItemUnitID = 0 // reset
		return
	}
	step.CloseAllMenus()

	// ✅ Fingerprint logic for marked Specific Item (NOW SAFE)
	if identified.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) &&
		identified.Quality == item.QualityMagic &&
		ctx.MarkedSpecificItemUnitID == identified.UnitID {

		if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(identified); res != nip.RuleResultFullMatch {
			fp := SpecificFingerprint(identified)

			ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint = fp
			ctx.Logger.Warn("SAVED MARKED SPECIFIC ITEM FINGERPRINT", "fp", fp)
			shouldStashIt(identified, true) // JUST ADDED
			if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
				ctx.Logger.Error("FAILED TO SAVE CharacterCfg WITH FINGERPRINT", "err", err)
			}

			//PHYSICALLY STASH IMMEDIATELY AFTER FINDING
			// 🚨 Immediately stash marked item to avoid any future selling/cubing issues
			ctx.Logger.Warn("Immediately stashing marked specific item", "unitID", identified.UnitID)

			// Ensure town
			if ctx.Data.PlayerUnit.Area != area.Harrogath && ctx.Data.PlayerUnit.Area != area.ThePandemoniumFortress && ctx.Data.PlayerUnit.Area != area.KurastDocks && ctx.Data.PlayerUnit.Area != area.LutGholein && ctx.Data.PlayerUnit.Area != area.RogueEncampment {
				ReturnTown()
				utils.PingSleep(utils.Critical, 1500)
			}
			// Open stash
			OpenStash()
			utils.PingSleep(utils.Medium, 800)
			// 🔁 REFRESH + RE-FIND ITEM
			ctx.RefreshGameData()

			var stashItem *data.Item
			for _, it := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
				// Match by fingerprint instead of UnitID
				if SpecificFingerprint(it) == ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint {
					stashItem = &it
					break
				}
			}

			if stashItem == nil {
				ctx.Logger.Error(
					"Marked item not found in inventory after opening stash",
				)
				step.CloseAllMenus()
				return
			}

			// ✅ Physically stash the identified marked item
			if stashItem != nil {
				if ctx.CharacterCfg.Character.StashToShared {
					if !stashItemAcrossTabs(*stashItem, "MARKED_SPECIFIC_ITEM", "", false) {
						ctx.Logger.Error("Failed to physically stash marked specific item", "unitID", stashItem.UnitID)
					} else {
						ctx.Logger.Warn("Successfully stashed marked specific item", "unitID", stashItem.UnitID)
					}
				} else {
					if !stashItemAction(*stashItem, "MARKED_SPECIFIC_ITEM", "", false) {
						ctx.Logger.Error("Failed to physically stash marked specific item", "unitID", stashItem.UnitID)
					} else {
						ctx.Logger.Warn("Successfully stashed marked specific item", "unitID", stashItem.UnitID)
					}
				}
				utils.PingSleep(utils.Medium, 2000) // ensure the stash action completes
				step.CloseAllMenus()
			}

			UsePortalInTown()
		} else {
			ctx.Logger.Warn("SPECIFIC ITEM THAT I WAS GOING TO MARK TURNED OUT TO BE A KEEPER, NOT MARKING IT")
		}

		// Clear temporary UnitID tracking (runtime-only)
		ctx.MarkedSpecificItemUnitID = 0
	}
}

func identifyRareSpecificMarkedItem(idTome data.Item, i data.Item) {
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
		ctx.MarkedRareSpecificItemUnitID = 0 // reset
	}

	if !found {
		ctx.Logger.Error("FAILED TO IDENTIFY ITEM AFTER TIMEOUT", "unitID", i.UnitID)
		ctx.MarkedRareSpecificItemUnitID = 0 // reset
		return
	}

	// ✅ Fingerprint logic for marked Specific Item (NOW SAFE)
	if identified.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) &&
		identified.Quality == item.QualityRare &&
		ctx.MarkedRareSpecificItemUnitID == identified.UnitID {

		if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(identified); res != nip.RuleResultFullMatch {
			fp := SpecificRareFingerprint(identified)

			ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint = fp
			ctx.Logger.Warn("SAVED MARKED RARE SPECIFIC ITEM FINGERPRINT", "fp", fp)
			shouldStashIt(identified, true) // JUST ADDED
			if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
				ctx.Logger.Error("FAILED TO SAVE CharacterCfg WITH FINGERPRINT", "err", err)
			}
			//PHYSICALLY STASH IMMEDIATELY AFTER FINDING
			// 🚨 Immediately stash marked item to avoid any future selling/cubing issues
			ctx.Logger.Warn("Immediately stashing marked rare specific item", "unitID", identified.UnitID)

			// Ensure town
			if ctx.Data.PlayerUnit.Area != area.Harrogath && ctx.Data.PlayerUnit.Area != area.ThePandemoniumFortress && ctx.Data.PlayerUnit.Area != area.KurastDocks && ctx.Data.PlayerUnit.Area != area.LutGholein && ctx.Data.PlayerUnit.Area != area.RogueEncampment {
				ReturnTown()
				utils.PingSleep(utils.Critical, 1500)
			}
			// Open stash
			OpenStash()
			utils.PingSleep(utils.Medium, 800)
			// 🔁 REFRESH + RE-FIND ITEM
			ctx.RefreshGameData()

			var stashItem *data.Item
			for _, it := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
				// Match by fingerprint instead of UnitID
				if SpecificRareFingerprint(it) == ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint {
					stashItem = &it
					break
				}
			}

			if stashItem == nil {
				ctx.Logger.Error(
					"Rare Marked item not found in inventory after opening stash",
				)
				step.CloseAllMenus()
				return
			}

			// ✅ Physically stash the identified marked item
			if stashItem != nil {
				if ctx.CharacterCfg.Character.StashToShared {
					if !stashItemAcrossTabs(*stashItem, "MARKED_RARE_ITEM", "", false) {
						ctx.Logger.Error("Failed to physically stash marked specific item", "unitID", stashItem.UnitID)
					} else {
						ctx.Logger.Warn("Successfully stashed marked specific item", "unitID", stashItem.UnitID)
					}
				} else {
					if !stashItemAction(*stashItem, "MARKED_RARE_ITEM", "", false) {
						ctx.Logger.Error("Failed to physically stash marked specific item", "unitID", stashItem.UnitID)
					} else {
						ctx.Logger.Warn("Successfully stashed marked specific item", "unitID", stashItem.UnitID)
					}
				}
				utils.PingSleep(utils.Medium, 2000) // ensure the stash action completes
				step.CloseAllMenus()
			}

			UsePortalInTown()
		} else {
			ctx.Logger.Warn("SPECIFIC RARE ITEM THAT I WAS GOING TO MARK TURNED OUT TO BE A KEEPER, NOT MARKING IT")
		}

		// Clear temporary UnitID tracking (runtime-only)
		ctx.MarkedRareSpecificItemUnitID = 0
	}
}

func HandleSmallCharmOnFloor(groundCharm data.Item) error {
	ctx := context.Get()
	ctx.RefreshGameData()

	// Guard: already in pickup-allowed phase
	if ctx.CharacterCfg.Inventory.AllowAnniPickup {
		return nil
	}

	var existingCharm *data.Item
	var origX, origY int

	// Find existing Anni
	for i := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		it := ctx.Data.Inventory.ByLocation(item.LocationInventory)[i]
		if it.Name == "SmallCharm" && it.Quality == item.QualityUnique {
			existingCharm = &it
			origX, origY = it.Position.X, it.Position.Y
			ctx.CharacterCfg.Inventory.AnniFingerprint = SpecificUniqueFingerprint(it)
			break
		}
	}

	// If we already have an Anni, stash it
	if existingCharm != nil {
		ctx.Logger.Warn("Existing Anni found, stashing it")

		origLock := ctx.CharacterCfg.Inventory.InventoryLock[origY][origX]
		ctx.Logger.Warn("ORIGINAL LOCKED/UNLOCKED INTEGER FOR ANNI IN INVENTORY: " + strconv.Itoa(origLock))
		ctx.Logger.Warn("SWITCHING INTEGER TO 0 (I THINK 0 MEANS UNLOCK IT?)")
		ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = 0

		if err := ReturnTown(); err != nil {
			return err
		}
		utils.PingSleep(utils.Critical, 1200)

		if err := OpenStash(); err != nil {
			return err
		}
		utils.PingSleep(utils.Medium, 800)

		stashItemAcrossTabs(*existingCharm, "Temp_Anni", "", false)
		utils.PingSleep(utils.Medium, 1500)
		step.CloseAllMenus()

		ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = origLock
	}

	// ✅ Enable pickup and return to area
	ctx.CharacterCfg.Inventory.AllowAnniPickup = true
	ctx.Logger.Warn("Anni pickup ENABLED, returning through TP")

	UsePortalInTown()
	return nil
}

func HandleNewAnniPickup(newCharm data.Item) error {
	ctx := context.Get()
	ctx.RefreshGameData()

	ctx.Logger.Warn("New Anni picked up, handling swap")

	// Disable further Anni pickup immediately
	ctx.CharacterCfg.Inventory.AllowAnniPickup = false

	// Save original inventory lock for the new Anni slot and unlock it
	newLock := ctx.CharacterCfg.Inventory.InventoryLock[newCharm.Position.Y][newCharm.Position.X]
	ctx.CharacterCfg.Inventory.InventoryLock[newCharm.Position.Y][newCharm.Position.X] = 0

	// Return to town
	if err := ReturnTown(); err != nil {
		return err
	}
	utils.PingSleep(utils.Critical, 1200)

	// Open stash
	if err := OpenStash(); err != nil {
		return err
	}
	utils.PingSleep(utils.Medium, 800)

	// Stash the newly picked Anni
	stashItemAcrossTabs(newCharm, "Temp_Anni", "", false)
	utils.PingSleep(utils.Medium, 1500)

	// Retrieve original Anni from stash
	ctx.RefreshGameData()
	var originalAnni *data.Item
	for _, it := range ctx.Data.Inventory.ByLocation(item.LocationSharedStash) {
		if it.Name == "SmallCharm" &&
			it.Quality == item.QualityUnique &&
			SpecificUniqueFingerprint(it) == ctx.CharacterCfg.Inventory.AnniFingerprint {
			originalAnni = &it
			break
		}
	}

	if originalAnni == nil {
		return fmt.Errorf("original Anni not found in stash")
	}

	// Save inventory lock for original Anni's original slot and unlock it
	origX, origY := originalAnni.Position.X, originalAnni.Position.Y
	origLock := ctx.CharacterCfg.Inventory.InventoryLock[origY][origX]
	ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = 0

	// Move original Anni from stash to its original inventory slot
	from := ui.GetScreenCoordsForItem(*originalAnni)
	to := ui.GetScreenCoordsForInventoryPosition(
		data.Position{X: origX, Y: origY},
		item.LocationInventory,
	)

	// Pick up original Anni from stash (normal left-click)
	ctx.HID.MovePointer(from.X, from.Y)
	ctx.HID.Click(game.LeftButton, from.X, from.Y)
	utils.PingSleep(utils.Medium, 300)

	// Place original Anni in its original inventory slot
	ctx.HID.MovePointer(to.X, to.Y)
	ctx.HID.Click(game.LeftButton, to.X, to.Y)
	utils.PingSleep(utils.Medium, 300)

	// Restore inventory locks
	ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = origLock
	ctx.CharacterCfg.Inventory.InventoryLock[newCharm.Position.Y][newCharm.Position.X] = newLock
	ctx.CharacterCfg.Inventory.AnniFingerprint = ""

	// Cleanup
	step.CloseAllMenus()
	UsePortalInTown()

	ctx.Logger.Warn("Anni swap complete")
	return nil
}

func onAnniPickedUp(itemToPickup data.Item) {
	ctx := context.Get()

	if itemToPickup.Name == "SmallCharm" &&
		itemToPickup.Quality == item.QualityUnique &&
		ctx.CharacterCfg.Inventory.AllowAnniPickup {

		ctx.Logger.Warn("Anni picked up — running post-pickup handler")

		ctx.RefreshInventory()

		for _, it := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
			if it.Name == "SmallCharm" && it.Quality == item.QualityUnique {
				_ = HandleNewAnniPickup(it)
				break
			}
		}
	}
}

/* func HandleSmallCharmOnFloor(groundCharm data.Item) error {
	ctx := context.Get()
	ctx.RefreshGameData()

	if ctx.CharacterCfg.Inventory.AllowAnniPickup {
		return nil
	}


	// ----------------------------------------
	// 1. Detect existing unique small charm
	// ----------------------------------------
	var existingCharm *data.Item
	var origX, origY int

	for i := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		it := ctx.Data.Inventory.ByLocation(item.LocationInventory)[i]
		if it.Name == "SmallCharm" && it.Quality == item.QualityUnique {
			existingCharm = &it
			origX, origY = it.Position.X, it.Position.Y
			ctx.CharacterCfg.Inventory.AnniFingerprint = SpecificUniqueFingerprint(it)
			break
		}
	}

	// ----------------------------------------
	// 2. If one exists → stash it
	// ----------------------------------------
	if existingCharm != nil {
		ctx.Logger.Warn("Existing unique small charm found, stashing it")

		origLock := ctx.CharacterCfg.Inventory.InventoryLock[origY][origX]
		ctx.Logger.Warn("ORIGINAL LOCKED/UNLOCKED INTEGER FOR ANNI IN INVENTORY: " + strconv.Itoa(origLock))
		ctx.Logger.Warn("SWITCHING INTEGER TO 0 (I THINK 0 MEANS UNLOCK IT?)")
		ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = 0

		if err := ReturnTown(); err != nil {
			return err
		}
		utils.PingSleep(utils.Critical, 1200)

		if err := OpenStash(); err != nil {
			return err
		}
		utils.PingSleep(utils.Medium, 800)

		stashItemAcrossTabs(*existingCharm, "Temporarily_stashing_small_charm", "", false)
		utils.PingSleep(utils.Medium, 1500)
		step.CloseAllMenus()
		ctx.CharacterCfg.Inventory.AllowAnniPickup = true

		ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = origLock
		UsePortalInTown()
	}

	// ----------------------------------------
	// 3. Pick up the unid ground charm
	// ----------------------------------------
	utils.PingSleep(utils.Critical, 2000)
	ctx.RefreshGameData()

	ctx.CharacterCfg.Inventory.AllowAnniPickup = false

	// ----------------------------------------
	// 4. Find newly picked-up charm
	// ----------------------------------------
	var newCharm *data.Item
	for i := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		it := ctx.Data.Inventory.ByLocation(item.LocationInventory)[i]
		if it.Name == "SmallCharm" && it.Quality == item.QualityUnique {
			newCharm = &it
			break
		}
	}

	if newCharm == nil {
		ctx.Logger.Warn("No new unique small charm found after pickup")
		return nil
	}

	// ----------------------------------------
	// 5. Stash the newly picked-up charm
	// ----------------------------------------
	ctx.Logger.Warn("Stashing newly picked-up unique small charm")

	newLock := ctx.CharacterCfg.Inventory.InventoryLock[newCharm.Position.Y][newCharm.Position.X]
	ctx.CharacterCfg.Inventory.InventoryLock[newCharm.Position.Y][newCharm.Position.X] = 0

	if err := ReturnTown(); err != nil {
		return err
	}
	utils.PingSleep(utils.Critical, 1200)

	if err := OpenStash(); err != nil {
		return err
	}
	utils.PingSleep(utils.Medium, 800)

	stashItemAcrossTabs(*newCharm, "Temporarily_stashing_small_charm", "", false)
	utils.PingSleep(utils.Medium, 1500)

	// ----------------------------------------
	// 6. Take back ORIGINAL charm from shared stash
	// ----------------------------------------
	ctx.RefreshGameData()
	var toTake []data.Item

	for _, it := range ctx.Data.Inventory.ByLocation(item.LocationSharedStash) {
		if it.Name == "SmallCharm" &&
			it.Quality == item.QualityUnique &&
			SpecificUniqueFingerprint(it) == ctx.CharacterCfg.Inventory.AnniFingerprint {
			toTake = append(toTake, it)
			break
		}
	}

	if len(toTake) == 0 {
		return fmt.Errorf("failed to locate original unique small charm in shared stash")
	}

	if err := TakeItemsFromStash(toTake); err != nil {
		return err
	}

	// ----------------------------------------
	// 7. Move charm back to ORIGINAL SLOT
	// ----------------------------------------
	ctx.RefreshGameData()

	var returnedCharm *data.Item
	for i := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		it := ctx.Data.Inventory.ByLocation(item.LocationInventory)[i]
		if it.Name == "SmallCharm" &&
			it.Quality == item.QualityUnique &&
			SpecificUniqueFingerprint(it) == ctx.CharacterCfg.Inventory.AnniFingerprint {
			returnedCharm = &it
			break
		}
	}

	if returnedCharm == nil {
		return fmt.Errorf("returned charm not found in inventory")
	}

	origLock := ctx.CharacterCfg.Inventory.InventoryLock[origY][origX]
	ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = 0

	from := ui.GetScreenCoordsForItem(*returnedCharm)
	to := ui.GetScreenCoordsForInventoryPosition(
		data.Position{X: origX, Y: origY},
		item.LocationInventory,
	)

	ctx.HID.Click(game.LeftButton, from.X, from.Y)
	utils.PingSleep(utils.Light, 150)
	ctx.HID.Click(game.LeftButton, to.X, to.Y)
	utils.PingSleep(utils.Light, 200)

	ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = origLock
	ctx.CharacterCfg.Inventory.InventoryLock[newCharm.Position.Y][newCharm.Position.X] = newLock

	step.CloseAllMenus()
	ctx.CharacterCfg.Inventory.AnniFingerprint = "" // reset fingerprint
	ctx.Logger.Warn("Small charm handling complete")
	UsePortalInTown()

	return nil
} */

//THIS BELOW IS A SAFER OPTION SO THAT THE ORINAL SLOT ALWAYS GETS SET BACK TO ITS ORIGINAL LOCK STATE NO MATTER WHAT HAPPENS
/* func HandleSmallCharmOnFloor(groundCharm data.Item) error {
	ctx := context.Get()
	ctx.RefreshGameData()

	// Safety net: never leave locks broken even on panic
	defer func() {
		if r := recover(); r != nil {
			ctx.Logger.Error("panic in HandleSmallCharmOnFloor", "panic", r)
		}
	}()

	invItems := ctx.Data.Inventory.ByLocation(item.LocationInventory)

	// ----------------------------------------
	// 1. Detect existing unique small charm
	// ----------------------------------------
	var existingCharm *data.Item
	var origX, origY int

	for i := range invItems {
		it := invItems[i]
		if it.Name == "SmallCharm" && it.Quality == item.QualityUnique {
			existingCharm = &it
			origX, origY = it.Position.X, it.Position.Y
			ctx.CharacterCfg.Inventory.AnniFingerprint = SpecificFingerprint(it)
			break
		}
	}

	// ----------------------------------------
	// 2. If one exists → stash it
	// ----------------------------------------
	if existingCharm != nil {
		ctx.Logger.Warn("Existing unique small charm found, stashing it")

		origLock := ctx.CharacterCfg.Inventory.InventoryLock[origY][origX]
		ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = 0

		// GUARANTEED restoration
		defer func(x, y, lock int) {
			ctx.CharacterCfg.Inventory.InventoryLock[y][x] = lock
			ctx.Logger.Debug("Restored original charm lock", "x", x, "y", y)
		}(origX, origY, origLock)

		if err := ReturnTown(); err != nil {
			return err
		}
		utils.PingSleep(utils.Critical, 1200)

		if err := OpenStash(); err != nil {
			return err
		}
		utils.PingSleep(utils.Medium, 800)

		stashItemAcrossTabs(*existingCharm, "Temporarily_stashing_small_charm", "", false)
		utils.PingSleep(utils.Medium, 1500)

		step.CloseAllMenus()
		UsePortalInTown()
	}

	// ----------------------------------------
	// 3. Pick up the ground charm
	// ----------------------------------------
	ItemPickup(40)
	ctx.RefreshGameData()

	invItems = ctx.Data.Inventory.ByLocation(item.LocationInventory)

	// ----------------------------------------
	// 4. Find newly picked-up charm
	// ----------------------------------------
	var newCharm *data.Item
	for i := range invItems {
		it := invItems[i]
		if it.Name == "SmallCharm" && it.Quality == item.QualityUnique {
			newCharm = &it
			break
		}
	}

	if newCharm == nil {
		ctx.Logger.Warn("No new unique small charm found after pickup")
		return nil
	}

	// ----------------------------------------
	// 5. Stash the newly picked-up charm
	// ----------------------------------------
	ctx.Logger.Warn("Stashing newly picked-up unique small charm")

	nx, ny := newCharm.Position.X, newCharm.Position.Y
	newLock := ctx.CharacterCfg.Inventory.InventoryLock[ny][nx]
	ctx.CharacterCfg.Inventory.InventoryLock[ny][nx] = 0

	// GUARANTEED restoration
	defer func(x, y, lock int) {
		ctx.CharacterCfg.Inventory.InventoryLock[y][x] = lock
		ctx.Logger.Debug("Restored new charm lock", "x", x, "y", y)
	}(nx, ny, newLock)

	if err := ReturnTown(); err != nil {
		return err
	}
	utils.PingSleep(utils.Critical, 1200)

	if err := OpenStash(); err != nil {
		return err
	}
	utils.PingSleep(utils.Medium, 800)

	stashItemAcrossTabs(*newCharm, "Temporarily_stashing_small_charm", "", false)
	utils.PingSleep(utils.Medium, 1500)

	// ----------------------------------------
	// 6. Take back ORIGINAL charm from shared stash
	// ----------------------------------------
	ctx.RefreshGameData()

	var toTake []data.Item
	for _, it := range ctx.Data.Inventory.ByLocation(item.LocationSharedStash) {
		if it.Name == "SmallCharm" &&
			it.Quality == item.QualityUnique &&
			SpecificFingerprint(it) == ctx.CharacterCfg.Inventory.AnniFingerprint {
			toTake = append(toTake, it)
			break
		}
	}

	if len(toTake) == 0 {
		return fmt.Errorf("failed to locate original unique small charm in shared stash")
	}

	if err := TakeItemsFromStash(toTake); err != nil {
		return err
	}

	// ----------------------------------------
	// 7. Move charm back to ORIGINAL SLOT
	// ----------------------------------------
	ctx.RefreshGameData()
	invItems = ctx.Data.Inventory.ByLocation(item.LocationInventory)

	var returnedCharm *data.Item
	for i := range invItems {
		it := invItems[i]
		if it.Name == "SmallCharm" &&
			it.Quality == item.QualityUnique &&
			SpecificFingerprint(it) == ctx.CharacterCfg.Inventory.AnniFingerprint {
			returnedCharm = &it
			break
		}
	}

	if returnedCharm == nil {
		return fmt.Errorf("returned charm not found in inventory")
	}

	// Temporarily unlock original slot again (safe due to defer)
	origLock := ctx.CharacterCfg.Inventory.InventoryLock[origY][origX]
	ctx.CharacterCfg.Inventory.InventoryLock[origY][origX] = 0

	defer func(x, y, lock int) {
		ctx.CharacterCfg.Inventory.InventoryLock[y][x] = lock
		ctx.Logger.Debug("Final restore of original slot lock", "x", x, "y", y)
	}(origX, origY, origLock)

	from := ui.GetScreenCoordsForItem(*returnedCharm)
	to := ui.GetScreenCoordsForInventoryPosition(
		data.Position{X: origX, Y: origY},
		item.LocationInventory,
	)

	ctx.HID.Click(game.LeftButton, from.X, from.Y)
	utils.PingSleep(utils.Light, 150)
	ctx.HID.Click(game.LeftButton, to.X, to.Y)
	utils.PingSleep(utils.Light, 200)

	step.CloseAllMenus()
	ctx.CharacterCfg.Inventory.AnniFingerprint = "" // reset fingerprint
	ctx.Logger.Warn("Small charm handling complete")
	UsePortalInTown()


	return nil
} */
