package action

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/d2go/pkg/nip"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
)

type CubeRecipe struct {
	Name             string
	Items            []string
	PurchaseRequired bool
	PurchaseItems    []string
}

var (
	Recipes = []CubeRecipe{

		// Perfects
		{
			Name:  "Flawed Amethyst",
			Items: []string{"ChippedAmethyst", "ChippedAmethyst", "ChippedAmethyst"},
		},

		{
			Name:  "Flawed Diamond",
			Items: []string{"ChippedDiamond", "ChippedDiamond", "ChippedDiamond"},
		},

		{
			Name:  "Flawed Emerald",
			Items: []string{"ChippedEmerald", "ChippedEmerald", "ChippedEmerald"},
		},

		{
			Name:  "Flawed Ruby",
			Items: []string{"ChippedRuby", "ChippedRuby", "ChippedRuby"},
		},

		{
			Name:  "Flawed Sapphire",
			Items: []string{"ChippedSapphire", "ChippedSapphire", "ChippedSapphire"},
		},

		{
			Name:  "Flawed Topaz",
			Items: []string{"ChippedTopaz", "ChippedTopaz", "ChippedTopaz"},
		},

		{
			Name:  "Flawed Skull",
			Items: []string{"ChippedSkull", "ChippedSkull", "ChippedSkull"},
		},

		{
			Name:  "Amethyst",
			Items: []string{"FlawedAmethyst", "FlawedAmethyst", "FlawedAmethyst"},
		},

		{
			Name:  "Diamond",
			Items: []string{"FlawedDiamond", "FlawedDiamond", "FlawedDiamond"},
		},

		{
			Name:  "Emerald",
			Items: []string{"FlawedEmerald", "FlawedEmerald", "FlawedEmerald"},
		},

		{
			Name:  "Ruby",
			Items: []string{"FlawedRuby", "FlawedRuby", "FlawedRuby"},
		},

		{
			Name:  "Sapphire",
			Items: []string{"FlawedSapphire", "FlawedSapphire", "FlawedSapphire"},
		},

		{
			Name:  "Topaz",
			Items: []string{"FlawedTopaz", "FlawedTopaz", "FlawedTopaz"},
		},

		{
			Name:  "Skull",
			Items: []string{"FlawedSkull", "FlawedSkull", "FlawedSkull"},
		},

		{
			Name:  "Flawless Amethyst",
			Items: []string{"Amethyst", "Amethyst", "Amethyst"},
		},

		{
			Name:  "Flawless Diamond",
			Items: []string{"Diamond", "Diamond", "Diamond"},
		},

		{
			Name:  "Flawless Emerald",
			Items: []string{"Emerald", "Emerald", "Emerald"},
		},

		{
			Name:  "Flawless Ruby",
			Items: []string{"Ruby", "Ruby", "Ruby"},
		},

		{
			Name:  "Flawless Sapphire",
			Items: []string{"Sapphire", "Sapphire", "Sapphire"},
		},

		{
			Name:  "Flawless Topaz",
			Items: []string{"Topaz", "Topaz", "Topaz"},
		},

		{
			Name:  "Flawless Skull",
			Items: []string{"Skull", "Skull", "Skull"},
		},

		{
			Name:  "Perfect Amethyst",
			Items: []string{"FlawlessAmethyst", "FlawlessAmethyst", "FlawlessAmethyst"},
		},

		{
			Name:  "Perfect Diamond",
			Items: []string{"FlawlessDiamond", "FlawlessDiamond", "FlawlessDiamond"},
		},

		{
			Name:  "Perfect Emerald",
			Items: []string{"FlawlessEmerald", "FlawlessEmerald", "FlawlessEmerald"},
		},

		{
			Name:  "Perfect Ruby",
			Items: []string{"FlawlessRuby", "FlawlessRuby", "FlawlessRuby"},
		},

		{
			Name:  "Perfect Sapphire",
			Items: []string{"FlawlessSapphire", "FlawlessSapphire", "FlawlessSapphire"},
		},

		{
			Name:  "Perfect Topaz",
			Items: []string{"FlawlessTopaz", "FlawlessTopaz", "FlawlessTopaz"},
		},

		{
			Name:  "Perfect Skull",
			Items: []string{"FlawlessSkull", "FlawlessSkull", "FlawlessSkull"},
		},

		// Token
		{
			Name:  "Token of Absolution",
			Items: []string{"TwistedEssenceOfSuffering", "ChargedEssenceOfHatred", "BurningEssenceOfTerror", "FesteringEssenceOfDestruction"},
		},

		// Runes
		{
			Name:  "Upgrade El",
			Items: []string{"ElRune", "ElRune", "ElRune"},
		},
		{
			Name:  "Upgrade Eld",
			Items: []string{"EldRune", "EldRune", "EldRune"},
		},
		{
			Name:  "Upgrade Tir",
			Items: []string{"TirRune", "TirRune", "TirRune"},
		},
		{
			Name:  "Upgrade Nef",
			Items: []string{"NefRune", "NefRune", "NefRune"},
		},
		{
			Name:  "Upgrade Eth",
			Items: []string{"EthRune", "EthRune", "EthRune"},
		},
		{
			Name:  "Upgrade Ith",
			Items: []string{"IthRune", "IthRune", "IthRune"},
		},
		{
			Name:  "Upgrade Tal",
			Items: []string{"TalRune", "TalRune", "TalRune"},
		},
		{
			Name:  "Upgrade Ral",
			Items: []string{"RalRune", "RalRune", "RalRune"},
		},
		{
			Name:  "Upgrade Ort",
			Items: []string{"OrtRune", "OrtRune", "OrtRune"},
		},
		{
			Name:  "Upgrade Thul",
			Items: []string{"ThulRune", "ThulRune", "ThulRune", "ChippedTopaz"},
		},
		{
			Name:  "Upgrade Amn",
			Items: []string{"AmnRune", "AmnRune", "AmnRune", "ChippedAmethyst"},
		},
		{
			Name:  "Upgrade Sol",
			Items: []string{"SolRune", "SolRune", "SolRune", "ChippedSapphire"},
		},
		{
			Name:  "Upgrade Shael",
			Items: []string{"ShaelRune", "ShaelRune", "ShaelRune", "ChippedRuby"},
		},
		{
			Name:  "Upgrade Dol",
			Items: []string{"DolRune", "DolRune", "DolRune", "ChippedEmerald"},
		},
		{
			Name:  "Upgrade Hel",
			Items: []string{"HelRune", "HelRune", "HelRune", "ChippedDiamond"},
		},
		{
			Name:  "Upgrade Io",
			Items: []string{"IoRune", "IoRune", "IoRune", "FlawedTopaz"},
		},
		{
			Name:  "Upgrade Lum",
			Items: []string{"LumRune", "LumRune", "LumRune", "FlawedAmethyst"},
		},
		{
			Name:  "Upgrade Ko",
			Items: []string{"KoRune", "KoRune", "KoRune", "FlawedSapphire"},
		},
		{
			Name:  "Upgrade Fal",
			Items: []string{"FalRune", "FalRune", "FalRune", "FlawedRuby"},
		},
		{
			Name:  "Upgrade Lem",
			Items: []string{"LemRune", "LemRune", "LemRune", "FlawedEmerald"},
		},
		{
			Name:  "Upgrade Pul",
			Items: []string{"PulRune", "PulRune", "FlawedDiamond"},
		},
		{
			Name:  "Upgrade Um",
			Items: []string{"UmRune", "UmRune", "Topaz"},
		},
		{
			Name:  "Upgrade Mal",
			Items: []string{"MalRune", "MalRune", "Amethyst"},
		},
		{
			Name:  "Upgrade Ist",
			Items: []string{"IstRune", "IstRune", "Sapphire"},
		},
		{
			Name:  "Upgrade Gul",
			Items: []string{"GulRune", "GulRune", "Ruby"},
		},
		{
			Name:  "Upgrade Vex",
			Items: []string{"VexRune", "VexRune", "Emerald"},
		},
		{
			Name:  "Upgrade Ohm",
			Items: []string{"OhmRune", "OhmRune", "Diamond"},
		},
		{
			Name:  "Upgrade Lo",
			Items: []string{"LoRune", "LoRune", "FlawlessTopaz"},
		},
		{
			Name:  "Upgrade Sur",
			Items: []string{"SurRune", "SurRune", "FlawlessAmethyst"},
		},
		{
			Name:  "Upgrade Ber",
			Items: []string{"BerRune", "BerRune", "FlawlessSapphire"},
		},
		{
			Name:  "Upgrade Jah",
			Items: []string{"JahRune", "JahRune", "FlawlessRuby"},
		},
		{
			Name:  "Upgrade Cham",
			Items: []string{"ChamRune", "ChamRune", "FlawlessEmerald"},
		},

		// STEP 1: MAKE WIRTSLEG MAGIC
		{
			Name:             "MagicWirtsLegStep1",
			Items:            []string{"WirtsLeg", "", "", ""},
			PurchaseRequired: false,
		},

		// STEP 2: MAKE WIRTSLEG CRAFTED
		{
			Name:             "MagicWirtsLegStep2",
			Items:            []string{"WirtsLeg", "TirRune", "PerfectSapphire", "Jewel"},
			PurchaseRequired: false,
		},

		// add sockets
		{
			Name:  "Add Sockets to Weapon",
			Items: []string{"RalRune", "AmnRune", "PerfectAmethyst", "NormalWeapon"},
		},
		{
			Name:  "Add Sockets to Armor",
			Items: []string{"TalRune", "ThulRune", "PerfectTopaz", "NormalArmor"},
		},
		{
			Name:  "Add Sockets to Helm",
			Items: []string{"RalRune", "ThulRune", "PerfectSapphire", "NormalHelm"},
		},
		{
			Name:  "Add Sockets to Shield",
			Items: []string{"TalRune", "AmnRune", "PerfectRuby", "NormalShield"},
		},

		// Caster Ring
		{
			Name:             "Caster Ring",
			Items:            []string{"AmnRune", "PerfectAmethyst", "Jewel", "Ring"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"Ring"},
		},

		// Caster Belt
		{
			Name:             "Caster Belt (VampirefangBelt)",
			Items:            []string{"VampirefangBelt", "IthRune", "PerfectAmethyst", "Jewel"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"SharkskinBelt", "VampirefangBelt"},
		},

		// Caster Boots
		{
			Name:             "Caster Boots",
			Items:            []string{"ThulRune", "PerfectAmethyst", "Jewel", "WyrmhideBoots"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"DemonhideBoots", "WyrmhideBoots"},
		},

		// Blood Amulet
		{
			Name:             "Blood Amulet",
			Items:            []string{"AmnRune", "PerfectRuby", "Jewel"},
			PurchaseRequired: true,
			PurchaseItems:    []string{"Amulet"},
		},

		// Blood Ring
		{
			Name:             "Blood Ring",
			Items:            []string{"SolRune", "PerfectRuby", "Jewel", "Ring"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"Ring"},
		},

		// Blood Gloves
		{
			Name:             "Blood Gloves (VampireboneGloves)",
			Items:            []string{"VampireboneGloves", "NefRune", "PerfectRuby", "Jewel"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"VampireboneGloves"},
		},

		// Blood Boots
		{
			Name:             "Blood Boots",
			Items:            []string{"EthRune", "PerfectRuby", "Jewel", "BattleBoots"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"BattleBoots"},
		},

		// Blood Belt
		{
			Name:             "Blood Belt (MithrilCoil)",
			Items:            []string{"MithrilCoil", "TalRune", "PerfectRuby", "Jewel"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"MeshBelt", "MithrilCoil"},
		},

		// Blood Helm
		{
			Name:             "Blood Helm (Armet)",
			Items:            []string{"Armet", "RalRune", "PerfectRuby", "Jewel"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"Armet"},
		},

		// Blood Armor
		{
			Name:             "Blood Armor",
			Items:            []string{"ThulRune", "PerfectRuby", "Jewel", "HellforgePlate"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"PlateMail", "TemplarPlate", "HellforgePlate"},
		},

		// Blood Weapon
		{
			Name:             "Blood Weapon",
			Items:            []string{"OrtRune", "PerfectRuby", "Jewel", "BerserkerAxe"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"BerserkerAxe"},
		},

		// Safety Shield
		{
			Name:             "Safety Shield",
			Items:            []string{"NefRune", "PerfectEmerald", "Jewel", "Monarch"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"KiteShield", "DragonShield", "Monarch"},
		},

		// Safety Armor
		{
			Name:             "Safety Armor",
			Items:            []string{"EthRune", "PerfectEmerald", "Jewel", "GreatHauberk"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"BreastPlate", "Curiass", "GreatHauberk"},
		},

		// Safety Boots
		{
			Name:             "Safety Boots",
			Items:            []string{"OrtRune", "PerfectEmerald", "Jewel", "WarBoots"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"Greaves", "WarBoots", "MyrmidonBoots"},
		},

		// Blood Gloves
		{
			Name:             "Blood Gloves (SharkskinGloves)",
			Items:            []string{"SharkskinGloves", "NefRune", "PerfectRuby", "Jewel"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"Gauntlets", "WarGauntlets", "OgreGauntlets"},
		},

		// Blood Belt
		{
			Name:             "Blood Belt (MeshBelt)",
			Items:            []string{"MeshBelt", "TalRune", "PerfectRuby", "Jewel"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"Sash", "DemonhideSash", "SpiderwebSash"},
		},

		// Safety Helm
		{
			Name:             "Safety Helm",
			Items:            []string{"IthRune", "PerfectEmerald", "Jewel", "GrandCrown"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"Crown", "GrandCrown", "Corona"},
		},

		// Hitpower Gloves
		{
			Name:             "Hitpower Gloves (HeavyBracers)",
			Items:            []string{"OrtRune", "PerfectSapphire", "Jewel", "HeavyBracers"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"ChainGloves", "HeavyBracers", "Vambraces"},
		},

		// Hitpower Boots
		{
			Name:             "Hitpower Boots",
			Items:            []string{"RalRune", "PerfectSapphire", "Jewel", "MeshBoots"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"ChainBoots", "MeshBoots", "Boneweave"},
		},

		// Caster Belt
		{
			Name:             "Caster Belt (SharkskinBelt)",
			Items:            []string{"SharkskinBelt", "IthRune", "PerfectAmethyst", "Jewel"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"HeavyBelt", "BattleBelt", "TrollBelt"},
		},

		// Hitpower Helm
		{
			Name:             "Hitpower Helm",
			Items:            []string{"NefRune", "PerfectSapphire", "Jewel", "GiantConch"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"FullHelm", "Basinet", "GiantConch"},
		},

		// Hitpower Armor
		{
			Name:             "Hitpower Armor",
			Items:            []string{"EthRune", "PerfectSapphire", "Jewel", "KrakenShell"},
			PurchaseRequired: false,
			PurchaseItems:    []string{"FieldPlate", "Sharktooth", "KrakenShell"},
		},

		// Caster Amulet
		{
			Name:             "Caster Amulet",
			Items:            []string{"RalRune", "PerfectAmethyst", "Jewel"},
			PurchaseRequired: true,
			PurchaseItems:    []string{"Amulet"},
		},

		/* // Reroll Grand Charms
		{
			Name:  "Reroll GrandCharms",
			Items: []string{"GrandCharm", "Perfect", "Perfect", "Perfect"}, // Special handling in hasItemsForRecipe
		}, */

		// Reroll Specific Magic Item
		{
			Name:  "Reroll Specific Magic Item",
			Items: []string{"Specificitem", "Perfect", "Perfect", "Perfect"}, // Special handling in hasItemsForRecipe
		},

		// Reroll Specific Rare Item
		{
			Name:  "Reroll Specific Rare Item",
			Items: []string{"Specificitem", "PerfectSkull", "PerfectSkull", "PerfectSkull", "PerfectSkull", "PerfectSkull", "PerfectSkull"}, // Special handling in hasItemsForRecipe
		},
	}
)

