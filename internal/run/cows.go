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

var ErrSkipCowRun = errors.New("skip cow run")

type Cows struct {
	ctx *context.Status
}

func NewCows() *Cows {
	return &Cows{ctx: context.Get()}
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

	// If leg + tome already in cube, just open cows
	if a.hasWristAndBookInCube() {

		action.PreRun(false)

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

		utils.Sleep(300)
		action.PreRun(false)
		utils.Sleep(300)

		if err := a.preparePortal(); err != nil {
			if errors.Is(err, ErrSkipCowRun) {
				return nil
			}
			return err
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

func (a Cows) preparePortal() error {

	if err := action.WayPoint(area.RogueEncampment); err != nil {
		return err
	}

	leg, found := a.ctx.Data.Inventory.Find(
		"WirtsLeg",
		item.LocationStash,
		item.LocationInventory,
		item.LocationCube,
	)

	if !found {
		return nil
	}
	if a.ctx.CharacterCfg.Game.Cows.CraftWirtsLeg {
		// 🔴 MAGIC LEG — always stash & skip
		if leg.Quality == item.QualityMagic {
			a.ctx.Logger.Info("Magic Wirt's Leg detected — stashing and skipping cows")
			_ = action.Stash(true)
			return ErrSkipCowRun
		}

		// 🔴 SOCKETED NORMAL LEG — stash & skip
		if leg.Quality == item.QualityNormal && leg.HasSockets {
			a.ctx.Logger.Info("Socketed Wirt's Leg detected — stashing and skipping cows")
			_ = action.Stash(true)
			return ErrSkipCowRun
		}

		// 🟡 CRAFTED LEG — consult NIP
		if leg.Quality == item.QualityCrafted {
			_, result := a.ctx.CharacterCfg.Runtime.Rules.EvaluateAll(leg)

			if result == nip.RuleResultFullMatch {
				a.ctx.Logger.Info("Crafted Wirt's Leg is a NIP keeper — stashing and skipping cows")
				_ = action.Stash(true)
				return ErrSkipCowRun
			}

			a.ctx.Logger.Info("Crafted Wirt's Leg is NOT a keeper — using it for cows")
		}

		// 🛑 SAFETY NET — never cube a keeper
		if leg.Quality >= item.QualityMagic {
			if _, result := a.ctx.CharacterCfg.Runtime.Rules.EvaluateAll(leg); result == nip.RuleResultFullMatch {
				a.ctx.Logger.Warn("Safety abort: keeper Wirt's Leg would be cubed")
				_ = action.Stash(true)
				return ErrSkipCowRun
			}
		}
	}
	// ===== Tome handling =====

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
			return err
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
		return errors.New("failed to obtain TomeOfTownPortal")
	}

	if err := action.CubeAddItems(leg, spareTome); err != nil {
		return err
	}

	return action.CubeTransmute()
}

func (a Cows) cleanupExtraPortalTomes() error {

	if _, hasLeg := a.ctx.Data.Inventory.Find(
		"WirtsLeg",
		item.LocationStash,
		item.LocationInventory,
		item.LocationCube,
	); hasLeg {
		return nil
	}

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

	if len(protected)+len(unprotected) > 1 {
		for _, itm := range unprotected {
			_ = action.DropInventoryItem(itm)
		}
	}

	return nil
}

func (a Cows) hasWristAndBookInCube() bool {
	var hasLeg, hasTome bool

	for _, itm := range a.ctx.Data.Inventory.ByLocation(item.LocationCube) {
		if a.ctx.CharacterCfg.Game.Cows.CraftWirtsLeg {
			if itm.Name == "WirtsLeg" && itm.Quality < item.QualityMagic {
				hasLeg = true
			}
		} else {
			// If CraftWirtsLeg is false, just check if any Wirt's Leg exists
			if itm.Name == "WirtsLeg" {
				hasLeg = true
			}
		}

		if itm.Name == item.TomeOfTownPortal {
			hasTome = true
		}
	}

	return hasLeg && hasTome
}

func (a Cows) HasWirtsLeg() bool {
	_, found := a.ctx.Data.Inventory.Find(
		"WirtsLeg",
		item.LocationStash,
		item.LocationInventory,
		item.LocationCube,
	)
	return found
}

func (a Cows) getWirtsLeg() error {

	if a.HasWirtsLeg() {
		return nil
	}

	if err := action.WayPoint(area.StonyField); err != nil {
		return err
	}

	cairn, found := a.ctx.Data.Objects.FindOne(object.CairnStoneAlpha)
	if !found {
		return errors.New("cairn stones not found")
	}

	if err := action.MoveToCoords(cairn.Position); err != nil {
		return err
	}

	action.ClearAreaAroundPlayer(10, data.MonsterAnyFilter())

	portal, found := a.ctx.Data.Objects.FindOne(object.PermanentTownPortal)
	if !found {
		return errors.New("tristram portal not found")
	}

	if err := action.InteractObject(portal, func() bool {
		return a.ctx.Data.AreaData.Area == area.Tristram
	}); err != nil {
		return err
	}

	wirt, found := a.ctx.Data.Objects.FindOne(object.WirtCorpse)
	if !found {
		return errors.New("wirt corpse not found")
	}

	if err := action.MoveToCoords(wirt.Position); err != nil {
		return err
	}

	if err := action.InteractObject(wirt, func() bool {
		return a.HasWirtsLeg()
	}); err != nil {
		return err
	}

	return action.ReturnTown()
}
