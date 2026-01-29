package action

import (
	"log/slog"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/context"
	botCtx "github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/town"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
	"github.com/lxn/win"
)

func prioritizeVendor(vendors []npc.ID, priority npc.ID) []npc.ID {
	out := make([]npc.ID, 0, len(vendors))
	out = append(out, priority)

	for _, v := range vendors {
		if v != priority {
			out = append(out, v)
		}
	}
	return out
}

func walkToHratli(ctx *context.Status) {
	ctx.Logger.Debug("Walking to Hratli")

	// Define positions to check for Hratli
	finalPos := data.Position{X: 5224, Y: 5045} // main position
	startPos := data.Position{X: 5116, Y: 5167} // fallback position

	// First, try moving to the final position
	ctx.SetLastStep("walkToHratli - finalPos")
	MoveToCoords(finalPos)

	// Check if Hratli is found
	_, found := ctx.Data.Monsters.FindOne(npc.Hratli, data.MonsterTypeNone)
	if found {
		ctx.Logger.Debug("Hratli found at final position")
		return
	}

	// Hratli not found at final position, move to start position
	ctx.Logger.Warn("Hratli not found at final position. Moving to start position.")
	ctx.SetLastStep("walkToHratli - startPos")
	MoveToCoords(startPos)

	// Try interacting with Hratli at start position
	if err := InteractNPC(npc.Hratli); err != nil {
		ctx.Logger.Warn("Failed to interact with Hratli at start position.", "error", err)
	}

	// Close any menus if opened during interaction
	step.CloseAllMenus()
}

func walkToLarzuk(ctx *context.Status) {
	ctx.Logger.Debug("Walking to Larzuk")

	target := data.Position{X: 5135, Y: 5046}

	// Start walking
	MoveTo(func() (data.Position, bool) {
		return target, true
	})

	// Wait until player reaches target or timeout
	timeout := 7000 // milliseconds
	interval := 100 // check every 100ms
	elapsed := 0

	for elapsed < timeout {
		ctx.RefreshGameData()

		// Check player position
		pos := ctx.Data.PlayerUnit.Position
		dx := pos.X - target.X
		dy := pos.Y - target.Y
		if dx*dx+dy*dy <= 9 { // within ~3 tiles
			ctx.Logger.Debug("Reached Larzuk position", slog.Any("pos", pos))
			return
		}

		// Optionally: check if Larzuk NPC is loaded
		_, found := ctx.Data.NPCs.FindOne(npc.Larzuk)
		if found {
			ctx.Logger.Debug("Larzuk loaded in area")
			return
		}

		utils.Sleep(interval)
		elapsed += interval
	}

	ctx.Logger.Warn("Timeout walking to Larzuk", slog.Any("pos", ctx.Data.PlayerUnit.Position))
}

func walkToAnya(ctx *context.Status) {
	ctx.Logger.Debug("Walking to Anya")

	target := data.Position{X: 5107, Y: 5119}

	// Start walking
	MoveTo(func() (data.Position, bool) {
		return target, true
	})

	// Wait until player reaches target or timeout
	timeout := 7000 // milliseconds
	interval := 100 // check every 100ms
	elapsed := 0

	for elapsed < timeout {
		ctx.RefreshGameData()

		// Check player position
		pos := ctx.Data.PlayerUnit.Position
		dx := pos.X - target.X
		dy := pos.Y - target.Y
		if dx*dx+dy*dy <= 9 { // within ~3 tiles
			ctx.Logger.Debug("Reached Anya position", slog.Any("pos", pos))
			return
		}

		// Optionally: check if Anya NPC is loaded
		_, found := ctx.Data.NPCs.FindOne(npc.Drehya) // backend ID for Anya
		if found {
			ctx.Logger.Debug("Anya loaded in area")
			return
		}

		utils.Sleep(interval)
		elapsed += interval
	}

	ctx.Logger.Warn("Timeout walking to Anya", slog.Any("pos", ctx.Data.PlayerUnit.Position))
}

// Global variable to track vendor inventory fingerprint
var lastVendorInventoryItems []string = nil