func isRecipeEnabled(name string, recipes []string) bool {
	for _, r := range recipes {
		if r == name {
			return true
		}
	}
	return false
}

func CubeRecipes() error {
	ctx := context.Get()
	ctx.SetLastAction("CubeRecipes")

	// If cubing is disabled from settings just return nil
	if !ctx.CharacterCfg.CubeRecipes.Enabled {
		ctx.Logger.Debug("Cube recipes are disabled, skipping")
		return nil
	}

	itemsInStash := ctx.Data.Inventory.ByLocation(item.LocationStash, item.LocationSharedStash)

	for _, recipe := range Recipes {
		// Check if the current recipe is Enabled
		if !slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, recipe.Name) {
			continue
		}

		ctx.Logger.Debug("Cube recipe is enabled, processing", "recipe", recipe.Name)

		isMagicReroll := recipe.Name == "Reroll Specific Magic Item"
		isRareReroll := recipe.Name == "Reroll Specific Rare Item"

		continueProcessing := true
		for continueProcessing {
			if items, hasItems := hasItemsForRecipe(ctx, recipe); hasItems {

				if recipe.PurchaseRequired {
					err := GambleSingleItem(recipe.PurchaseItems, item.QualityMagic)
					if err != nil {
						ctx.Logger.Error("Error gambling item, skipping recipe", "error", err, "recipe", recipe.Name)
						break
					}

					purchasedItem := getPurchasedItem(ctx, recipe.PurchaseItems)
					if purchasedItem.Name == "" {
						ctx.Logger.Debug("Could not find purchased item. Skipping recipe", "recipe", recipe.Name)
						break
					}

					items = append(items, purchasedItem)
				}

				if err := CubeAddItems(items...); err != nil {
					return err
				}
				if err := CubeTransmute(); err != nil {
					return err
				}

				itemsInInv := ctx.Data.Inventory.ByLocation(item.LocationInventory)

				stashingRequired := false
				stashingSpecificItem := false
				stashingRareSpecificItem := false

				for _, it := range itemsInInv {
					if ctx.CharacterCfg.Inventory.InventoryLock[it.Position.Y][it.Position.X] != 1 {
						continue
					}

					if it.Name == "Key" || it.IsPotion() || it.Name == item.TomeOfTownPortal || it.Name == item.TomeOfIdentify {
						continue
					}

					if it.Name == "WirtsLeg" && it.Quality >= item.QualityMagic {
						ctx.Logger.Debug("FORCING STASH OF WIRT'S LEG AFTER CUBING", "quality", it.Quality.ToString())
						stashingRequired = true
						continue
					}

					shouldStash, _, reason, _ := shouldStashIt(it, false)
					if shouldStash {
						ctx.Logger.Debug("Stashing item after cube recipe.", "item", it.Name, "recipe", recipe.Name, "reason", reason)
						stashingRequired = true
						continue
					}

					// ───────── Magic specific reroll ─────────
					if isMagicReroll && it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) {
						ctx.Logger.Warn("KEEPING MARKED SPECIFIC ITEM AFTER REROLL — FORCING STASH")
						stashingRequired = true
						stashingSpecificItem = true
						continue
					}

					// ───────── Rare specific reroll ─────────
					if isRareReroll && it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) {
						ctx.Logger.Warn("KEEPING MARKED RARE SPECIFIC ITEM AFTER REROLL — FORCING STASH")
						stashingRequired = true
						stashingRareSpecificItem = true
						continue
					}
				}

				if stashingRequired {
					force := stashingSpecificItem || stashingRareSpecificItem
					_ = Stash(force)

					// refresh stash state
					itemsInStash = ctx.Data.Inventory.ByLocation(item.LocationStash, item.LocationSharedStash)
				}

				itemsInStash = removeUsedItems(itemsInStash, items)

				// ───────── Update magic fingerprint ─────────
				if isMagicReroll {
					for _, it := range itemsInInv {
						if it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) && it.Quality == item.QualityMagic {
							fp := SpecificFingerprint(it)
							ctx.Logger.Warn("MARKED SPECIFIC ITEM REROLLED — UPDATING FINGERPRINT", "newFP", fp)

							ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint = fp
							ctx.MarkedSpecificItemUnitID = 0

							if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
								ctx.Logger.Error("FAILED TO SAVE CharacterCfg AFTER UPDATING FINGERPRINT", "err", err)
							}
							break
						}
					}
				}

				// ───────── Update rare fingerprint ─────────
				if isRareReroll {
					for _, it := range itemsInInv {
						if it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) && it.Quality == item.QualityRare {
							fp := SpecificRareFingerprint(it)
							ctx.Logger.Warn("MARKED SPECIFIC RARE ITEM REROLLED — UPDATING FINGERPRINT", "newFP", fp)

							ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint = fp
							ctx.MarkedRareSpecificItemUnitID = 0

							if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
								ctx.Logger.Error("FAILED TO SAVE CharacterCfg AFTER UPDATING FINGERPRINT", "err", err)
							}
							break
						}
					}
				}

			} else {
				continueProcessing = false
			}
		}
	}

	ctx.Logger.Warn("Reroll Specific Magic Item enabled?", "enabled",
		slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Magic Item"))
	ctx.Logger.Warn("Marked SpecificItem UnitID", "unitID", ctx.MarkedSpecificItemUnitID)
	ctx.Logger.Warn("Current marked SpecificItem fingerprint", "fp",
		ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint)

	ctx.Logger.Warn("Reroll Specific Rare Item enabled?", "enabled",
		slices.Contains(ctx.CharacterCfg.CubeRecipes.EnabledRecipes, "Reroll Specific Rare Item"))
	ctx.Logger.Warn("Marked RareSpecificItem UnitID", "unitID", ctx.MarkedRareSpecificItemUnitID)
	ctx.Logger.Warn("Current marked RareSpecificItem fingerprint", "fp",
		ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint)

	return nil
}

