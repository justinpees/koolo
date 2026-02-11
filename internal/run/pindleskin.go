package run

import (
	"errors"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/data/quest"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
)

var fixedPlaceNearRedPortal = data.Position{
	X: 5130,
	Y: 5120,
}

var pindleSafePosition = data.Position{
	X: 10058,
	Y: 13236,
}

type Pindleskin struct {
	ctx *context.Status
}

func NewPindleskin() *Pindleskin {
	return &Pindleskin{
		ctx: context.Get(),
	}
}

func (p Pindleskin) Name() string {
	return string(config.PindleskinRun)
}

func (p Pindleskin) CheckConditions(parameters *RunParameters) SequencerResult {
	if IsQuestRun(parameters) {
		return SequencerError
	}
	if !p.ctx.Data.Quests[quest.Act5PrisonOfIce].Completed() {
		return SequencerSkip
	}
	return SequencerOk
}

func (p Pindleskin) Run(parameters *RunParameters) error {
	err := action.WayPoint(area.Harrogath)
	if err != nil {
		return err
	}

	_ = action.MoveToCoords(fixedPlaceNearRedPortal)

	redPortal, found := p.ctx.Data.Objects.FindOne(object.PermanentTownPortal)
	if !found {
		if err := action.InteractNPC(npc.Drehya); err != nil {
			return err
		}
		step.CloseAllMenus()
		p.ctx.RefreshGameData()
		redPortal, found = p.ctx.Data.Objects.FindOne(object.PermanentTownPortal)
		if !found {
			return errors.New("red portal not found after talking to anya")
		}
	}

	err = action.InteractObject(redPortal, func() bool {
		return p.ctx.Data.AreaData.Area == area.NihlathaksTemple && p.ctx.Data.AreaData.IsInside(p.ctx.Data.PlayerUnit.Position)
	})
	if err != nil {
		return err
	}

	_ = action.MoveToCoords(pindleSafePosition)
	p.TreatDiabloCloneAsPindleskin() // <- new line

	if err := p.ctx.Char.KillPindle(); err != nil {
		return err
	}

	action.ItemPickup(30)

	return nil
}
func (p *Pindleskin) TreatDiabloCloneAsPindleskin() {
	// Scan all monsters to see if Diablo Clone exists
	for _, m := range p.ctx.Data.Monsters.Enemies(p.ctx.Data.MonsterFilterAnyReachable()) {
		if m.Name == npc.DiabloClone {
			p.ctx.Logger.Info("Diablo Clone detected — treating as Pindleskin")
			// Override its ID so KillPindleskin() sees it as normal Pindleskin
			if m.Type == data.MonsterTypeSuperUnique {
				m.Name = npc.DefiledWarrior
			}
			// Optionally break here if you only care about one clone
			break
		}
	}
}
