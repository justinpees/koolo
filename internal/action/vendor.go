package action

import (
	"log/slog"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/koolo/internal/action/step"
	botCtx "github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/town"
	"github.com/lxn/win"
)

func VendorRefill(forceRefill bool, sellJunk bool, tempLock ...[][]int) (err error) {
	ctx := botCtx.Get()
	ctx.SetLastAction("VendorRefill")

	if !forceRefill && !shouldVisitVendor() && len(tempLock) == 0 {
		return nil
	}

	ctx.Logger.Info("Visiting vendor...", slog.Bool("forceRefill", forceRefill))

	vendorNPC := town.GetTownByArea(ctx.Data.PlayerUnit.Area).RefillNPC()
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
	if vendorNPC == npc.Jamella {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_RETURN)
	} else {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
	}

	// Sell junk
	if sellJunk {
		var lockConfig [][]int
		if len(tempLock) > 0 {
			lockConfig = tempLock[0]
			town.SellJunk(lockConfig)
		} else {
			town.SellJunk()
		}
	}

	// Buy potions / scrolls
	SwitchStashTab(4)
	ctx.RefreshGameData()
	town.BuyConsumables(forceRefill)

	// ---- SHOP VENDOR USING NIP RULES ----
	// Uses existing shouldMatchRulesOnly() logic
	plan := ActionShoppingPlan{
		Enabled: true,
		Vendors: []npc.ID{vendorNPC},
		// Rules == nil → uses global NIP rules automatically
		// Types == nil → allow all item types
	}

	scanAndPurchaseItems(vendorNPC, plan)

	return step.CloseAllMenus()
}

func BuyAtVendor(vendor npc.ID, items ...VendorItemRequest) error {
	ctx := botCtx.Get()
	ctx.SetLastAction("BuyAtVendor")

	if err := InteractNPC(vendor); err != nil {
		return err
	}

	if vendor == npc.Jamella {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
	} else {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
	}

	for _, i := range items {
		SwitchStashTab(i.Tab)
		itm, found := ctx.Data.Inventory.Find(i.Item, item.LocationVendor)
		if found {
			town.BuyItem(itm, i.Quantity)
		} else {
			ctx.Logger.Warn("Item not found in vendor", slog.String("Item", string(i.Item)))
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