func VendorRefill(forceRefill bool, sellJunk bool, tempLock ...[][]int) (err error) {
	ctx := botCtx.Get()
	ctx.SetLastAction("VendorRefill")

	if !forceRefill && !shouldVisitVendor() && len(tempLock) == 0 {
		return nil
	}

	currentArea := ctx.Data.PlayerUnit.Area
	ctx.Logger.Info("Visiting vendor...", slog.Any("area", currentArea))

	// ---------- REFILL VENDOR ----------
	vendorNPC := town.GetTownByArea(currentArea).RefillNPC()

	if vendorNPC == npc.Drognan {
		_, needsBuy := town.ShouldBuyKeys()
		if needsBuy && ctx.Data.PlayerUnit.Class != data.Assassin {
			vendorNPC = npc.Lysander
		}
	}
	if vendorNPC == npc.Ormus {
		_, needsBuy := town.ShouldBuyKeys()
		if needsBuy && ctx.Data.PlayerUnit.Class != data.Assassin {
			if err := FindHratliEverywhere(); err != nil {
				return err
			}
			vendorNPC = npc.Hratli
		}
	}

	if err = InteractNPC(vendorNPC); err != nil {
		return err
	}

	// Open vendor trade
	if vendorNPC == npc.Jamella || vendorNPC == npc.Halbu {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_RETURN)
	} else {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
	}
	// 🔒 Protect marked specific item from being sold
	if ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint != "" {
		for _, it := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
			if it.Quality == item.QualityMagic &&
				it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) &&
				SpecificFingerprint(it) == ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint {

				ctx.Logger.Warn(
					"LOCKING MARKED SPECIFIC ITEM TO PREVENT SELLING",
					"unitID", it.UnitID,
					"fp", ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint,
				)

				// Lock its inventory position
				tempLock = append(tempLock, [][]int{
					{it.Position.X, it.Position.Y},
				})
				break
			}
		}
	}

	// 🔒 Protect marked specific rare item from being sold
	if ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint != "" {
		for _, it := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
			if it.Quality == item.QualityRare &&
				it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) &&
				SpecificRareFingerprint(it) == ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint {

				ctx.Logger.Warn(
					"LOCKING RARE MARKED SPECIFIC ITEM TO PREVENT SELLING",
					"unitID", it.UnitID,
					"fp", ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint,
				)

				// Lock its inventory position
				tempLock = append(tempLock, [][]int{
					{it.Position.X, it.Position.Y},
				})
				break
			}
		}
	}

	// Sell junk
	if sellJunk {
		if len(tempLock) > 0 {
			ctx.Logger.Warn(
				"SELLING JUNK WITH LOCKED SLOTS",
				"locks", tempLock,
			)
			town.SellJunk(tempLock...)
		} else {
			town.SellJunk()
		}
	}

	// Buy consumables
	SwitchVendorTab(4)
	ctx.RefreshGameData()
	town.BuyConsumables(forceRefill)

	if ctx.CharacterCfg.Game.ShopVendorsDuringTownVisits {
		// ---------- SHOP ALL VENDORS ----------
		shopPlan := NewTownActionShoppingPlan()

		refillVendor := town.GetTownByArea(currentArea).RefillNPC()
		shopPlan.Vendors = prioritizeVendor(shopPlan.Vendors, refillVendor)

		keepVendorOpen := len(shopPlan.Vendors) > 0 && shopPlan.Vendors[0] == vendorNPC

		if !keepVendorOpen {
			step.CloseAllMenus()
			ctx.RefreshGameData()
		} else {
			ctx.RefreshGameData()
		}

		currentItems := getVendorInventoryItems(ctx)
		shouldShop := !allItemsStillExist(lastVendorInventoryItems, currentItems)

		if !shouldShop {
			ctx.Logger.Info("Skipping shopping - refill vendor inventory unchanged")
			return nil
		}

		for idx, vendor := range shopPlan.Vendors {
			vendorArea, ok := VendorLocationMap[vendor]
			if !ok || vendorArea != currentArea {
				continue
			}

			// Open vendor unless reusing refill vendor
			if !(idx == 0 && keepVendorOpen && vendor == vendorNPC) {
				step.CloseAllMenus()
				ctx.RefreshGameData()

				if vendor == npc.Hratli {
					walkToHratli(ctx)
				}
				if vendor == npc.Larzuk {
					walkToLarzuk(ctx)
				}
				if vendor == npc.Drehya {
					walkToAnya(ctx)
				}

				if err := InteractNPC(vendor); err != nil {
					ctx.Logger.Warn("Failed to interact vendor", slog.Any("err", err))
					continue
				}

				switch vendor {
				case npc.Jamella, npc.Halbu:
					ctx.HID.KeySequence(win.VK_HOME, win.VK_RETURN)
				case npc.Asheara:
					ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_DOWN, win.VK_RETURN)
				default:
					ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
				}

				ctx.RefreshGameData()
			}

			scanAndPurchaseItems(vendor, shopPlan)

			// Close vendor unless explicitly kept open
			if !(idx == 0 && keepVendorOpen && vendor == vendorNPC) {
				step.CloseAllMenus()
				ctx.RefreshGameData()
			}
		}

		// ✅ Store fingerprint AFTER all vendors
		lastVendorInventoryItems = currentItems
		ctx.Logger.Debug(
			"Shopping completed, stored refill vendor inventory items",
			slog.Int("itemCount", len(lastVendorInventoryItems)),
		)
	}

	// Safety close
	step.CloseAllMenus()
	ctx.RefreshGameData()

	return nil
}

