package types

import "time"

const (
	RankTypeSolo = "RANKED_SOLO_5x5"
	RankTypeFlex = "RANKED_FLEX_SR"

	PerkStyleDescriptionTypePrimary = "primaryStyle"
	PerkStyleDescriptionTypeSub     = "subStyle"

	LoadInitialMatchCount    = 20
	LoadMoreMatchCount       = 10
	LoadInitialMatchCountDev = 10
	LoadMoreMatchCountDev    = 5

	DataExplorerLoopPeriod       = 24 * time.Hour
	DataExplorerLoopPeriodDev    = 24 * time.Hour
	DataExplorerLoadMatchesCount = 3

	SummonerRankingRevisionPeriod = 7 * 24 * time.Hour

	PositionTop     = "TOP"
	PositionJungle  = "JUNGLE"
	PositionMid     = "MID"
	PositionAdc     = "ADC"
	PositionSupport = "SUPPORT"

	TeamPositionTop     = "TOP"
	TeamPositionJungle  = "JUNGLE"
	TeamPositionMid     = "MIDDLE"
	TeamPositionAdc     = "BOTTOM"
	TeamPositionSupport = "UTILITY"

	WeightLineFairness     = 0.36
	WeightTierFairness     = 0.24
	WeightLineSatisfaction = 1 - WeightLineFairness - WeightTierFairness

	WeightTopInfluence     = 0.14
	WeightJungleInfluence  = 0.23
	WeightMidInfluence     = 0.25
	WeightAdcInfluence     = 0.21
	WeightSupportInfluence = 1 - WeightTopInfluence - WeightJungleInfluence - WeightMidInfluence - WeightAdcInfluence

	QueueTypeAll         = 0   // 전체
	QueueTypeNormalDraft = 400 // 일반 (드래프트)
	QueueTypeSolo        = 420 // 솔랭
	QueueTypeNormal      = 430 // 일반
	QueueTypeFlex        = 440 // 자유 5:5 랭크
	QueueTypeAram        = 450 // 칼바람
	QueueTypeClash       = 700 // 클래시
	QueueTypeUrf         = 900 // 우르프
	QueueTypePoro        = 920 // 포로왕?

	MapTypeSummonersRift = 11
	MapTypeHowlingAbyss  = 12

	MatchDecoTypeFirstBloodKill     = "FIRST_BLOOD"
	MatchDecoTypeHighestDamage      = "HIGHEST_DAMAGE"
	MatchDecoTypeHighestDamageTaken = "HIGHEST_DAMAGE_TAKEN"
	MatchDecoTypeMostKill           = "MOST_KILL"
	MatchDecoTypeMostAssist         = "MOST_ASSIST"
	MatchDecoTypeMostMinionKill     = "MOST_MINION_KILL"
	MatchDecoTypeHighestKda         = "HIGHEST_KDA"
	MatchDecoTypeMostGold           = "MOST_GOLD"
	MatchDecoTypeMostVisionScore    = "MOST_VISION_SCORE"
	MatchDecoTypeMostWardPlaced     = "MOST_WARD_PLACED"
	MatchDecoTypeMostWardKilled     = "MOST_WARD_KILLED"
	MatchDecoTypeHighestVisionScore = "HIGHEST_VISION_SCORE"
)

const (
	ChampionTypeAttack  = "attack"
	ChampionTypeDefense = "defense"
	ChampionTypeMagic   = "magic"
)

