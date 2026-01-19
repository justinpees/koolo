package action

import (
	"fmt"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/context"
	botCtx "github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/town"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
	"github.com/lxn/win"
)

func Repair() error {
	ctx := context.Get()
	ctx.SetLastAction("Repair")

	for _, i := range ctx.Data.Inventory.ByLocation(item.LocationEquipped) {
		triggerRepair := false
		logMessage := ""

		_, indestructible := i.FindStat(stat.Indestructible, 0)
		quantity, quantityFound := i.FindStat(stat.Quantity, 0)

		// skip indestructible and no quantity
		if indestructible && !quantityFound {
			continue
		}

		// skip eth and no qnt
		if i.Ethereal && !quantityFound {
			continue
		}

		// quantity check
		if quantityFound {
			if quantity.Value < 15 || i.IsBroken {
				triggerRepair = true
				logMessage = fmt.Sprintf("Replenishing %s, quantity is %d", i.Name, quantity.Value)
			}
		} else {
			durability, found := i.FindStat(stat.Durability, 0)
			maxDurability, maxDurabilityFound := i.FindStat(stat.MaxDurability, 0)
			durabilityPercent := -1

			if maxDurabilityFound && found {
				durabilityPercent = int((float64(durability.Value) / float64(maxDurability.Value)) * 100)
			}

			if i.IsBroken || (durabilityPercent != -1 && durabilityPercent <= 20) {
				triggerRepair = true
				logMessage = fmt.Sprintf("Repairing %s, item durability is %d percent", i.Name, durabilityPercent)
			}
		}

		if triggerRepair {
			ctx.Logger.Info(logMessage)

			repairNPC := town.GetTownByArea(ctx.Data.PlayerUnit.Area).RepairNPC()

			if repairNPC == npc.Larzuk {
				MoveToCoords(data.Position{X: 5135, Y: 5046})
			}

			if repairNPC == npc.Hratli {
				if err := FindHratliEverywhere(); err != nil {
					return err
				}
			}

			if err := InteractNPC(repairNPC); err != nil {
				return err
			}

			// Open repair panel
			if repairNPC != npc.Halbu {
				ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
			} else {
				ctx.HID.KeySequence(win.VK_HOME, win.VK_RETURN)
			}

			utils.Sleep(100)

			// Click repair button
			if ctx.Data.LegacyGraphics {
				ctx.HID.Click(game.LeftButton, ui.RepairButtonXClassic, ui.RepairButtonYClassic)
			} else {
				ctx.HID.Click(game.LeftButton, ui.RepairButtonX, ui.RepairButtonY)
			}

			utils.Sleep(300)
			ctx.RefreshGameData()

			// ------------------------------------------------
			// SWITCH TO TRADE TAB AND SHOP AFTER REPAIR
			// ------------------------------------------------
			ctx.HID.KeySequence(win.VK_HOME, win.VK_UP)
			utils.Sleep(80)
			ctx.RefreshGameData()

			plan := repairShoppingPlan()

			if ctx.Data.PlayerUnit.TotalPlayerGold() >= plan.MinGoldReserve {
				items, gold := scanAndPurchaseItems(repairNPC, plan)
				if items > 0 {
					ctx.Logger.Info(
						"Purchased items after repair",
						"items", items,
						"goldSpent", gold,
					)
				}
			}

			return step.CloseAllMenus()
		}
	}

	return nil
}

func repairShoppingPlan() ActionShoppingPlan {
	return ActionShoppingPlan{
		Types: []string{
			string(item.TypeScroll),
			string(item.TypePotion),
		},
		MinGoldReserve: 20000,
	}
}

func RepairRequired() bool {
	ctx := context.Get()
	ctx.SetLastAction("RepairRequired")

	for _, i := range ctx.Data.Inventory.ByLocation(item.LocationEquipped) {
		_, indestructible := i.FindStat(stat.Indestructible, 0)
		quantity, quantityFound := i.FindStat(stat.Quantity, 0)

		if indestructible && !quantityFound {
			continue
		}

		if i.Ethereal && !quantityFound {
			continue
		}

		if quantityFound {
			if quantity.Value < 15 || i.IsBroken {
				return true
			}
		} else {
			durability, found := i.FindStat(stat.Durability, 0)
			maxDurability, maxDurabilityFound := i.FindStat(stat.MaxDurability, 0)

			if i.IsBroken || (maxDurabilityFound && !found) {
				return true
			}

			if found && maxDurabilityFound {
				durabilityPercent := int((float64(durability.Value) / float64(maxDurability.Value)) * 100)
				if durabilityPercent <= 20 {
					return true
				}
			}
		}
	}

	return false
}

func IsEquipmentBroken() bool {
	ctx := context.Get()
	ctx.SetLastAction("EquipmentBroken")

	for _, i := range ctx.Data.Inventory.ByLocation(item.LocationEquipped) {
		_, indestructible := i.FindStat(stat.Indestructible, 0)
		_, quantityFound := i.FindStat(stat.Quantity, 0)

		if i.Ethereal && !quantityFound {
			continue
		}

		if indestructible && !quantityFound {
			continue
		}

		if i.IsBroken {
			ctx.Logger.Debug("Equipment is broken, returning to town", "item", i.Name)
			return true
		}
	}

	return false
}

func FindHratliEverywhere() error {
	ctx := botCtx.Get()
	ctx.SetLastStep("FindHratliEverywhere")

	finalPos := data.Position{X: 5224, Y: 5045}
	MoveToCoords(finalPos)

	_, found := ctx.Data.Monsters.FindOne(npc.Hratli, data.MonsterTypeNone)

	if !found {
		ctx.Logger.Warn("Hratli not found at final position. Moving to start position.")

		startPos := data.Position{X: 5116, Y: 5167}
		MoveToCoords(startPos)

		if err := InteractNPC(npc.Hratli); err != nil {
			ctx.Logger.Warn("Failed to interact with Hratli at start position.", "error", err)
		}

		step.CloseAllMenus()
		return nil
	}

	return nil
}