package service

import (
	"fmt"
	log "github.com/shyunku-libraries/go-logger"
	"os"
	"sort"
)

const (
	TierUnranked    = "UNRANKED"
	TierIron        = "IRON"
	TierBronze      = "BRONZE"
	TierSilver      = "SILVER"
	TierGold        = "GOLD"
	TierPlatinum    = "PLATINUM"
	TierEmerald     = "EMERALD"
	TierDiamond     = "DIAMOND"
	TierMaster      = "MASTER"
	TierGrandmaster = "GRANDMASTER"
	TierChallenger  = "CHALLENGER"

	TierHighUnderBound = TierMaster

	TierHighGrandmasterUnderBound = 650
	TierHighChallengerUnderBound  = 900

	RankI   = "I"
	RankII  = "II"
	RankIII = "III"
	RankIV  = "IV"

	RankUnitRatingPoint = 100
)

type Tier string
type Rank string
type LeaguePoint int64

var (
	TierRankMap = map[Tier][]Rank{
		TierUnranked:    {},
		TierIron:        {RankI, RankII, RankIII, RankIV},
		TierBronze:      {RankI, RankII, RankIII, RankIV},
		TierSilver:      {RankI, RankII, RankIII, RankIV},
		TierGold:        {RankI, RankII, RankIII, RankIV},
		TierPlatinum:    {RankI, RankII, RankIII, RankIV},
		TierEmerald:     {RankI, RankII, RankIII, RankIV},
		TierDiamond:     {RankI, RankII, RankIII, RankIV},
		TierMaster:      {RankI},
		TierGrandmaster: {RankI},
		TierChallenger:  {RankI},
	}
	Ranks                  = []Rank{RankI, RankII, RankIII, RankIV}
	TierBaseRatingPointMap = func() map[Tier]int64 {
		m := make(map[Tier]int64)
		ratingPoint := int64(0)

		tierKeys := make([]Tier, 0)
		for tier, _ := range TierRankMap {
			if tier == TierUnranked {
				continue
			}
			tierKeys = append(tierKeys, tier)
		}

		// sort tier keys
		sort.SliceStable(tierKeys, func(i, j int) bool {
			tierLevelI, err := GetTierLevel(tierKeys[i])
			if err != nil {
				log.Fatal(err)
				os.Exit(-1)
			}
			tierLevelJ, err := GetTierLevel(tierKeys[j])
			if err != nil {
				log.Fatal(err)
				os.Exit(-1)
			}
			return tierLevelI < tierLevelJ
		})

		for _, tier := range tierKeys {
			m[tier] = ratingPoint
			ranks := TierRankMap[tier]
			for _, _ = range ranks {
				ratingPoint += RankUnitRatingPoint
			}
		}
		//log.Debug(m)
		return m
	}()
	//test = func() error {
	//	for tier, _ := range TierRankMap {
	//		for _, rank := range TierRankMap[tier] {
	//			ratingPoint, err := CalculateRatingPoint(string(tier), string(rank), 0)
	//			if err != nil {
	//				log.Fatal(err)
	//				os.Exit(-1)
	//			}
	//			log.Debugf("tier: %s, rank: %s, ratingPoint: %d", tier, rank, ratingPoint)
	//		}
	//	}
	//	return nil
	//}()
	//test2 = func() error {
	//	i := 0
	//	for {
	//		if i > 5000 {
	//			break
	//		}
	//		i += 30
	//		tier, rank, lp, err := CalculateTierRank(float64(int64(i)))
	//		if err != nil {
	//			log.Fatal(err)
	//			os.Exit(-1)
	//		}
	//		log.Debugf("ratingPoint: %d, tier: %s, rank: %s, lp: %d", i, tier, rank, lp)
	//	}
	//	return nil
	//}()
)

func IsValidTierRank(tier, rank string) bool {
	ranks, ok := TierRankMap[Tier(tier)]
	if !ok {
		return false
	}

	if len(ranks) == 0 {
		return rank == ""
	}

	for _, r := range ranks {
		if r == Rank(rank) {
			return true
		}
	}

	return false
}