// getVendorInventoryItems returns a list of item identifiers from the vendor
func getVendorInventoryItems(ctx *context.Status) []string {
	vendorItems := ctx.Data.Inventory.ByLocation(item.LocationVendor)

	items := make([]string, 0, len(vendorItems))
	for _, itm := range vendorItems {
		// Create a unique identifier for each item
		// Using name + quality + position to ensure uniqueness
		identifier := string(itm.Name) + "|" + string(itm.Quality) + "|" +
			string(rune(itm.Position.X)) + "," + string(rune(itm.Position.Y))
		items = append(items, identifier)
	}

	return items
}

// allItemsStillExist checks if all items from the original list exist in the current list
func allItemsStillExist(originalItems []string, currentItems []string) bool {
	// If no original items stored (first visit), return false to trigger shopping
	if originalItems == nil || len(originalItems) == 0 {
		return false
	}

	// Create a map of current items for faster lookup
	currentItemsMap := make(map[string]bool)
	for _, item := range currentItems {
		currentItemsMap[item] = true
	}

	// Check if all original items still exist
	for _, originalItem := range originalItems {
		if !currentItemsMap[originalItem] {
			// An original item is missing - inventory has changed
			return false
		}
	}

	// All original items still exist
	return true
}

func BuyAtVendor(vendor npc.ID, items ...VendorItemRequest) error {
	ctx := botCtx.Get()
	ctx.SetLastAction("BuyAtVendor")

	if err := InteractNPC(vendor); err != nil {
		return err
	}

	ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)

	for _, i := range items {
		SwitchVendorTab(i.Tab)
		itm, found := ctx.Data.Inventory.Find(i.Item, item.LocationVendor)
		if found {
			town.BuyItem(itm, i.Quantity)
		}
	}

	return step.CloseAllMenus()
}

type VendorItemRequest struct {
	Item     item.Name
	Quantity int
	Tab      int
}

func shouldVisitVendor() bool {
	ctx := botCtx.Get()
	ctx.SetLastStep("shouldVisitVendor")

	if len(town.ItemsToBeSold()) > 0 {
		return true
	}

	if ctx.Data.PlayerUnit.TotalPlayerGold() < 1000 {
		return false
	}

	if ctx.BeltManager.ShouldBuyPotions() || town.ShouldBuyTPs() || town.ShouldBuyIDs() {
		return true
	}

	return false
}

func SwitchVendorTab(tab int) {
	// Ensure any chat messages that could prevent clicking on the tab are cleared
	ClearMessages()
	utils.Sleep(200)

	ctx := context.Get()
	ctx.SetLastStep("switchVendorTab")

	if ctx.GameReader.LegacyGraphics() {
		x := ui.SwitchVendorTabBtnXClassic
		y := ui.SwitchVendorTabBtnYClassic

		tabSize := ui.SwitchVendorTabBtnTabSizeClassic
		x = x + tabSize*tab - tabSize/2
		ctx.HID.Click(game.LeftButton, x, y)
		utils.PingSleep(utils.Medium, 500) // Medium operation: Wait for tab switch
	} else {
		x := ui.SwitchVendorTabBtnX
		y := ui.SwitchVendorTabBtnY

		tabSize := ui.SwitchVendorTabBtnTabSize
		x = x + tabSize*tab - tabSize/2
		ctx.HID.Click(game.LeftButton, x, y)
		utils.PingSleep(utils.Medium, 500) // Medium operation: Wait for tab switch
	}
}