func hasItemsForRecipe(ctx *context.Status, recipe CubeRecipe) ([]data.Item, bool) {

	ctx.RefreshGameData()
	items := ctx.Data.Inventory.ByLocation(item.LocationStash, item.LocationSharedStash)

	if strings.Contains(recipe.Name, "Add Sockets to") {
		return hasItemsForSocketRecipe(ctx, recipe, items)
	}

	/* // Special handling for "Reroll GrandCharms" recipe
	if recipe.Name == "Reroll GrandCharms" {
		return hasItemsForGrandCharmReroll(ctx, items)
	} */
	// Special handling for "Reroll Specific" recipe
	if recipe.Name == "Reroll Specific Magic Item" {
		return hasItemsForSpecificReroll(ctx, items)
	}

	if recipe.Name == "Reroll Specific Rare Item" {
		return hasItemsForRareSpecificReroll(ctx, items)
	}

	if recipe.Name == "MagicWirtsLegStep1" {
		return hasItemsForMagicWirtsLegReroll(ctx, items)
	}

	if recipe.Name == "MagicWirtsLegStep2" {
		return hasItemsForCraftedWirtsLeg(ctx, items)
	}

	recipeItems := make(map[string]int)
	for _, item := range recipe.Items {
		recipeItems[item]++
	}

	// --- SPECIAL CASE: Require 6+ Topaz for Flawless Topaz recipe ---
	if recipe.Name == "Flawless Topaz" {
		topazCount := 0
		for _, stashItem := range items {
			if stashItem.Name == "Topaz" {
				topazCount++
			}
		}

		// Only allow recipe if we have 6 or more
		if topazCount < 6 {
			ctx.Logger.Debug("Skipping Flawless Topaz recipe: need at least 6 Topaz, have", "count", topazCount)
			return nil, false
		}
	}

	// --- SPECIAL CASE: WAIT UNTIL BOT HAS 2x Ral Runes, 2x Jewels, 2x Perfect Amethysts TO BEGIN THE CASTER AMULET RECIPE. THIS WILL GUARANTEE BLOOD HELM TO HAVE 1x Ral, 1x Jewel AND WILL GUARANTEE CASTER BELT TO HAVE 1x Perfect Amethyst, 1x Jewel
	if recipe.Name == "Caster Amulet" && isRecipeEnabled("Caster Amulet", ctx.CharacterCfg.CubeRecipes.EnabledRecipes) && isRecipeEnabled("Blood Helm (Armet)", ctx.CharacterCfg.CubeRecipes.EnabledRecipes) && isRecipeEnabled("Caster Belt (SharkskinBelt)", ctx.CharacterCfg.CubeRecipes.EnabledRecipes) {

		usableJewelCount := 0
		ralCount := 0
		perfectAmethystCount := 0

		allItems := ctx.Data.Inventory.ByLocation(
			item.LocationInventory,
			item.LocationStash,
			item.LocationSharedStash,
		)

		for _, it := range allItems {
			switch it.Name {

			case "Jewel":
				// ✅ ONLY count jewels that do NOT match a NIP rule
				if _, res := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(it); res != nip.RuleResultFullMatch {
					usableJewelCount++
				}

			case "RalRune":
				ralCount++

			case "PerfectAmethyst":
				perfectAmethystCount++
			}
		}

		readyForCrafting :=
			usableJewelCount >= 3 &&
				ralCount >= 2 &&
				perfectAmethystCount >= 2

		if !readyForCrafting {
			missing := []string{}

			if usableJewelCount < 3 {
				missing = append(missing, fmt.Sprintf("Jewels (%d/3, NIP-safe)", usableJewelCount))
			}
			if ralCount < 2 {
				missing = append(missing, fmt.Sprintf("Ral Runes (%d/2)", ralCount))
			}
			if perfectAmethystCount < 2 {
				missing = append(missing, fmt.Sprintf("Perfect Amethysts (%d/2)", perfectAmethystCount))
			}

			ctx.Logger.Debug(
				"Deferring Caster Amulet – missing: " + strings.Join(missing, ", "),
			)
			return nil, false
		}

	}

	// --- SPECIAL CASE: Require 6+ Diamond for Flawless Diamond recipe ---
	if recipe.Name == "Flawless Diamond" {
		diamondCount := 0
		for _, stashItem := range items {
			if stashItem.Name == "Diamond" {
				diamondCount++
			}
		}

		// Only allow recipe if we have 6 or more
		if diamondCount < 6 {
			ctx.Logger.Debug("Skipping Flawless Diamond recipe: need at least 6 Diamond, have", "count", diamondCount)
			return nil, false
		}
	}

	// --- SPECIAL CASE: Require 6+ Emerald for Flawless Emerald recipe ---
	if recipe.Name == "Flawless Emerald" {
		emeraldCount := 0
		for _, stashItem := range items {
			if stashItem.Name == "Emerald" {
				emeraldCount++
			}
		}

		// Only allow recipe if we have 6 or more
		if emeraldCount < 6 {
			ctx.Logger.Debug("Skipping Flawless Emerald recipe: need at least 6 Emerald, have", "count", emeraldCount)
			return nil, false
		}
	}

	itemsForRecipe := []data.Item{}

	// Iterate over the items in our stash to see if we have the items for the recipe.
	for _, it := range items {
		if count, ok := recipeItems[string(it.Name)]; ok {

			if it.Name == "Jewel" || it.Name == "Ring" || it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) || it.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) || it.Name == "Amulet" || it.Name == "Wirt'sLeg" || it.Name == "WirtsLeg" || it.Name == "MithrilCoil" || it.Name == "MeshBelt" || it.Name == "VampirefangBelt" || it.Name == "HeavyBracers" || it.Name == "SharkskinGloves" || it.Name == "Armet" || it.Name == "SharkskinBelt" || it.Name == "VampireboneGloves" {
				if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(it); result == nip.RuleResultFullMatch {
					ctx.Logger.Debug("Skipping item that matches NIP rules for cubing recipe", "item", it.Name, "recipe", recipe.Name)

					// Skip this item for cubing
					continue
				}
			}

			itemsForRecipe = append(itemsForRecipe, it)

			// Check if we now have exactly the needed count before decrementing
			count -= 1
			if count == 0 {
				delete(recipeItems, string(it.Name))
				if len(recipeItems) == 0 {
					return itemsForRecipe, true
				}
			} else {
				recipeItems[string(it.Name)] = count
			}
		}
	}

	// We don't have all the items for the recipie.
	return nil, false
}

