package action

import (
	"errors"
	"log/slog"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/d2go/pkg/nip"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/town"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
	"github.com/lxn/win"
)

var sharedStashesEmpty = false

// Gamble initiates the gambling process if personal stash gold is high enough
func Gamble() error {
	ctx := context.Get()
	ctx.SetLastAction("Gamble")

	if !ctx.CharacterCfg.Gambling.Enabled {
		return nil
	}
//time.Sleep(5000 * time.Millisecond)
	for {
		stashedGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
		if stashedGold.Value < 2500000 {
			ctx.Logger.Info("Not enough stash gold to gamble, trying to refill from shared stash")

			withdrawGoldFromSharedStash()
			stashAllGold()

			stashedGold, _ = ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
			if stashedGold.Value < 2500000 {
				ctx.Logger.Info("Still not enough gold after refill, stopping gambling")
				return nil
			}
		}

		if err := runGamblingProcess(); err != nil {
			return err
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func runGamblingProcess() error {
	ctx := context.Get()
	ctx.Logger.Info("Time to gamble! Visiting vendor...")
	time.Sleep(500 * time.Millisecond)

	vendorNPC := town.GetTownByArea(ctx.Data.PlayerUnit.Area).GamblingNPC()

	if vendorNPC == npc.Drehya {
		_ = MoveToCoords(data.Position{X: 5107, Y: 5119})
	}

	InteractNPC(vendorNPC)

	if vendorNPC == npc.Jamella {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
	} else {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_DOWN, win.VK_RETURN)
	}

	if !ctx.Data.OpenMenus.NPCShop {
		return errors.New("failed opening gambling window")
	}

	return gambleItems()
}

// GambleSingleItem gambles for specific items and stops if successful
func GambleSingleItem(items []string, desiredQuality item.Quality) error {
	ctx := context.Get()
	ctx.SetLastAction("GambleSingleItem")

	charGold := ctx.Data.PlayerUnit.TotalPlayerGold()
	var itemBought data.Item

	if charGold < 150000 {
		return errors.New("not enough gold to gamble")
	}

	ctx.Logger.Info("Gambling for items", slog.Any("items", items))

	vendorNPC := town.GetTownByArea(ctx.Data.PlayerUnit.Area).GamblingNPC()

	if vendorNPC == npc.Drehya {
		_ = MoveToCoords(data.Position{X: 5107, Y: 5119})
	}

	InteractNPC(vendorNPC)

	if vendorNPC == npc.Jamella {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
	} else {
		ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_DOWN, win.VK_RETURN)
	}

	if !ctx.Data.OpenMenus.NPCShop {
		return errors.New("failed opening gambling window")
	}

	for {
		if itemBought.Name != "" {
			processBoughtItem(&itemBought)
		}

		if ctx.Data.PlayerUnit.TotalPlayerGold() < 1000000 {
			return errors.New("gold is below 1000000, stopping gamble")
		}

		itemFound := false
		for _, itmName := range items {
			itm, found := ctx.Data.Inventory.Find(item.Name(itmName), item.LocationVendor)
			if found {
				town.BuyItem(itm, 1)
				itemBought = itm
				itemFound = true
				break
			}
		}

		if !itemFound {
			refreshGamblingWindow(ctx)
			utils.Sleep(500)
		}
	}
}

func gambleItems() error {
	ctx := context.Get()
	ctx.SetLastAction("gambleItems")

	var itemBought data.Item
	var refreshAttempts int
	var currentItemIndex int
	const maxRefreshAttempts = 11

	for {
		ctx.PauseIfNotPriority()
		ctx.RefreshGameData()

		stashGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
if stashGold.Value < 1000000 {
	ctx.Logger.Info(
		"Finished gambling - stash gold below threshold",
		slog.Int("stashGold", stashGold.Value),
	)
	return step.CloseAllMenus()
}

		if itemBought.Name != "" {
			processBoughtItem(&itemBought)
			refreshAttempts = 0
			currentItemIndex = (currentItemIndex + 1) % len(ctx.Data.CharacterCfg.Gambling.Items)
			continue
		}

		itemFound := false
		if len(ctx.Data.CharacterCfg.Gambling.Items) > 0 {
			currentItem := ctx.Data.CharacterCfg.Gambling.Items[currentItemIndex]
			itm, found := ctx.Data.Inventory.Find(currentItem, item.LocationVendor)
			if found {
				town.BuyItem(itm, 1)
				itemBought = itm
				itemFound = true
			}
		}

		if !itemFound {
			refreshAttempts++
			if refreshAttempts >= maxRefreshAttempts {
				ctx.Logger.Info("Too many refresh attempts, reopening gambling window")
				_ = step.CloseAllMenus()
				utils.Sleep(200)

				vendorNPC := town.GetTownByArea(ctx.Data.PlayerUnit.Area).GamblingNPC()
				_ = InteractNPC(vendorNPC)

				if vendorNPC == npc.Jamella {
					ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
				} else {
					ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_DOWN, win.VK_RETURN)
				}

				refreshAttempts = 0
				continue
			}

			refreshGamblingWindow(ctx)
			utils.Sleep(500)
		}
	}
}

func processBoughtItem(itemBought *data.Item) {
	ctx := context.Get()

	for _, itm := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if itm.UnitID == itemBought.UnitID {
			*itemBought = itm
			break
		}
	}

	if _, result := ctx.Data.CharacterCfg.Runtime.Rules.EvaluateAll(*itemBought); result == nip.RuleResultFullMatch {
		ctx.Logger.Info("Found item matching NIP rules, keeping", slog.Any("item", *itemBought))
	} else {
		town.SellItem(*itemBought)
	}

	*itemBought = data.Item{}
}

