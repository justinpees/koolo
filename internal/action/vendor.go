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


func walkToHratli(ctx *context.Status) {
	ctx.Logger.Debug("Walking to Hratli")

	// Define positions to check for Hratli
	finalPos := data.Position{X: 5224, Y: 5045}  // main position
	startPos := data.Position{X: 5116, Y: 5167}  // fallback position

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

	target := data.Position{X: 5090, Y: 5080}

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

	// Sell junk
	if sellJunk {
		if len(tempLock) > 0 {
			town.SellJunk(tempLock[0])
		} else {
			town.SellJunk()
		}
	}


	// Buy consumables

	SwitchVendorTab(4)

	ctx.RefreshGameData()
	town.BuyConsumables(forceRefill)

	// Close refill vendor
	step.CloseAllMenus()
	ctx.RefreshGameData()

	// ---------- SHOP ALL VENDORS IN CURRENT TOWN ----------
	shopPlan := NewTownActionShoppingPlan()
		

	for vendor, vendorArea := range VendorLocationMap {
	if vendorArea != currentArea {
		continue
	}

	ctx.Logger.Debug("Shopping vendor", slog.Int("vendor", int(vendor)))

// 🔑 SPECIAL CASE: Walk to Hratli first
	if vendor == npc.Hratli {
		walkToHratli(ctx)
	}

	// 🔑 SPECIAL CASE: Walk to Larzuk first
	if vendor == npc.Larzuk {
		walkToLarzuk(ctx)
	}
	
	// 🔑 SPECIAL CASE: Walk to Larzuk first
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
		scanAndPurchaseItems(vendor, shopPlan)

		step.CloseAllMenus()
		ctx.RefreshGameData()
	}

	return nil
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