func hasItemsForSocketRecipe(ctx *context.Status, recipe CubeRecipe, items []data.Item) ([]data.Item, bool) {
	ctx.Logger.Debug("Processing socket recipe", "recipe", recipe.Name, "totalItems", len(items))

	recipeItems := make(map[string]int)
	for _, itemName := range recipe.Items {
		recipeItems[itemName]++
	}

	itemsForRecipe := []data.Item{}

	var targetItemTypes []string
	switch recipe.Name {
	case "Add Sockets to Weapon":
		targetItemTypes = []string{
			item.TypeWeapon, item.TypeAxe, item.TypeSword, item.TypeSpear,
			item.TypePolearm, item.TypeMace, item.TypeBow,
			item.TypeWand, item.TypeStaff, item.TypeScepter, item.TypeClub, item.TypeHammer, item.TypeKnife,
			item.TypeCrossbow, item.TypeHandtoHand, item.TypeHandtoHand2, item.TypeOrb,
			item.TypeAmazonBow, item.TypeAmazonSpear,
		}
	case "Add Sockets to Armor":
		targetItemTypes = []string{item.TypeArmor}
	case "Add Sockets to Helm":
		targetItemTypes = []string{
			item.TypeHelm,
			item.TypePrimalHelm, item.TypePelt, item.TypeCirclet,
		}
	case "Add Sockets to Shield":
		targetItemTypes = []string{
			item.TypeShield, item.TypeAuricShields, item.TypeVoodooHeads,
		}
	default:
		return nil, false
	}

	for _, itm := range items {
		itemName := string(itm.Name)

		if count, ok := recipeItems[itemName]; ok {
			itemsForRecipe = append(itemsForRecipe, itm)
			count--
			if count == 0 {
				delete(recipeItems, itemName)
			} else {
				recipeItems[itemName] = count
			}
		} else {

			specialType := ""
			switch recipe.Name {
			case "Add Sockets to Weapon":
				specialType = "NormalWeapon"
			case "Add Sockets to Armor":
				specialType = "NormalArmor"
			case "Add Sockets to Helm":
				specialType = "NormalHelm"
			case "Add Sockets to Shield":
				specialType = "NormalShield"
			}

			if count, ok := recipeItems[specialType]; ok && isSocketableItemMultiType(itm, targetItemTypes) {
				ctx.Logger.Debug("Found socketable item for recipe", "recipe", recipe.Name, "item", itm.Name, "quality", itm.Quality.ToString(), "ethereal", itm.Ethereal)
				itemsForRecipe = append(itemsForRecipe, itm)
				count--
				if count == 0 {
					delete(recipeItems, specialType)
				} else {
					recipeItems[specialType] = count
				}
			}
		}

		if len(recipeItems) == 0 {
			ctx.Logger.Debug("Socket recipe ready to execute", "recipe", recipe.Name, "itemCount", len(itemsForRecipe))
			return itemsForRecipe, true
		}
	}

	return nil, false
}

