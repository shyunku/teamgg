package statistics

import (
	"fmt"
	uuid2 "github.com/google/uuid"
	"team.gg-server/service"
)

type MetaPick struct {
	Id          string
	Summoner1Id int
	Summoner2Id int

	PrimaryStyleId int
	PrimaryPerk0   int
	PrimaryPerk1   int
	PrimaryPerk2   int
	PrimaryPerk3   int

	SubStyleId int
	SubPerk0   int
	SubPerk1   int

	StatPerkDefenseId int
	StatPerkFlexId    int
	StatPerkOffenseId int

	Item0    *int
	Item1    *int
	Item2    *int
	Item3    *int
	Item4    *int
	Item5    *int
	Wins     int
	Total    int
	WinRate  float64
	PickRate float64
	MetaRank int
	MajorTag string
	MinorTag *string

	StartItems []int
	BasicItems []int
	SubItems   []int
}

func (m *MetaPick) toRealMeta() (*ChampionDetailStatisticsMeta, error) {
	items := []*int{
		m.Item0,
		m.Item1,
		m.Item2,
		m.Item3,
		m.Item4,
		m.Item5,
	}
	validItems := make([]int, 0)
	for _, itemId := range items {
		if itemId != nil {
			validItems = append(validItems, *itemId)
		}
	}

	mainSlots, subSlots, statSlots, err := getSlotsFromStyle(m.PrimaryStyleId, m.SubStyleId)
	if err != nil {
		return nil, err
	}

	primaryPerkStyle, ok := service.PerkStyles[m.PrimaryStyleId]
	if !ok {
		return nil, fmt.Errorf("primary perk style not found: %d", m.PrimaryStyleId)
	}
	subPerkStyle, ok := service.PerkStyles[m.SubStyleId]
	if !ok {
		return nil, fmt.Errorf("sub perk style not found: %d", m.SubStyleId)
	}

	uuid := uuid2.New()
	return &ChampionDetailStatisticsMeta{
		MetaKey:     fmt.Sprintf("meta-%s-%s", m.MajorTag, uuid.String()),
		MajorTag:    m.MajorTag,
		MinorTag:    m.MinorTag,
		Summoner1Id: m.Summoner1Id,
		Summoner2Id: m.Summoner2Id,
		MajorPerkGroup: PerkGroup{
			PerkStyleName: primaryPerkStyle.Name,
			PerkStyleId:   m.PrimaryStyleId,
			SubPerks:      []int{m.PrimaryPerk0, m.PrimaryPerk1, m.PrimaryPerk2, m.PrimaryPerk3},
		},
		MinorPerkGroup: PerkGroup{
			PerkStyleName: subPerkStyle.Name,
			PerkStyleId:   m.SubStyleId,
			SubPerks:      []int{m.SubPerk0, m.SubPerk1},
		},
		PerkExtra: PerkExtra{
			StatDefenseId: m.StatPerkDefenseId,
			StatFlexId:    m.StatPerkFlexId,
			StatOffenseId: m.StatPerkOffenseId,
		},
		MainSlots:     mainSlots,
		SubSlots:      subSlots,
		StatSlots:     statSlots,
		StartItemTree: m.StartItems,
		BasicItemTree: m.BasicItems,
		ItemTree:      validItems,
		SubItemTree:   m.SubItems,
		Count:         m.Total,
		Win:           m.Wins,
		PickRate:      m.PickRate,
		WinRate:       m.WinRate,
	}, nil
}

type MetaGroup []MetaPick

func (mg *MetaGroup) getTotalWinRate() float64 {
	totalWins := 0
	totalTotal := 0
	for _, metaPick := range *mg {
		totalWins += metaPick.Wins
		totalTotal += metaPick.Total
	}
	return float64(totalWins) / float64(totalTotal)
}

func (mg *MetaGroup) getTotalPickCount() int {
	totalPickCount := 0
	for _, metaPick := range *mg {
		totalPickCount += metaPick.Total
	}
	return totalPickCount
}