func CalculateRatingPoint(tier, rank string, lp int) (int64, error) {
	var ratingPoint int64 = 0

	tierLevel, err := GetTierLevel(Tier(tier))
	if err != nil {
		return 0, err
	}

	highTierUnderBoundLevel, err := GetTierLevel(TierHighUnderBound)
	if err != nil {
		return 0, err
	}

	if tierLevel < highTierUnderBoundLevel {
		base, ok := TierBaseRatingPointMap[Tier(tier)]
		if !ok {
			return 0, fmt.Errorf("invalid tier: %s", tier)
		}
		rankLevel, err := GetRankLevel(Tier(tier), Rank(rank))
		if err != nil {
			return 0, err
		}

		ratingPoint = base + int64(rankLevel*RankUnitRatingPoint) + int64(lp)
	} else {
		highTierUnderBoundBaseRatingPoint, ok := TierBaseRatingPointMap[TierHighUnderBound]
		if !ok {
			return 0, fmt.Errorf("invalid tier: %s", tier)
		}

		ratingPoint = highTierUnderBoundBaseRatingPoint + int64(lp)
	}

	return ratingPoint, nil
}

func CalculateTierRank(ratingPointRaw float64) (Tier, Rank, LeaguePoint, error) {
	var tier Tier = TierIron
	var rank Rank
	var lp LeaguePoint

	ratingPoint := int64(ratingPointRaw)

	if ratingPoint < 0 {
		return "", "", 0, fmt.Errorf("invalid rating point: %d", ratingPoint)
	}

	highTierUnderBoundBaseRatingPoint, ok := TierBaseRatingPointMap[TierHighUnderBound]
	if !ok {
		return "", "", 0, fmt.Errorf("invalid tier: %s", tier)
	}

	if ratingPoint < highTierUnderBoundBaseRatingPoint {
		var highestTierBase int64 = 0
		for tierKey, tierBase := range TierBaseRatingPointMap {
			if ratingPoint >= TierBaseRatingPointMap[tierKey] {
				if tierBase > highestTierBase {
					highestTierBase = tierBase
					tier = tierKey
				}
			}
		}

		remainingLp := ratingPoint - highestTierBase
		rankLevel := int(remainingLp / RankUnitRatingPoint)
		ranks, ok := TierRankMap[tier]
		if !ok {
			return "", "", 0, fmt.Errorf("invalid tier: %s", tier)
		}
		rank = ranks[len(ranks)-rankLevel-1]
		lp = LeaguePoint(remainingLp % RankUnitRatingPoint)
		//log.Debugf(
		//	"tierBase: %s, highestTierBase: %d, remainingLp: %d, rankLevel: %d, rank: %s",
		//	tier, highestTierBase, remainingLp, rankLevel, rank,
		//)
	} else {
		// high tier
		remainingLp := ratingPoint - highTierUnderBoundBaseRatingPoint
		if remainingLp < TierHighGrandmasterUnderBound {
			tier = TierMaster
			rank = RankI
		} else if remainingLp < TierHighChallengerUnderBound {
			tier = TierGrandmaster
			rank = RankI
		} else {
			tier = TierChallenger
			rank = RankI
		}
		lp = LeaguePoint(remainingLp)
	}

	return tier, rank, lp, nil
}

func GetTierLevel(tier Tier) (int, error) {
	switch tier {
	case TierUnranked:
		return 0, nil
	case TierIron:
		return 1, nil
	case TierBronze:
		return 2, nil
	case TierSilver:
		return 3, nil
	case TierGold:
		return 4, nil
	case TierPlatinum:
		return 5, nil
	case TierEmerald:
		return 6, nil
	case TierDiamond:
		return 7, nil
	case TierMaster:
		return 8, nil
	case TierGrandmaster:
		return 9, nil
	case TierChallenger:
		return 10, nil
	default:
		return 0, fmt.Errorf("invalid tier: %s", tier)
	}
}

func GetRankLevel(tier Tier, rank Rank) (int, error) {
	ranks, ok := TierRankMap[tier]
	if !ok {
		return 0, fmt.Errorf("invalid tier: %s", tier)
	}
	var rankReverseLevel int

	switch rank {
	case RankI:
		rankReverseLevel = 1
	case RankII:
		rankReverseLevel = 2
	case RankIII:
		rankReverseLevel = 3
	case RankIV:
		rankReverseLevel = 4
	default:
		return 0, fmt.Errorf("invalid rank: %s", rank)
	}

	return len(ranks) - rankReverseLevel, nil
}