func isSocketableItemMultiType(itm data.Item, targetTypes []string) bool {

	excludedItems := []string{
		"Runic Talons",
		"War Scepter",
		"Greater Talons",
		"Caduceus",
		"Divine Scepter",
		"Cedar Staff",
		"Elder Staff",
		"Gnarled Staff",
		"Walking Stick",
	}

	for _, excluded := range excludedItems {
		if string(itm.Name) == excluded {
			return false
		}
	}

	if itm.Quality != item.QualityNormal {
		return false
	}

	if itm.HasSockets || len(itm.Sockets) > 0 {
		return false
	}

	if itm.Desc().MaxSockets == 0 {
		return false
	}

	for _, targetType := range targetTypes {
		if itm.Type().IsType(targetType) {
			return true
		}
	}

	return false
}

/* func hasItemsForGrandCharmReroll(ctx *context.Status, items []data.Item) ([]data.Item, bool) {
	var grandCharm *data.Item
	perfectGems := make([]data.Item, 0, 3)
	countAmethyst := 0
	countRuby := 0
	countSapphire := 0

	// Use only ilvl 91 or higher to roll grand charm
	// Use only ilvl 91 or higher to roll grand charm
	if ctx.CharacterCfg.CubeRecipes.RerollGrandCharms {
		for _, itm := range items {
			if itm.Name != "GrandCharm" || itm.Quality != item.QualityMagic {
				continue
			}

			fp := utils.GrandCharmFingerprint(itm)
			if fp != ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint {
				continue
			}

			// ✅ Ignore NIP; evaluate keeper rules only
			if !isKeeperGrandCharm(itm) {
				// ❌ Not a keeper → reroll
				grandCharm = &itm
				ctx.Logger.Warn(
					"CURRENTLY HAVE NON-GODLY MARKED GRAND CHARM PENDING REROLL RECIPE",
					"fp", fp,
				)
				break
			}

			// ✅ Keeper GC → stop rerolling forever
			ctx.Logger.Warn(
				"GODLY!!!! GRAND CHARM FOUND — STOPPING REROLL",
				"fp", fp,
			)
			// LOG STATS OF GODLY CHARM
			ctx.Logger.Warn(
				"GODLY GRAND CHARM RAW STATS",
				"stats", itm.Stats,
			)

			ctx.CharacterCfg.CubeRecipes.MarkedGrandCharmFingerprint = ""
			ctx.MarkedGrandCharmUnitID = 0 // added after, not sure if ok
			if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
				ctx.Logger.Error("FAILED TO SAVE CONFIG AFTER KEEPER GC", "err", err)
			}

			// Do not select for reroll; stash logic will handle it
		}
	} else {

		// Use any magic grand charm that does NOT match NIP
		if grandCharm == nil {
			for _, itm := range items {
				if itm.Name == "GrandCharm" && itm.Quality == item.QualityMagic {
					if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(itm); result != nip.RuleResultFullMatch {
						grandCharm = &itm
						break
					}
				}
			}
		}
	}

	// Collect perfect gems
	for _, itm := range items {
		switch itm.Name {
		case "PerfectAmethyst":
			countAmethyst++
		case "PerfectRuby":
			countRuby++
		case "PerfectSapphire":
			countSapphire++
		}
		if isPerfectGem(itm) && len(perfectGems) < 3 {
			if (ctx.CharacterCfg.CubeRecipes.SkipPerfectAmethysts && itm.Name == "PerfectAmethyst" && countAmethyst <= 3) ||
				(ctx.CharacterCfg.CubeRecipes.SkipPerfectRubies && itm.Name == "PerfectRuby" && countRuby <= 3) ||
				(itm.Name == "PerfectSapphire" && countSapphire <= 3) {
				continue
			}
			perfectGems = append(perfectGems, itm)
		}
	}

	if grandCharm != nil && len(perfectGems) == 3 {
		return append([]data.Item{*grandCharm}, perfectGems...), true
	}

	return nil, false
} */