const (
	ItemTagAbilityHaste      = "AbilityHaste"
	ItemTagActive            = "Active"
	ItemTagArmor             = "Armor"
	ItemTagArmorPenetration  = "ArmorPenetration"
	ItemTagAttackSpeed       = "AttackSpeed"
	ItemTagAura              = "Aura"
	ItemTagBoots             = "Boots"
	ItemTagConsumable        = "Consumable"
	ItemTagCooldownReduction = "CooldownReduction"
	ItemTagCriticalStrike    = "CriticalStrike"
	ItemTagDamage            = "Damage"
	ItemTagGoldPer           = "GoldPer"
	ItemTagHealth            = "Health"
	ItemTagHealthRegen       = "HealthRegen"
	ItemTagJungle            = "Jungle"
	ItemTagLane              = "Lane"
	ItemTagLifeSteal         = "LifeSteal"
	ItemTagMagicPenetration  = "MagicPenetration"
	ItemTagMagicResist       = "MagicResist"
	ItemTagMana              = "Mana"
	ItemTagManaRegen         = "ManaRegen"
	ItemTagNonbootsMovement  = "NonbootsMovement"
	ItemTagOnHit             = "OnHit"
	ItemTagSlow              = "Slow"
	ItemTagSpellBlock        = "SpellBlock"
	ItemTagSpellDamage       = "SpellDamage"
	ItemTagSpellVamp         = "SpellVamp"
	ItemTagStealth           = "Stealth"
	ItemTagTenacity          = "Tenacity"
	ItemTagTrinket           = "Trinket"
	ItemTagVision            = "Vision"

	ItemCategoryTanker           = "Armor" // 방어력/마방
	ItemCategoryArmorPenetration = "ArmorPenetration"
	ItemCategoryAttackSpeed      = "AttackSpeed"
	ItemCategoryCriticalStrike   = "CriticalStrike"
	ItemCategoryAD               = "AD"
	ItemCategoryAP               = "AP"
	ItemCategoryGoldSupply       = "GoldSupply"
	ItemCategoryHealth           = "Health"
	ItemCategoryHealthRegen      = "HealthRegen"
	ItemCategoryLifeSteal        = "LifeSteal" // AD/AP 피흡
	ItemCategoryMagicPenetration = "MagicPenetration"
	ItemCategoryMana             = "Mana"
	ItemCategoryManaRegen        = "ManaRegen"
	ItemCategoryMoveSpeed        = "MoveSpeed"
	ItemCategoryOnHit            = "OnHit"
	ItemCategorySlow             = "Slow"
	ItemCategorySpellBlock       = "SpellBlock"
	ItemCategoryStealth          = "Stealth" // deprecated
	ItemCategoryTenacity         = "Tenacity"
)

var (
	itemCategories = map[string]string{
		ItemTagAbilityHaste:      "",
		ItemTagActive:            "",
		ItemTagArmor:             ItemCategoryTanker,
		ItemTagArmorPenetration:  ItemCategoryArmorPenetration,
		ItemTagAttackSpeed:       ItemCategoryAttackSpeed,
		ItemTagAura:              "",
		ItemTagBoots:             "",
		ItemTagConsumable:        "",
		ItemTagCooldownReduction: "",
		ItemTagCriticalStrike:    ItemCategoryCriticalStrike,
		ItemTagDamage:            "",
		ItemTagGoldPer:           ItemCategoryGoldSupply,
		ItemTagHealth:            ItemCategoryHealth,
		ItemTagHealthRegen:       ItemCategoryHealthRegen,
		ItemTagJungle:            "",
		ItemTagLane:              "",
		ItemTagLifeSteal:         ItemCategoryLifeSteal,
		ItemTagMagicPenetration:  ItemCategoryMagicPenetration,
		ItemTagMagicResist:       ItemCategoryTanker,
		ItemTagMana:              ItemCategoryMana,
		ItemTagManaRegen:         ItemCategoryManaRegen,
		ItemTagNonbootsMovement:  ItemCategoryMoveSpeed,
		ItemTagOnHit:             ItemCategoryOnHit,
		ItemTagSlow:              ItemCategorySlow,
		ItemTagSpellBlock:        ItemCategorySpellBlock,
		ItemTagSpellDamage:       "",
		ItemTagSpellVamp:         ItemCategoryLifeSteal,
		ItemTagStealth:           ItemCategoryStealth,
		ItemTagTenacity:          ItemCategoryTenacity,
		ItemTagTrinket:           "",
		ItemTagVision:            "",
	}
	GetItemCategories = func(tag string) *string {
		if category, ok := itemCategories[tag]; ok {
			if category != "" {
				return &category
			}
		}
		return nil
	}
)

const (
	PerkSlotTypeKeystone = "kKeyStone"
	PerkSlotTypeStatMod  = "kStatMod"
)