func refreshGamblingWindow(ctx *context.Status) {
	if ctx.Data.LegacyGraphics {
		ctx.HID.Click(game.LeftButton, ui.GambleRefreshButtonXClassic, ui.GambleRefreshButtonYClassic)
	} else {
		ctx.HID.Click(game.LeftButton, ui.GambleRefreshButtonX, ui.GambleRefreshButtonY)
	}
}

// withdrawGoldFromSharedStash iterates shared stash tabs
func withdrawGoldFromSharedStash() {
	ctx := context.Get()

	if sharedStashesEmpty {
		ctx.Logger.Info("Skipping shared stash check, previously determined empty")
		return
	}

	// Check if personal stash is already full before doing anything
	stashGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
	if stashGold.Value >= 2500000 {
		ctx.Logger.Info("Personal stash full, skipping withdrawal from shared stash")
		return
	}
time.Sleep(5000 * time.Millisecond)
	// Ensure stash is open initially
	if !ctx.Data.OpenMenus.Stash {
		if err := OpenStash(); err != nil {
			return
		}
		utils.PingSleep(utils.Medium, 300)
		ctx.RefreshGameData()
		time.Sleep(200)
	}

	emptyTabs := 0

	for tab := 0; tab < 3; tab++ {
		for {
			// Before switching tabs, check if personal stash is full
			stashGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
			if stashGold.Value >= 2500000 {
				ctx.Logger.Info("Personal stash full, stopping shared stash withdrawal")
				return
			}

			// Ensure stash is open before switching tabs
			if !ctx.Data.OpenMenus.Stash {
				if err := OpenStash(); err != nil {
					return
				}
				utils.PingSleep(utils.Medium, 300)
				ctx.RefreshGameData()
				time.Sleep(200)
			}

			SwitchStashTab(tab + 2)
			utils.Sleep(200)
			ctx.RefreshGameData()

			hasGold := probeSharedStashTab()
			if !hasGold {
				emptyTabs++

				// 🔴 STASH WAS CLOSED BY ESC — REOPEN IT
				if !ctx.Data.OpenMenus.Stash {
					if err := OpenStash(); err != nil {
						return
					}
					utils.PingSleep(utils.Medium, 300)
					ctx.RefreshGameData()
					time.Sleep(200)
				}

				break
			}

			// Gold was withdrawn → stash it immediately
			time.Sleep(300)
			ctx.RefreshGameData()
			stashAllGold()
			time.Sleep(300)
			ctx.RefreshGameData()
		}

		if emptyTabs >= 3 {
			ctx.Logger.Info("All shared stash tabs are empty, will skip checking them in future")
			sharedStashesEmpty = true
			return
		}
	}
}

func probeSharedStashTab() bool {
	ctx := context.Get()

	if ctx.Data.LegacyGraphics {
		ctx.HID.Click(game.LeftButton, ui.WithdrawStashGoldBtnXClassic, ui.WithdrawStashGoldBtnYClassic)
	} else {
		ctx.HID.Click(game.LeftButton, ui.WithdrawStashGoldBtnX, ui.WithdrawStashGoldBtnY)
	}

	utils.Sleep(150)
	ctx.HID.PressKey(win.VK_ESCAPE)
	utils.Sleep(150)
	ctx.RefreshGameData()

	if ctx.Data.OpenMenus.Stash {
		if ctx.Data.LegacyGraphics {
			ctx.HID.Click(game.LeftButton, ui.WithdrawStashGoldBtnXClassic, ui.WithdrawStashGoldBtnYClassic)
			utils.Sleep(150)
			ctx.HID.Click(game.LeftButton, ui.WithdrawStashGoldBtnConfirmXClassic, ui.WithdrawStashGoldBtnConfirmYClassic)
		} else {
			ctx.HID.Click(game.LeftButton, ui.WithdrawStashGoldBtnX, ui.WithdrawStashGoldBtnY)
			utils.Sleep(150)
			ctx.HID.Click(game.LeftButton, ui.WithdrawStashGoldBtnConfirmX, ui.WithdrawStashGoldBtnConfirmY)
		}

		utils.Sleep(1000)
		ctx.RefreshGameData()
		return true
	}

	return false
}

func stashAllGold() {
	ctx := context.Get()

	if !ctx.Data.OpenMenus.Stash {
		return
	}

	SwitchStashTab(1) // personal stash tab
	utils.Sleep(200)
	ctx.RefreshGameData()
	time.Sleep(300)

	for safeStashGold() {
		// If stash is full, safeStashGold() will return false and break loop
		time.Sleep(200)
	}
}

// Returns true if gold was successfully deposited
func safeStashGold() bool {
	ctx := context.Get()

	if !ctx.Data.OpenMenus.Stash {
		return false
	}

	// Check if personal stash is already full
	stashGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
	if stashGold.Value >= 2500000 {
		ctx.Logger.Info("Personal stash full, stopping gold deposit")
		return false
	}

	beforeGold := stashGold

	// Deposit gold
	if ctx.Data.LegacyGraphics {
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnXClassic, ui.StashGoldBtnYClassic)
		time.Sleep(150)
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnConfirmXClassic, ui.StashGoldBtnConfirmYClassic)
	} else {
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnX, ui.StashGoldBtnY)
		time.Sleep(150)
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnConfirmX, ui.StashGoldBtnConfirmY)
	}

	time.Sleep(200)
	ctx.RefreshGameData()

	afterGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
	return afterGold.Value > beforeGold.Value
}