func hasItemsForSpecificReroll(ctx *context.Status, items []data.Item) ([]data.Item, bool) {
	var specificitem data.Item
	perfectGems := make([]data.Item, 0, 3)
	// Count Perfect Amethyst and Perfect Ruby in inventory
	countAmethyst := 0
	countRuby := 0
	countSapphire := 0

	for _, itm := range items {

		switch itm.Name {
		case "PerfectAmethyst":
			countAmethyst++
		case "PerfectRuby":
			countRuby++
		case "PerfectSapphire":
			countSapphire++
		}
		specificfp := SpecificFingerprint(itm)

		if itm.Name == item.Name(ctx.CharacterCfg.CubeRecipes.SpecificItemToReroll) && itm.Quality == item.QualityMagic && specificfp == ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint {
			if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(itm); result != nip.RuleResultFullMatch {
				specificitem = itm
			} else {
				ctx.Logger.Warn(
					"GODLY!!!! SPECIFIC ITEM MATCHES NIP — SKIPPING REROLL",
					"item", itm.Name,
					"fp", specificfp,
					"quality", itm.Quality,
				)
				ctx.CharacterCfg.CubeRecipes.MarkedSpecificItemFingerprint = ""
				ctx.MarkedSpecificItemUnitID = 0
				if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
					ctx.Logger.Error("FAILED TO SAVE CONFIG AFTER NIP MATCH", "err", err)
				}
			}
		} else if isPerfectGem(itm) && len(perfectGems) < 3 {
			// Skip perfect amethysts and rubies when i have less than 4 (if configured) AND skip perfect sapphires when I only have 2
			if (ctx.CharacterCfg.CubeRecipes.SkipPerfectAmethysts && itm.Name == "PerfectAmethyst" && countAmethyst <= 3) ||
				(ctx.CharacterCfg.CubeRecipes.SkipPerfectRubies && itm.Name == "PerfectRuby" && countRuby <= 3) || (itm.Name == "PerfectSapphire" && countSapphire <= 3) {
				continue
			}
			perfectGems = append(perfectGems, itm)
		}

		if specificitem.Name != "" && len(perfectGems) == 3 {
			return append([]data.Item{specificitem}, perfectGems...), true
		}
	}

	return nil, false
}

