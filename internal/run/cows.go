package run

import (
	"errors"
	"strings"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/data/quest"
	"github.com/hectorgimenez/d2go/pkg/nip"
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

		action.PreRun(false)

		a.ctx.Logger.Info("Wrist Leg and Book found in cube")

		if !a.ctx.Data.PlayerUnit.Area.IsTown() {
			if err := action.ReturnTown(); err != nil {
				return err
			}
		}

		bank, found := a.ctx.Data.Objects.FindOne(object.Bank)
		if !found {
			return errors.New("stash not found")
		}
		if err := action.InteractObject(bank, func() bool {
			return a.ctx.Data.OpenMenus.Stash
		}); err != nil {
			return err
		}

		if err := action.CubeTransmute(); err != nil {
			return err
		}

	} else {
		if err := a.cleanupExtraPortalTomes(); err != nil {
			return err
		}

		if err := a.getWirtsLeg(); err != nil {
			return err
		}

		utils.Sleep(500)
		action.PreRun(false)
		utils.Sleep(500)

		skip, err := a.preparePortal()
		if err != nil {
			return err
		}
		if skip {
			a.ctx.Logger.Info("Cow run skipped")
			return nil
		}
	}

	if err := step.CloseAllMenus(); err != nil {
		return err
	}

	utils.Sleep(700)

	townPortal, found := a.ctx.Data.Objects.FindOne(object.PermanentTownPortal)
	if !found {
		return errors.New("cow portal not found")
	}

	if err := action.InteractObject(townPortal, func() bool {
		return a.ctx.Data.AreaData.Area == area.MooMooFarm &&
			a.ctx.Data.AreaData.IsInside(a.ctx.Data.PlayerUnit.Position)
	}); err != nil {
		return err
	}

	return action.ClearCurrentLevel(
		a.ctx.CharacterCfg.Game.Cows.OpenChests,
		data.MonsterAnyFilter(),
	)
}

func (a Cows) getWirtsLeg() error {
	if a.hasWirtsLeg() {
		a.ctx.Logger.Info("WirtsLeg found from previous game, we can skip")
		return nil
	}

	if err := action.WayPoint(area.StonyField); err != nil {
		return err
	}

	cainStone, found := a.ctx.Data.Objects.FindOne(object.CairnStoneAlpha)
	if !found {
		return errors.New("cain stones not found")
	}

	if err := action.MoveToCoords(cainStone.Position); err != nil {
		return err
	}

	action.ClearAreaAroundPlayer(10, data.MonsterAnyFilter())

	portal, found := a.ctx.Data.Objects.FindOne(object.PermanentTownPortal)
	if !found {
		return errors.New("tristram not found")
	}

	if err := action.InteractObject(portal, func() bool {
		return a.ctx.Data.AreaData.Area == area.Tristram &&
			a.ctx.Data.AreaData.IsInside(a.ctx.Data.PlayerUnit.Position)
	}); err != nil {
		return err
	}

	wirtCorpse, found := a.ctx.Data.Objects.FindOne(object.WirtCorpse)
	if !found {
		return errors.New("wirt corpse not found")
	}

	if err := action.MoveToCoords(wirtCorpse.Position); err != nil {
		return err
	}

	if err := action.InteractObject(wirtCorpse, func() bool {
		return a.hasWirtsLeg()
	}); err != nil {
		return err
	}

	return action.ReturnTown()
}

func (a Cows) preparePortal() (bool, error) {
	if err := action.WayPoint(area.RogueEncampment); err != nil {
		return false, err
	}

	leg, found := a.ctx.Data.Inventory.Find(
		"WirtsLeg",
		item.LocationStash,
		item.LocationInventory,
		item.LocationCube,
	)
	if !found {
		a.ctx.Logger.Info("WirtsLeg not found – skipping cow run")
		return true, nil
	}

	if leg.Quality == item.QualityMagic {
		a.ctx.Logger.Info("MAGIC WIRTS LEG FOUND – SKIPPING COW RUN")
		return true, nil
	}

	if leg.Quality == item.QualityCrafted {
		if _, result := a.ctx.CharacterCfg.Runtime.Rules.EvaluateAll(leg); result == nip.RuleResultFullMatch {
			a.ctx.Logger.Info("CRAFTED WIRTS LEG KEPT BY NIP – SKIPPING COW RUN")
			return true, nil
		}
	}

	var spareTome data.Item
	tomeCount := 0

	for _, itm := range a.ctx.Data.Inventory.ByLocation(item.LocationInventory) {
		if strings.EqualFold(string(itm.Name), item.TomeOfTownPortal) {
			tomeCount++
			if !action.IsInLockedInventorySlot(itm) {
				spareTome = itm
			}
		}
	}

	if tomeCount <= 1 {
		spareTome = data.Item{}
	}

	if spareTome.UnitID == 0 {
		if err := action.BuyAtVendor(npc.Akara, action.VendorItemRequest{
			Item:     item.TomeOfTownPortal,
			Quantity: 1,
			Tab:      4,
		}); err != nil {
			return false, err
		}

		for _, itm := range a.ctx.Data.Inventory.ByLocation(item.LocationInventory) {
			if strings.EqualFold(string(itm.Name), item.TomeOfTownPortal) &&
				!action.IsInLockedInventorySlot(itm) {
				spareTome = itm
				break
			}
		}
	}

	if spareTome.UnitID == 0 {
		return false, errors.New("failed to obtain spare TomeOfTownPortal")
	}

	if err := action.CubeAddItems(leg, spareTome); err != nil {
		return false, err
	}

	return false, action.CubeTransmute()
}

func (a Cows) cleanupExtraPortalTomes() error {
	if _, hasLeg := a.ctx.Data.Inventory.Find(
		"WirtsLeg",
		item.LocationStash,
		item.LocationInventory,
		item.LocationCube,
	); !hasLeg {

		var protected, unprotected []data.Item

		for _, itm := range a.ctx.Data.Inventory.ByLocation(item.LocationInventory) {
			if strings.EqualFold(string(itm.Name), item.TomeOfTownPortal) {
				if action.IsInLockedInventorySlot(itm) {
					protected = append(protected, itm)
				} else {
					unprotected = append(unprotected, itm)
				}
			}
		}

		if len(protected)+len(unprotected) > 1 && len(unprotected) > 0 {
			a.ctx.Logger.Info("Extra TomeOfTownPortal found - dropping it")
			for _, itm := range unprotected {
				if err := action.DropInventoryItem(itm); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (a Cows) hasWristAndBookInCube() bool {
	cubeItems := a.ctx.Data.Inventory.ByLocation(item.LocationCube)

	var hasLeg, hasTome bool
	for _, invItem := range cubeItems {
		if strings.EqualFold(string(invItem.Name), "WirtsLeg") &&
			invItem.Quality <= item.QualitySuperior {
			hasLeg = true
		}
		if strings.EqualFold(string(invItem.Name), "TomeOfTownPortal") {
			hasTome = true
		}
	}
	return hasLeg && hasTome
}

func (a Cows) hasWirtsLeg() bool {
	_, found := a.ctx.Data.Inventory.Find(
		"WirtsLeg",
		item.LocationStash,
		item.LocationInventory,
		item.LocationCube,
	)
	return found
}