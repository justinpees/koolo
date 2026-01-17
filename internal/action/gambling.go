package action

import (
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/d2go/pkg/nip"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
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

	const minStashGold = 2500000

	if ctx.CharacterCfg.Gambling.GambleFromSharedStashes && ctx.CharacterCfg.Gambling.Enabled {
		// Shared stash gambling logic
		for {
			stashedGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)

			if stashedGold.Value < minStashGold {
				ctx.Logger.Info("Not enough stash gold to gamble, trying to refill from shared stash")

				withdrawGoldFromSharedStash()
				stashAllGold()

				stashedGold, _ = ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
				if stashedGold.Value < minStashGold {
					ctx.Logger.Info("Still not enough gold after refill, stopping gambling")
					return nil
				}
			}

			if err := runGamblingProcess(); err != nil {
				return err
			}

			// ✅ Reset shared stash empty flag after gambling
			if sharedStashesEmpty {
				ctx.Logger.Debug("Resetting sharedStashesEmpty after gambling")
				sharedStashesEmpty = false
			}

			break // stop loop after one gambling run
		}
	} else {
		// Original gambling logic without shared stash
		stashedGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)

		if stashedGold.Value >= minStashGold {
			ctx.Logger.Info("Time to gamble! Visiting vendor...")

			vendorNPC := town.GetTownByArea(ctx.Data.PlayerUnit.Area).GamblingNPC()

			// Fix for Anya position
			if vendorNPC == npc.Drehya {
				_ = MoveToCoords(data.Position{
					X: 5107,
					Y: 5119,
				})
			}

			InteractNPC(vendorNPC)
			// Jamella gamble button is the second one
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
	}

	return nil
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

	const minGold = 150000
	var itemBought data.Item

	// Check if we should gamble from shared stash
	if ctx.CharacterCfg.Gambling.Enabled && ctx.CharacterCfg.Gambling.GambleFromSharedStashes {
		for {
			charGold := ctx.Data.PlayerUnit.TotalPlayerGold()

			if charGold < minGold {
				ctx.Logger.Info("Not enough gold to gamble, trying to refill from shared stash")
				withdrawGoldFromSharedStash()
				stashAllGold()

				charGold = ctx.Data.PlayerUnit.TotalPlayerGold()
				if charGold < minGold {
					ctx.Logger.Info("Still not enough gold after refill, stopping gambling")
					return nil
				}
			}

			// Run the normal single-item gambling loop
			if err := runGambleSingleItemProcess(items, desiredQuality, &itemBought); err != nil {
				return err
			}

			break // stop loop after one run
		}

	} else if ctx.CharacterCfg.Gambling.Enabled && !ctx.CharacterCfg.Gambling.GambleFromSharedStashes {
		// Original gambling logic without shared stash
		charGold := ctx.Data.PlayerUnit.TotalPlayerGold()
		if charGold >= minGold {
			return runGambleSingleItemProcess(items, desiredQuality, &itemBought)
		}
	}

	return nil
}