func hasItemsForRareSpecificReroll(ctx *context.Status, items []data.Item) ([]data.Item, bool) {
	var specificitem data.Item
	perfectSkulls := make([]data.Item, 0, 6)

	for _, itm := range items {

		specificfp := SpecificRareFingerprint(itm)

		// ───────── Identify the marked rare item ─────────
		if itm.Name == item.Name(ctx.CharacterCfg.CubeRecipes.RareSpecificItemToReroll) &&
			itm.Quality == item.QualityRare &&
			specificfp == ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint {

			if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(itm); result != nip.RuleResultFullMatch {
				specificitem = itm
			} else {
				ctx.Logger.Warn(
					"GODLY!!!! RARE SPECIFIC ITEM MATCHES NIP — SKIPPING REROLL",
					"item", itm.Name,
					"fp", specificfp,
					"quality", itm.Quality,
				)

				ctx.CharacterCfg.CubeRecipes.MarkedRareSpecificItemFingerprint = ""
				ctx.MarkedRareSpecificItemUnitID = 0

				if err := config.SaveSupervisorConfig(ctx.Name, ctx.CharacterCfg); err != nil {
					ctx.Logger.Error("FAILED TO SAVE CONFIG AFTER NIP MATCH", "err", err)
				}
			}
			continue
		}

		// ───────── Collect Perfect Skulls only ─────────
		if itm.Name == "PerfectSkull" && len(perfectSkulls) < 6 {
			perfectSkulls = append(perfectSkulls, itm)
		}

		// ───────── Recipe satisfied ─────────
		if specificitem.Name != "" && len(perfectSkulls) == 6 {
			return append([]data.Item{specificitem}, perfectSkulls...), true
		}
	}

	return nil, false
}

func hasItemsForMagicWirtsLegReroll(ctx *context.Status, items []data.Item) ([]data.Item, bool) {
	var magicwirtsleg data.Item
	standardGems := make([]data.Item, 0, 3)

	for _, itm := range items {

		if itm.Name == "WirtsLeg" && itm.HasSockets {
			if itm.Quality <= item.QualitySuperior {
				magicwirtsleg = itm
			}
		} else if isStandardGem(itm) && len(standardGems) < 3 {

			standardGems = append(standardGems, itm)
		}

		if magicwirtsleg.Name != "" && len(standardGems) == 3 {
			return append([]data.Item{magicwirtsleg}, standardGems...), true
		}
	}

	return nil, false
}

func hasItemsForCraftedWirtsLeg(ctx *context.Status, items []data.Item) ([]data.Item, bool) {
	var leg data.Item
	var runeItem, gem, jewel data.Item

	for _, itm := range items {
		switch itm.Name {
		case "WirtsLeg":
			// Only use Wirt's Leg if it passes rules and is Magic quality
			if _, result := ctx.CharacterCfg.Runtime.Rules.EvaluateAll(itm); result != nip.RuleResultFullMatch && itm.Quality == item.QualityMagic {
				leg = itm
			}
		case "TirRune":
			runeItem = itm
		case "PerfectSapphire":
			gem = itm
		case "Jewel":
			jewel = itm
		}
	}

	// Only return if we have all 4 items
	if leg.Name != "" && runeItem.Name != "" && gem.Name != "" && jewel.Name != "" {
		return []data.Item{leg, runeItem, gem, jewel}, true
	}

	ctx.Logger.Debug("SKIPPING RECIPE... missing ingredients for MagicWirtsLegStep2")
	return nil, false
}

func isPerfectGem(item data.Item) bool {
	perfectGems := []string{"PerfectDiamond", "PerfectEmerald", "PerfectRuby", "PerfectTopaz", "PerfectAmethyst", "PerfectSapphire"} //took out PerfectSkulls (keep for rolling) and Perfect Sapphires (for wirts leg)
	for _, gemName := range perfectGems {
		if string(item.Name) == gemName {
			return true
		}
	}
	return false
}

