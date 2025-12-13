package run

import (
	"errors"
	"strings"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/nip"
	"github.com/hectorgimenez/d2go/pkg/data/quest"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/utils"
)

type Cows struct {
	ctx *context.Status
}

func NewCows() *Cows {
	return &Cows{
		ctx: context.Get(),
	}
}

func (a Cows) Name() string {
	return string(config.CowsRun)
}

func (a Cows) CheckConditions(parameters *RunParameters) SequencerResult {
	if IsQuestRun(parameters) {
		return SequencerError
	}
	if !a.ctx.Data.Quests[quest.Act5EveOfDestruction].Completed() {
		return SequencerSkip
	}
	return SequencerOk
}

func (a Cows) Run(parameters *RunParameters) error {

	// Check if we already have the items in cube so we can skip.
	if a.hasWristAndBookInCube() {

		// Sell junk, refill potions, etc. (basically ensure space for getting the TP tome)
		action.PreRun(false)

		a.ctx.Logger.Info("Wrist Leg and Book found in cube")
		// Move to town if needed
		if !a.ctx.Data.PlayerUnit.Area.IsTown() {
			if err := action.ReturnTown(); err != nil {
				return err
			}
		}

		// Find and interact with stash
		bank, found := a.ctx.Data.Objects.FindOne(object.Bank)
		if !found {
			return errors.New("stash not found")
		}
		err := action.InteractObject(bank, func() bool {
			return a.ctx.Data.OpenMenus.Stash
		})
		if err != nil {
			return err
		}

		// Open cube and transmute Cow Level portal
		if err := action.CubeTransmute(); err != nil {
			return err
		}
		// If we dont have Wirstleg and Book in cube
	} else {
		// First clean up any extra tomes if needed
		err := a.cleanupExtraPortalTomes()
		if err != nil {
			return err
		}

		// Get Wrist leg
		err = a.getWirtsLeg()
		if err != nil {
			return err
		}

		utils.Sleep(500)
		// Sell junk, refill potions, etc. (basically ensure space for getting the TP tome)
		action.PreRun(false)

		utils.Sleep(500)
		err = a.preparePortal()
		if err != nil {
			return err
		}
	}
	// Make sure all menus are closed before interacting with cow portal
	if err := step.CloseAllMenus(); err != nil {
		return err
	}

	// Add a small delay to ensure everything is settled
	utils.Sleep(700)

	townPortal, found := a.ctx.Data.Objects.FindOne(object.PermanentTownPortal)
	if !found {
		return errors.New("cow portal not found")
	}

	err := action.InteractObject(townPortal, func() bool {
		return a.ctx.Data.AreaData.Area == area.MooMooFarm && a.ctx.Data.AreaData.IsInside(a.ctx.Data.PlayerUnit.Position)
	})
	if err != nil {
		return err
	}

	return action.ClearCurrentLevel(a.ctx.CharacterCfg.Game.Cows.OpenChests, data.MonsterAnyFilter())
}

func (a Cows) getWirtsLeg() error {
	if a.hasWirtsLeg() {
		a.ctx.Logger.Info("WirtsLeg found from previous game, we can skip")
		return nil
	}

	err := action.WayPoint(area.StonyField)
	if err != nil {
		return err
	}

	cainStone, found := a.ctx.Data.Objects.FindOne(object.CairnStoneAlpha)
	if !found {
		return errors.New("cain stones not found")
	}
	err = action.MoveToCoords(cainStone.Position)
	if err != nil {
		return err
	}
	action.ClearAreaAroundPlayer(10, data.MonsterAnyFilter())

	portal, found := a.ctx.Data.Objects.FindOne(object.PermanentTownPortal)
	if !found {
		return errors.New("tristram not found")
	}
	err = action.InteractObject(portal, func() bool {
		return a.ctx.Data.AreaData.Area == area.Tristram && a.ctx.Data.AreaData.IsInside(a.ctx.Data.PlayerUnit.Position)
	})
	if err != nil {
		return err
	}

	wirtCorpse, found := a.ctx.Data.Objects.FindOne(object.WirtCorpse)
	if !found {
		return errors.New("wirt corpse not found")
	}

	if err := action.MoveToCoords(wirtCorpse.Position); err != nil {
		return err
	}

	err = action.InteractObject(wirtCorpse, func() bool {
		return a.hasWirtsLeg()
	})

	if err != nil {
		return err
	}

	return action.ReturnTown()
}