// Helper to avoid duplicating the gambling loop
func runGambleSingleItemProcess(items []string, desiredQuality item.Quality, itemBought *data.Item) error {
	ctx := context.Get()

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
			for _, itm := range ctx.Data.Inventory.ByLocation(item.LocationInventory) {
				if itm.UnitID == itemBought.UnitID {
					*itemBought = itm
					ctx.Logger.Debug("Gambled for item", slog.Any("item", *itemBought))
					break
				}
			}

			if _, result := ctx.Data.CharacterCfg.Runtime.Rules.EvaluateAll(*itemBought); result == nip.RuleResultFullMatch {
				ctx.Logger.Info("Found item matching nip rules, will be kept", slog.Any("item", *itemBought))
				*itemBought = data.Item{}
				continue
			} else {
				if itemBought.Quality == desiredQuality {
					ctx.Logger.Info("Found item matching desired quality, will be kept", slog.Any("item", *itemBought))
					return step.CloseAllMenus()
				} else {
					town.SellItem(*itemBought)
					*itemBought = data.Item{}
				}
			}
		}

		if ctx.Data.PlayerUnit.TotalPlayerGold() < 150000 {
			return errors.New("gold is below 150000, stopping gamble")
		}

		for _, itmName := range items {
			itm, found := ctx.Data.Inventory.Find(item.Name(itmName), item.LocationVendor)
			if found {
				town.BuyItem(itm, 1)
				*itemBought = itm
				break
			}
		}

		if itemBought.Name == "" {
			ctx.Logger.Debug("Desired items not found in gambling window, refreshing...", slog.Any("items", items))
			if ctx.Data.LegacyGraphics {
				ctx.HID.Click(game.LeftButton, ui.GambleRefreshButtonXClassic, ui.GambleRefreshButtonYClassic)
			} else {
				ctx.HID.Click(game.LeftButton, ui.GambleRefreshButtonX, ui.GambleRefreshButtonY)
			}
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

	// 🔎 DEBUG: confirm final gamble state
	ctx.Logger.Warn(
		"GAMBLED ITEM FINAL STATE",
		"name", itemBought.Name,
		"quality", itemBought.Quality,
	)

	// NIP match → keep normally
	if _, result := ctx.Data.CharacterCfg.Runtime.Rules.EvaluateAll(*itemBought); result == nip.RuleResultFullMatch {
		ctx.Logger.Info("Found item matching NIP rules, keeping", slog.Any("item", *itemBought))
		*itemBought = data.Item{} // 🔑 REQUIRED
		return
	}

	keep := false

	if slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Rare Item") &&
		ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint == "" &&
		itemBought.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) &&
		itemBought.Quality == item.QualityRare &&
		ctx.CharacterCfg.CubeRecipes.RareMinMonsterLevel >= 90 {

		// Helper closure to reduce duplication
		markAndKeep := func() {
			keep = true
			fp := SpecificRareFingerprint(*itemBought)

			ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint = fp
			ctx.MarkedRareSpecificItemUnitID = itemBought.UnitID

			ctx.Logger.Warn(
				"SAVED GAMBLED RARE ITEM — MARKED FOR STASH",
				"fp", fp,
				"unitID", itemBought.UnitID,
			)

			if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
				ctx.Logger.Error("FAILED TO SAVE CharacterCfg AFTER UPDATING FINGERPRINT", "err", err)
			}
		}

		if levelStat, ok := ctx.Data.PlayerUnit.FindStat(stat.Level, 0); ok {

			switch ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll {
			case "Circlet":
				if levelStat.Value >= 92 {
					markAndKeep()
				}

			case "Coronet":
				if levelStat.Value >= 87 {
					markAndKeep()
				}

			case "Tiara":
				if levelStat.Value >= 82 {
					markAndKeep()
				}

			case "Diadem":
				if levelStat.Value >= 77 {
					markAndKeep()
				}

			case "Amulet":
				if levelStat.Value >= 95 {
					markAndKeep()
				}
			}
		}
	}

	if !keep {
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
	if ctx.CharacterCfg.ClassicMode {
		if !ctx.Data.LegacyGraphics { // wait only if legacy graphics is not active yet
			ctx.Logger.Info("Waiting up to 5 seconds for Legacy Graphics to activate...")
			if !waitForLegacyGraphics(5 * time.Second) {
				ctx.Logger.Warn("Legacy Graphics not detected after 5 seconds, proceeding anyway")
			} else {
				ctx.Logger.Info("Legacy Graphics detected, continuing...")
			}
		}
	}
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
			ctx.Logger.Info("All shared stash tabs are empty, will skip checking them until we gamble next time")
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
			utils.Sleep(1000)
			ctx.HID.Click(game.LeftButton, ui.WithdrawStashGoldBtnConfirmXClassic, ui.WithdrawStashGoldBtnConfirmYClassic)
		} else {
			ctx.HID.Click(game.LeftButton, ui.WithdrawStashGoldBtnX, ui.WithdrawStashGoldBtnY)
			utils.Sleep(1000)
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
	utils.Sleep(1000)
	ctx.RefreshGameData()
	time.Sleep(1000)

	for safeStashGold() {
		// If stash is full, safeStashGold() will return false and break loop
		time.Sleep(200)
	}
}

// Returns true if gold was successfully deposited
func safeStashGold() bool {
	ctx := context.Get()

	// Stash must be open
	if !ctx.Data.OpenMenus.Stash {
		return false
	}

	// Check if personal stash is already full
	stashGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
	if stashGold.Value >= 2500000 {
		ctx.Logger.Info("Personal stash full, stopping gold deposit")

		// 🔑 IMPORTANT: clean up UI state
		step.CloseAllMenus()
		ctx.RefreshGameData()

		return false
	}

	beforeGold := stashGold.Value

	// Deposit gold
	if ctx.Data.LegacyGraphics {
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnXClassic, ui.StashGoldBtnYClassic)
		time.Sleep(1000)
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnConfirmXClassic, ui.StashGoldBtnConfirmYClassic)
	} else {
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnX, ui.StashGoldBtnY)
		time.Sleep(1000)
		ctx.HID.Click(game.LeftButton, ui.StashGoldBtnConfirmX, ui.StashGoldBtnConfirmY)
	}

	time.Sleep(200)
	ctx.RefreshGameData()

	afterGold, _ := ctx.Data.PlayerUnit.FindStat(stat.StashGold, 0)
	return afterGold.Value > beforeGold
}

// waitForLegacyGraphics waits until LegacyGraphics is true or timeout is reached
func waitForLegacyGraphics(timeout time.Duration) bool {
	ctx := context.Get()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if ctx.Data.LegacyGraphics {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}

	return false
}