func isStandardGem(item data.Item) bool {
	perfectGems := []string{"Diamond", "Emerald", "Topaz"} //USE ONLY DIAMONDS, EMERALDS AND TOPAZ FOR WIRTS LEG STEP 1
	for _, gemName := range perfectGems {
		if string(item.Name) == gemName {
			return true
		}
	}
	return false
}

func removeUsedItems(stash []data.Item, usedItems []data.Item) []data.Item {
	remainingItems := make([]data.Item, 0)
	usedItemMap := make(map[string]int)

	// Populate a map with the count of used items
	for _, item := range usedItems {
		usedItemMap[string(item.Name)] += 1 // Assuming 'ID' uniquely identifies items in 'usedItems'
	}

	// Filter the stash by excluding used items based on the count in the map
	for _, item := range stash {
		if count, exists := usedItemMap[string(item.Name)]; exists && count > 0 {
			usedItemMap[string(item.Name)] -= 1
		} else {
			remainingItems = append(remainingItems, item)
		}
	}

	return remainingItems
}

func getPurchasedItem(ctx *context.Status, purchaseItems []string) data.Item {
	itemsInInv := ctx.Data.Inventory.ByLocation(item.LocationInventory)
	for _, citem := range itemsInInv {
		for _, pi := range purchaseItems {
			if string(citem.Name) == pi && citem.Quality == item.QualityMagic {
				return citem
			}
		}
	}
	return data.Item{}
}

func isKeeperGrandCharm(itm data.Item) bool {

	skillTab, _ := itm.FindStat(stat.AddSkillTab, 0)
	maxLife, _ := itm.FindStat(stat.MaxLife, 0)
	fhr, _ := itm.FindStat(stat.FasterHitRecovery, 0)
	maxDmg, _ := itm.FindStat(stat.MaxDamage, 0)
	ar, _ := itm.FindStat(stat.AttackRating, 0)
	//frw, _ := itm.FindStat(stat.FasterRunWalk, 0)
	fireRes, _ := itm.FindStat(stat.FireResist, 0)
	coldRes, _ := itm.FindStat(stat.ColdResist, 0)
	lightRes, _ := itm.FindStat(stat.LightningResist, 0)
	poisonRes, _ := itm.FindStat(stat.PoisonResist, 0)
	//tests
	def, _ := itm.FindStat(stat.Defense, 0)
	mf, _ := itm.FindStat(stat.MagicFind, 0)
	mana, _ := itm.FindStat(stat.MaxMana, 0)
	//minDmg, _ := itm.FindStat(stat.MinDamage, 0)
	//lightDmg, _ := itm.FindStat(stat.LightningMaxDamage, 0)
	//coldDmg, _ := itm.FindStat(stat.ColdMaxDamage, 0)

	// 🎯 Skiller + fhr
	if fhr.Value >= 12 && fireRes.Value+coldRes.Value+poisonRes.Value+lightRes.Value == 60 {
		return true
	}
	// 🎯 Skiller + fhr
	if fhr.Value >= 12 && skillTab.Value == 1 {
		return true
	}
	// 🎯 def / hp
	if mf.Value >= 12 && maxLife.Value >= 45 {
		return true
	}
	// 🎯 def / hp
	if def.Value >= 100 && maxLife.Value >= 41 {
		return true
	}
	// 🎯 mana / hp
	if mana.Value >= 59 && maxLife.Value >= 45 {
		return true
	}

	// 🎯 psnres / hp
	if poisonRes.Value >= 30 && maxLife.Value >= 45 {
		return true
	}
	// 🎯 fireres / hp
	if fireRes.Value >= 30 && maxLife.Value >= 45 {
		return true
	}
	// 🎯 lightres / hp
	if lightRes.Value >= 30 && maxLife.Value >= 45 {
		return true
	}
	// 🎯 coldres / hp
	if coldRes.Value >= 30 && maxLife.Value >= 45 {
		return true
	}

	// 🎯 Skiller + Life
	if skillTab.Value == 1 && maxLife.Value >= 41 {
		return true
	}
	// 🎯 Melee GC example
	if maxDmg.Value >= 9 && ar.Value >= 49 && maxLife.Value >= 30 {
		return true
	}
	// 🎯 Melee GC example
	if ar.Value >= 130 && maxLife.Value >= 41 {
		return true
	}
	// 🎯 allres / life
	if maxLife.Value >= 41 && fireRes.Value+coldRes.Value+poisonRes.Value+lightRes.Value == 60 {
		return true
	}
	// 🎯 SPECIFIC SKILLTAB (defined below) + fhr
	if hasAllowedSkillTab(itm) && fhr.Value >= 12 {
		return true
	}

	return false
}

var allowedSkillTabs = map[int]bool{
	// Amazon
	0: true, // Bow & Crossbow
	1: true, // Javelin & Spear
	2: true, // Passive & Magic

	// Sorceress
	3: true, // Fire Skills
	4: true, // Lightning Skills
	5: true, // Cold Skills

	// Necromancer
	6: true, // Summoning
	7: true, // Poison & Bone
	//8: true, // Curses

	// Paladin
	9:  true, // Combat
	10: true, // Offensive Auras
	11: true, // Defensive Auras

	// Barbarian
	//12: true, // Combat Skills
	//13: true, // Masteries
	14: true, // Warcries

	// Druid
	15: true, // Summoning
	16: true, // Shape Shifting
	17: true, // Elemental

	// Assassin
	18: true, // Martial Arts
	//19: true, // Shadow Disciplines
	20: true, // Traps

	// Misc / Amazon extra
	//21: true, // Misc / unused?
}

func hasAllowedSkillTab(itm data.Item) bool {
	for _, s := range itm.Stats {
		if s.ID != stat.AddSkillTab || s.Value != 1 {
			continue
		}

		if allowedSkillTabs[s.Layer] {
			return true
		}
	}
	return false
}
