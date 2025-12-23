package action

import "github.com/hectorgimenez/d2go/pkg/data/npc"

func NewTownActionShoppingPlan() ActionShoppingPlan {
	return ActionShoppingPlan{
		Enabled:         true,
		RefreshesPerRun: 0,
		MinGoldReserve:  0,
		Vendors: []npc.ID{
			npc.Akara,
			npc.Charsi,
			npc.Gheed,
			npc.Fara,
			npc.Drognan,
			npc.Elzix,
			npc.Ormus,
			npc.Hratli,
			npc.Asheara,
			npc.Jamella,
			npc.Halbu,
			npc.Malah,
			npc.Larzuk,
			npc.Drehya, //Anya?
		},
		Rules: nil, // use shouldBePickedUp()
		Types: nil, // no type filtering
	}
}