func (a Cows) preparePortal() error {
	err := action.WayPoint(area.RogueEncampment)
	if err != nil {
		return err
	}

	
	
		//
	//
	//
	//
//
//
//
// THIS IS WHERE WE CAN FILTER THE LEG BELOW
//
//
//
	//
	//
	//
	//
	
	leg, found := a.ctx.Data.Inventory.Find("WirtsLeg",
		item.LocationStash,
		item.LocationInventory,
		item.LocationCube)
	if !found {
		a.ctx.Logger.Info("WirtsLeg not found – skipping cow run")
    return nil // just skip the run, do not error out
	}
	
	// SKIP COWS IF MAGIC WIRTS LEG IS FOUND
	if leg.Quality == item.QualityMagic {
		a.ctx.Logger.Info("MAGIC WIRTS LEG FOUND IN STASH – SKIPPING COW RUN") // HERE IT IS!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
    return nil
}

	if leg.Quality == item.QualityCrafted {
		if _, result := a.ctx.CharacterCfg.Runtime.Rules.EvaluateAll(leg); result == nip.RuleResultFullMatch {
        a.ctx.Logger.Info("WIRTS LEG IS CRAFTED AND NIP RULE KEPT IT – SKIPPING COW RUN")
        return nil
    }
}
	
	
	
	
	
	
	
	
	
	
	
	

	// Track if we found a usable spare tome
	var spareTome data.Item
	tomeCount := 0
	// Look for an existing spare tome (not in locked inventory slots)
	for _, itm := range a.ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if strings.EqualFold(string(itm.Name), item.TomeOfTownPortal) {
			tomeCount++
			if !action.IsInLockedInventorySlot(itm) {
				spareTome = itm
			}
		}
	}

	//Only 1 tome in inventory, buy one
	if tomeCount <= 1 {
		spareTome = data.Item{}
	}

	// If no spare tome found, buy a new one
	if spareTome.UnitID == 0 {
		err = action.BuyAtVendor(npc.Akara, action.VendorItemRequest{
			Item:     item.TomeOfTownPortal,
			Quantity: 1,
			Tab:      4,
		})
		if err != nil {
			return err
		}

		// Find the newly bought tome (not in locked slots)
		for _, itm := range a.ctx.Data.Inventory.ByLocation(item.LocationInventory) {
			if strings.EqualFold(string(itm.Name), item.TomeOfTownPortal) && !action.IsInLockedInventorySlot(itm) {
				spareTome = itm
				break
			}
		}
	}

	if spareTome.UnitID == 0 {
		return errors.New("failed to obtain spare TomeOfTownPortal for cow portal")
	}

	err = action.CubeAddItems(leg, spareTome)
	if err != nil {
		return err
	}

	return action.CubeTransmute()
}
func (a Cows) cleanupExtraPortalTomes() error {
	// Only attempt cleanup if we don't have Wirt's Leg
	if _, hasLeg := a.ctx.Data.Inventory.Find("WirtsLeg", item.LocationStash, item.LocationInventory, item.LocationCube); !hasLeg {
		// Find all portal tomes, keeping track of which are in locked slots
		var protectedTomes []data.Item
		var unprotectedTomes []data.Item

		for _, itm := range a.ctx.Data.Inventory.ByLocation(item.LocationInventory) {
			if strings.EqualFold(string(itm.Name), item.TomeOfTownPortal) {
				if action.IsInLockedInventorySlot(itm) {
					protectedTomes = append(protectedTomes, itm)
				} else {
					unprotectedTomes = append(unprotectedTomes, itm)
				}
			}
		}

		//Do not drop any tome if only 1 in inventory
		if len(protectedTomes)+len(unprotectedTomes) > 1 {
			// Only drop extra unprotected tomes if we have any
			if len(unprotectedTomes) > 0 {
				a.ctx.Logger.Info("Extra TomeOfTownPortal found - dropping it")
				for i := 0; i < len(unprotectedTomes); i++ {
					err := action.DropInventoryItem(unprotectedTomes[i])
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}








func (a Cows) hasWristAndBookInCube() bool {
	var legItem data.Item // variable to hold the Wirt's Leg
	cubeItems := a.ctx.Data.Inventory.ByLocation(item.LocationCube)

	var hasLeg, hasTome bool
	for _, invItem := range cubeItems { // renamed loop variable to avoid conflicts
		if strings.EqualFold(string(invItem.Name), "WirtsLeg") {
			legItem = invItem
			
			if legItem.Quality <= item.QualitySuperior { // correctly reference package constant
				hasLeg = true
			}
		}
		if strings.EqualFold(string(invItem.Name), "TomeOfTownPortal") {
			hasTome = true
		}
	}

	return hasLeg && hasTome
}












func (a Cows) hasWirtsLeg() bool {
	_, found := a.ctx.Data.Inventory.Find("WirtsLeg",
		item.LocationStash,
		item.LocationInventory,
		item.LocationCube)
	return found
}
