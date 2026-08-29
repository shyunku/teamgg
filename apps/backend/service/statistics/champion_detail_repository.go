package statistics

import (
	"context"
	"fmt"
	uuid2 "github.com/google/uuid"
	log "github.com/shyunku-libraries/go-logger"
	"sort"
	"strconv"
	"sync"
	"team.gg-server/libs/db"
	"team.gg-server/models/mixed"
	"team.gg-server/models/mixed/statistics_models"
	"team.gg-server/service"
	"team.gg-server/types"
	"team.gg-server/util"
	"time"
)

/* ----------------------- Champion Detail statistics_models ----------------------- */

type ChampionPositionStatistics struct {
	PickCount int `json:"pickCount"`
	WinCount  int `json:"winCount"`
}

type PerkSlot struct {
	Type      string `json:"type"`
	SlotLabel string `json:"slotLabel"`
	Perks     []int  `json:"perks"`
}

type PerkGroup struct {
	PerkStyleName string `json:"perkStyleName"`
	PerkStyleId   int    `json:"perkStyleId"`
	SubPerks      []int  `json:"subPerks"`
}

type PerkExtra struct {
	StatDefenseId int `json:"statDefenseId"`
	StatFlexId    int `json:"statFlexId"`
	StatOffenseId int `json:"statOffenseId"`
}

type ChampionDetailStatisticsMeta struct {
	MetaKey  string  `json:"metaKey"`
	MajorTag string  `json:"majorTag"`
	MinorTag *string `json:"minorTag"`

	Summoner1Id int `json:"summoner1Id"`
	Summoner2Id int `json:"summoner2Id"`

	MajorPerkGroup PerkGroup `json:"majorPerkGroup"` // 메인 룬 추천
	MinorPerkGroup PerkGroup `json:"minorPerkGroup"` // Sub 룬 추천
	PerkExtra      PerkExtra `json:"perkExtra"`      // 메인 룬 스탯 추천

	MainSlots []PerkSlot `json:"mainSlots"` // 메인 룬 Placeholders
	SubSlots  []PerkSlot `json:"subSlots"`  // Sub 룬 Placeholders
	StatSlots []PerkSlot `json:"statSlots"` // 메인 룬 스탯 Placeholders

	StartItemTree []int `json:"startItemTree"` // 시작 아이템 추천
	BasicItemTree []int `json:"basicItemTree"` // 기본 아이템 추천
	ItemTree      []int `json:"itemTree"`      // 메인 아이템 추천
	SubItemTree   []int `json:"subItemTree"`   // 부가 아이템 추천

	Count int `json:"count"` // 메타 픽 수
	Win   int `json:"win"`   // 해당 메타 승리 수

	WinRate  float64 `json:"winRate"`
	PickRate float64 `json:"pickRate"`
}

type ChampionCounterStatistics struct {
	CounterChampionId   int    `json:"counterChampionId"`
	CounterChampionName string `json:"counterChampionName"`

	AvgKills   float64 `json:"avgKills"`
	AvgDeaths  float64 `json:"avgDeaths"`
	AvgAssists float64 `json:"avgAssists"`

	CounterAvgKills   *float64 `json:"counterAvgKills"`
	CounterAvgDeaths  *float64 `json:"counterAvgDeaths"`
	CounterAvgAssists *float64 `json:"counterAvgAssists"`

	Summoner1Id int `json:"summoner1Id"`
	Summoner2Id int `json:"summoner2Id"`

	MajorPerkGroup *PerkGroup `json:"majorPerkGroup"` // 메인 룬 추천
	MinorPerkGroup *PerkGroup `json:"minorPerkGroup"` // Sub 룬 추천
	PerkExtra      *PerkExtra `json:"perkExtra"`      // 메인 룬 스탯 추천

	MainSlots []PerkSlot `json:"mainSlots"` // 메인 룬 Placeholders
	SubSlots  []PerkSlot `json:"subSlots"`  // Sub 룬 Placeholders
	StatSlots []PerkSlot `json:"statSlots"` // 메인 룬 스탯 Placeholders

	StartItemTree []int `json:"startItemTree"` // 시작 아이템 추천
	BasicItemTree []int `json:"basicItemTree"` // 기본 아이템 추천
	ItemTree      []int `json:"itemTree"`      // 메인 아이템 추천
	SubItemTree   []int `json:"subItemTree"`   // 부가 아이템 추천

	Count int `json:"count"` // 메타 픽 수
	Win   int `json:"win"`   // 해당 메타 승리 수

	WinRate         float64  `json:"winRate"`
	ExpectedWinRate *float64 `json:"expectedWinRate"` // 해당 룬/아이템 조합의 기대 승률

	IsValid bool `json:"isValid"`
}

type ChampionDetailStatisticsMetaTree struct {
	MajorMetaPicks []ChampionDetailStatisticsMeta    `json:"majorMetaPicks"`
	MinorMetaPicks []ChampionDetailStatisticsMeta    `json:"minorMetaPick"`
	MetaPicks      []ChampionDetailStatisticsMeta    `json:"metaPicks"`
	PickCount      int                               `json:"pickCount"`
	WinCount       int                               `json:"winCount"`
	CounterMap     map[int]ChampionCounterStatistics `json:"counterMap"`
}

type ChampionDetailStatisticsPositionMetaTree struct {
	Top     *ChampionDetailStatisticsMetaTree `json:"top"`
	Jungle  *ChampionDetailStatisticsMetaTree `json:"jungle"`
	Mid     *ChampionDetailStatisticsMetaTree `json:"mid"`
	Adc     *ChampionDetailStatisticsMetaTree `json:"adc"`
	Support *ChampionDetailStatisticsMetaTree `json:"support"`
}

type ChampionDetailStatisticsExtraStats struct {
	AvgMinionsKilled float64 `json:"avgMinionsKilled"`
	AvgKills         float64 `json:"avgKills"`
	AvgDeaths        float64 `json:"avgDeaths"`
	AvgAssists       float64 `json:"avgAssists"`
	AvgGoldEarned    float64 `json:"avgGoldEarned"`
}

type ChampionDetailStatisticsNormalizedStats struct {
	AvgTotalHealPerSec           float64 `json:"avgTotalHealPerSec"`
	AvgVisionScorePerSec         float64 `json:"avgVisionScorePerSec"`
	AvgTotalDamageTakenPerSec    float64 `json:"avgTotalDamageTakenPerSec"`
	AvgTotalTimeCCDealtPerSec    float64 `json:"avgTotalTimeCCDealtPerSec"`
	AvgDamageSelfMitigatedPerSec float64 `json:"avgDamageSelfMitigatedPerSec"`
}

type ChampionDetailStatisticsItem struct {
	ChampionId    int    `json:"championId"`
	ChampionName  string `json:"championName"`
	ChampionTitle string `json:"championTitle"`
	ChampionStory string `json:"championStory"`

	Win         int     `json:"win"`
	Total       int     `json:"total"`
	AvgPickRate float64 `json:"avgPickRate"`
	AvgBanRate  float64 `json:"avgBanRate"`
	AvgWinRate  float64 `json:"avgWinRate"`

	ExtraStats      ChampionDetailStatisticsExtraStats       `json:"extraStats"`
	NormalizedStats ChampionDetailStatisticsNormalizedStats  `json:"normalizedStats"`
	MetaTree        ChampionDetailStatisticsPositionMetaTree `json:"metaTree"`
}

type ChampionDetailStatistics struct {
	UpdatedAt time.Time                            `json:"updatedAt"`
	Patches   []string                             `json:"patches"`
	Data      map[int]ChampionDetailStatisticsItem `json:"data"`
}

// MetaStatistics contains only the fields required by the meta list page.
// Full rune, item and counter data stays in ChampionDetailStatistics for the
// champion detail endpoints.
type MetaStatistics struct {
	UpdatedAt time.Time            `json:"updatedAt"`
	Patches   []string             `json:"patches"`
	Data      []MetaStatisticsItem `json:"data"`
}

type MetaStatisticsItem struct {
	MetaKey      string  `json:"metaKey"`
	Lane         string  `json:"lane"`
	ChampionId   int     `json:"championId"`
	ChampionName string  `json:"championName"`
	MajorTag     string  `json:"majorTag"`
	MinorTag     *string `json:"minorTag"`
	ItemTree     []int   `json:"itemTree"`
	Count        int     `json:"count"`
	WinRate      float64 `json:"winRate"`
	AvgPickRate  float64 `json:"avgPickRate"`
	AvgBanRate   float64 `json:"avgBanRate"`
}

type ChampionDetailStatisticsRepository struct {
	mu        sync.RWMutex
	Cache     *ChampionDetailStatistics
	MetaCache *MetaStatistics
	config    RunnerConfig
}

func NewChampionDetailStatisticsRepository(config RunnerConfig) *ChampionDetailStatisticsRepository {
	cdsr := &ChampionDetailStatisticsRepository{
		Cache:     nil,
		MetaCache: nil,
		config:    config,
	}
	value, err := cdsr.Load()
	if err != nil {
		log.Warnf("Failed to preload %s: %v", cdsr.key(), err)
	}
	logLoadedSnapshot(cdsr.key(), value)
	return cdsr
}

func (cdsr *ChampionDetailStatisticsRepository) key() string {
	return "champion_detail_statistics"
}

func (cdsr *ChampionDetailStatisticsRepository) Period() time.Duration {
	return cdsr.config.Period
}

func (cdsr *ChampionDetailStatisticsRepository) Loop(ctx context.Context) {
	runLoop(ctx, cdsr.key(), cdsr.config, func(ctx context.Context) (time.Duration, error) {
		_, nextDelay, err := cdsr.collectCoordinatedScheduled(ctx)
		return nextDelay, err
	})
}

func (cdsr *ChampionDetailStatisticsRepository) Collect() (*ChampionDetailStatistics, error) {
	return cdsr.collectCoordinated(context.Background())
}

func (cdsr *ChampionDetailStatisticsRepository) collectCoordinated(ctx context.Context) (*ChampionDetailStatistics, error) {
	value, _, err := cdsr.collectCoordinatedScheduled(ctx)
	return value, err
}

func (cdsr *ChampionDetailStatisticsRepository) collectCoordinatedScheduled(ctx context.Context) (*ChampionDetailStatistics, time.Duration, error) {
	return collectCoordinated(
		ctx,
		cdsr.key(),
		cdsr.config,
		cdsr.collect,
		cdsr.setCache,
	)
}

func (cdsr *ChampionDetailStatisticsRepository) collect(database db.Context) (*ChampionDetailStatistics, error) {
	log.Infof("Collecting %s statistics...", cdsr.key())

	// collect recent versions
	versionCount := 3 // 4 ~ 6 weeks
	recentMatchGameVersions, recentMatchGameShortVersions, err := mixed.GetRecentMatchGameVersions_byDescendingShortVersion_withCount(database, versionCount)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	log.Debugf("recentMatchGameVersions: %v", recentMatchGameVersions)

	// collect data
	championDetailStatisticsMXDAOmap := make(map[int]statistics_models.ChampionDetailStatisticMXDAO)
	championDetailStatisticMXDAOs, err := statistics_models.GetChampionDetailStatisticMXDAOs(database, recentMatchGameVersions)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	for _, championDetailStatisticMXDAO := range championDetailStatisticMXDAOs {
		championDetailStatisticsMXDAOmap[championDetailStatisticMXDAO.ChampionId] = championDetailStatisticMXDAO
	}
	log.Debugf("championDetailStatisticMXDAOs fetch complete: %d, size: %s",
		len(championDetailStatisticMXDAOs), util.MemorySizeOfArray(championDetailStatisticMXDAOs))

	// collect champion pick count by team position
	championPositionStatisticsMXDAOmap := make(map[int]map[string]ChampionPositionStatistics)
	championPositionStatisticsMXDAOs, err := statistics_models.GetChampionPositionStatisticsMXDAOs(database, recentMatchGameVersions)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	for _, championPositionStatisticsMXDAO := range championPositionStatisticsMXDAOs {
		championId := championPositionStatisticsMXDAO.ChampionId
		if _, exists := championPositionStatisticsMXDAOmap[championId]; !exists {
			championPositionStatisticsMXDAOmap[championId] = make(map[string]ChampionPositionStatistics)
		}
		teamPosition := championPositionStatisticsMXDAO.TeamPosition
		if teamPosition != types.TeamPositionTop &&
			teamPosition != types.TeamPositionJungle &&
			teamPosition != types.TeamPositionMid &&
			teamPosition != types.TeamPositionAdc &&
			teamPosition != types.TeamPositionSupport {
			log.Warnf("team position not matched: %s", teamPosition)
			continue
		}
		championPositionStatisticsMXDAOmap[championId][teamPosition] = ChampionPositionStatistics{
			PickCount: championPositionStatisticsMXDAO.Total,
			WinCount:  championPositionStatisticsMXDAO.Win,
		}
	}

	if err := statistics_models.PrepareChampionDetailStatisticsSource(database, recentMatchGameVersions); err != nil {
		log.Error(err)
		return nil, err
	}

	// collect meta
	championDetailStatisticsMetaMap := make(map[int][]statistics_models.ChampionDetailStatisticsMetaMXDAO)
	metaStarted := time.Now()
	championDetailStatisticsMetaMXDAOs, err := statistics_models.GetChampionDetailStatisticsMetaMXDAOs(database)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	for _, meta := range championDetailStatisticsMetaMXDAOs {
		championId := meta.ChampionId
		if _, exists := championDetailStatisticsMetaMap[championId]; !exists {
			championDetailStatisticsMetaMap[championId] = make([]statistics_models.ChampionDetailStatisticsMetaMXDAO, 0)
		}
		championDetailStatisticsMetaMap[championId] = append(championDetailStatisticsMetaMap[championId], meta)
	}
	log.Infof("champion detail meta query complete: rows=%d duration=%s",
		len(championDetailStatisticsMetaMXDAOs), time.Since(metaStarted))
	log.Debugf("championDetailStatisticsMetaMXDAOs fetch complete: %d, size: %s",
		len(championDetailStatisticsMetaMXDAOs), util.MemorySizeOfArray(championDetailStatisticsMetaMXDAOs))

	// collect counter data
	championCounterStatisticsMap := make(map[int][]statistics_models.ChampionCounterStatisticsMXDAO) // key: championId
	counterStarted := time.Now()
	championCounterStatisticsMXDAOs, err := statistics_models.GetChampionCounterStatisticsMXDAOs(database)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	for _, counter := range championCounterStatisticsMXDAOs {
		championId := counter.ChampionId
		if _, exists := championCounterStatisticsMap[championId]; !exists {
			championCounterStatisticsMap[championId] = make([]statistics_models.ChampionCounterStatisticsMXDAO, 0)
		}
		championCounterStatisticsMap[championId] = append(championCounterStatisticsMap[championId], counter)
	}
	log.Infof("champion detail counter query complete: rows=%d duration=%s",
		len(championCounterStatisticsMXDAOs), time.Since(counterStarted))
	log.Debugf("championCounterStatisticsMXDAOs fetch complete: %d, size: %s",
		len(championCounterStatisticsMXDAOs), util.MemorySizeOfArray(championCounterStatisticsMXDAOs))

	stats := make(map[int]ChampionDetailStatisticsItem)
	for key, champion := range service.Champions {
		championId, err := strconv.Atoi(key)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		championPositionStatisticsMXDAO, exists := championPositionStatisticsMXDAOmap[championId]
		if !exists {
			log.Warnf("championId not found: %d", championId)
			continue
		}

		metas, ok := championDetailStatisticsMetaMap[championId]
		if !ok {
			log.Warnf("championId not found: %d", championId)
			continue
		}

		counters, exists := championCounterStatisticsMap[championId]
		if !exists {
			log.Warnf("championId not found: %d", championId)
			counters = make([]statistics_models.ChampionCounterStatisticsMXDAO, 0)
		}

		metaTree, err := cdsr.collectEachChampionMetas(championId, championPositionStatisticsMXDAO, metas, counters)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		//// get champion detail meta statistics
		//metaTree, err := cdsr.collectEachChampionMetas(championId)
		//if err != nil {
		//	log.Error(err)
		//	return nil, err
		//}

		e := ChampionDetailStatisticsItem{
			ChampionId:    championId,
			ChampionName:  champion.Name,
			ChampionTitle: champion.Title,
			ChampionStory: champion.Blurb,
			Win:           0,
			Total:         0,
			AvgPickRate:   0,
			AvgBanRate:    0,
			ExtraStats: ChampionDetailStatisticsExtraStats{
				AvgMinionsKilled: 0,
				AvgKills:         0,
				AvgDeaths:        0,
				AvgAssists:       0,
				AvgGoldEarned:    0,
			},
			NormalizedStats: ChampionDetailStatisticsNormalizedStats{
				AvgTotalHealPerSec:           0,
				AvgVisionScorePerSec:         0,
				AvgTotalDamageTakenPerSec:    0,
				AvgTotalTimeCCDealtPerSec:    0,
				AvgDamageSelfMitigatedPerSec: 0,
			},
			MetaTree: *metaTree,
		}

		if championDetailStatisticMXDAO, exists := championDetailStatisticsMXDAOmap[championId]; exists {
			e.Win = championDetailStatisticMXDAO.Win
			e.Total = championDetailStatisticMXDAO.Total
			e.AvgPickRate = championDetailStatisticMXDAO.PickRate
			e.AvgBanRate = championDetailStatisticMXDAO.BanRate
			e.AvgWinRate = float64(championDetailStatisticMXDAO.Win) / float64(championDetailStatisticMXDAO.Total)
			e.ExtraStats = ChampionDetailStatisticsExtraStats{
				AvgMinionsKilled: championDetailStatisticMXDAO.AvgMinionsKilled,
				AvgKills:         championDetailStatisticMXDAO.AvgKills,
				AvgDeaths:        championDetailStatisticMXDAO.AvgDeaths,
				AvgAssists:       championDetailStatisticMXDAO.AvgAssists,
				AvgGoldEarned:    championDetailStatisticMXDAO.AvgGoldEarned,
			}
			e.NormalizedStats = ChampionDetailStatisticsNormalizedStats{
				AvgTotalHealPerSec:           championDetailStatisticMXDAO.AvgHealPerSec,
				AvgVisionScorePerSec:         championDetailStatisticMXDAO.AvgVisionScorePerSec,
				AvgTotalDamageTakenPerSec:    championDetailStatisticMXDAO.AvgDamageTakenPerSec,
				AvgTotalTimeCCDealtPerSec:    championDetailStatisticMXDAO.AvgTimeCCDealtPerSec,
				AvgDamageSelfMitigatedPerSec: championDetailStatisticMXDAO.AvgDamageSelfMitigatedPerSec,
			}
		}

		stats[championId] = e
	}

	collected := &ChampionDetailStatistics{
		UpdatedAt: time.Now(),
		Patches:   recentMatchGameShortVersions,
		Data:      stats,
	}

	log.Infof("%s statistics collected successfully", cdsr.key())
	return collected, nil
}

func (cdsr *ChampionDetailStatisticsRepository) collectEachChampionMetas(
	championId int,
	countByPosition map[string]ChampionPositionStatistics,
	championDetailStatisticsMetaMXDAOs []statistics_models.ChampionDetailStatisticsMetaMXDAO,
	championCounterMXDAOs []statistics_models.ChampionCounterStatisticsMXDAO,
) (*ChampionDetailStatisticsPositionMetaTree, error) {

	metaTrees := &ChampionDetailStatisticsPositionMetaTree{
		Top:     nil,
		Jungle:  nil,
		Mid:     nil,
		Adc:     nil,
		Support: nil,
	}

	metaMXDAOsByPosition := map[string][]statistics_models.ChampionDetailStatisticsMetaMXDAO{
		types.TeamPositionTop:     make([]statistics_models.ChampionDetailStatisticsMetaMXDAO, 0),
		types.TeamPositionJungle:  make([]statistics_models.ChampionDetailStatisticsMetaMXDAO, 0),
		types.TeamPositionMid:     make([]statistics_models.ChampionDetailStatisticsMetaMXDAO, 0),
		types.TeamPositionAdc:     make([]statistics_models.ChampionDetailStatisticsMetaMXDAO, 0),
		types.TeamPositionSupport: make([]statistics_models.ChampionDetailStatisticsMetaMXDAO, 0),
	}
	for _, championDetailStatisticsMetaMXDAO := range championDetailStatisticsMetaMXDAOs {
		teamPosition := championDetailStatisticsMetaMXDAO.TeamPosition
		if _, exists := metaMXDAOsByPosition[teamPosition]; !exists {
			log.Warnf("team position not exists: %s", teamPosition)
			continue
		}
		metaMXDAOsByPosition[teamPosition] = append(metaMXDAOsByPosition[teamPosition], championDetailStatisticsMetaMXDAO)
	}

	// key: teamPosition -> counterChampionId -> ChampionCounterStatisticsMXDAO
	countersByPositionMap := make(map[string]map[int]statistics_models.ChampionCounterStatisticsMXDAO)
	for _, counter := range championCounterMXDAOs {
		teamPosition := counter.TeamPosition
		if _, exists := countersByPositionMap[teamPosition]; !exists {
			countersByPositionMap[teamPosition] = make(map[int]statistics_models.ChampionCounterStatisticsMXDAO)
		}
		countersByPositionMap[teamPosition][counter.EnemyChampionId] = counter
	}

	for teamPosition, metaMXDAOs := range metaMXDAOsByPosition {
		pickCount, winCount := 0, 0
		positionStatistics, ok := countByPosition[teamPosition]
		if ok {
			pickCount = positionStatistics.PickCount
			winCount = positionStatistics.WinCount
		} else {
			pickCount = 0
			winCount = 0
		}

		positionItems := make([]int, 0)
		for _, metaMXDAO := range metaMXDAOs {
			majorItems := getValidItems([]*int{
				&metaMXDAO.Item0Id,
				&metaMXDAO.Item1Id,
				&metaMXDAO.Item2Id,
				metaMXDAO.Item3Id,
				metaMXDAO.Item4Id,
				metaMXDAO.Item5Id,
			})
			for _, itemId := range majorItems {
				positionItems = append(positionItems, itemId)
			}
		}

		positionItemCounts := getDescSortedPositionItemCounts(positionItems)
		positionItemTags := getPositionItemTags(positionItems)
		lowDepthItemRecommends, err := getLowDepthItemRecommendations(championId, teamPosition, positionItemTags)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		metaGroup := make(MetaGroup, 0)
		for _, metaMXDAO := range metaMXDAOs {
			majorItems := getValidItems([]*int{
				&metaMXDAO.Item0Id,
				&metaMXDAO.Item1Id,
				&metaMXDAO.Item2Id,
				metaMXDAO.Item3Id,
				metaMXDAO.Item4Id,
				metaMXDAO.Item5Id,
			})

			// get categories of tags from major items
			tagCategories := make(map[string]int)
			for _, itemId := range majorItems {
				item := service.Items[itemId]
				for _, tag := range item.Tags {
					category := types.GetItemCategories(tag)
					if category != nil {
						if _, exists := tagCategories[*category]; !exists {
							tagCategories[*category] = 0
						}
						tagCategories[*category]++
					}
				}
			}

			// collect counts of tag categories
			type CategoryCount struct {
				category string
				count    int
			}
			categoryCounts := make([]CategoryCount, 0)
			maxCount := 0
			for category, count := range tagCategories {
				categoryCounts = append(categoryCounts, CategoryCount{category: category, count: count})
				if count > maxCount {
					maxCount = count
				}
			}

			var majorTag string
			var minorTag *string
			if maxCount > 1 {
				// sort categories by count (desc)
				sort.SliceStable(categoryCounts, func(i, j int) bool {
					if categoryCounts[i].count == categoryCounts[j].count {
						return categoryCounts[i].category < categoryCounts[j].category
					}
					return categoryCounts[i].count > categoryCounts[j].count
				})

				// get major tag and minor tag
				if len(categoryCounts) > 0 {
					majorTag = categoryCounts[0].category
					if len(categoryCounts) > 1 {
						minorTag = &categoryCounts[1].category
					}
				} else {
					continue
				}
			} else {
				// set major tag as first category & minor tag as nil
				if len(categoryCounts) > 0 {
					majorTag = categoryCounts[0].category
				} else {
					continue
				}
			}

			startItems, basicItems, subItems, err := getItemTrees(positionItemCounts, lowDepthItemRecommends, majorItems)
			if err != nil {
				log.Error(err)
				return nil, err
			}

			uuid := uuid2.New()
			pickRate := 0.0
			if pickCount > 0 {
				pickRate = float64(metaMXDAO.Total) / float64(pickCount)
			}
			item0Id := metaMXDAO.Item0Id
			item1Id := metaMXDAO.Item1Id
			item2Id := metaMXDAO.Item2Id
			metaPick := MetaPick{
				Id:                uuid.String(),
				Summoner1Id:       metaMXDAO.Summoner1Id,
				Summoner2Id:       metaMXDAO.Summoner2Id,
				PrimaryStyleId:    metaMXDAO.PrimaryStyle,
				PrimaryPerk0:      metaMXDAO.PrimaryPerk0,
				PrimaryPerk1:      metaMXDAO.PrimaryPerk1,
				PrimaryPerk2:      metaMXDAO.PrimaryPerk2,
				PrimaryPerk3:      metaMXDAO.PrimaryPerk3,
				SubStyleId:        metaMXDAO.SubStyle,
				SubPerk0:          metaMXDAO.SubPerk0,
				SubPerk1:          metaMXDAO.SubPerk1,
				StatPerkDefenseId: metaMXDAO.StatPerkDefense,
				StatPerkFlexId:    metaMXDAO.StatPerkFlex,
				StatPerkOffenseId: metaMXDAO.StatPerkOffense,
				Item0:             &item0Id,
				Item1:             &item1Id,
				Item2:             &item2Id,
				Item3:             metaMXDAO.Item3Id,
				Item4:             metaMXDAO.Item4Id,
				Item5:             metaMXDAO.Item5Id,
				Wins:              metaMXDAO.Wins,
				Total:             metaMXDAO.Total,
				WinRate:           metaMXDAO.WinRate,
				PickRate:          pickRate,
				MetaRank:          metaMXDAO.MetaRank,
				MajorTag:          majorTag,
				MinorTag:          minorTag,
				StartItems:        startItems,
				BasicItems:        basicItems,
				SubItems:          subItems,
			}

			metaGroup = append(metaGroup, metaPick)
		}

		// categorize meta picks by concept (concept = major tag + minor tag)
		conceptGroups := make(map[string]MetaGroup) // concept -> MetaGroup
		for _, metaPick := range metaGroup {
			concept := metaPick.MajorTag
			if metaPick.MinorTag != nil {
				concept += "-" + *metaPick.MinorTag
			}
			if _, exists := conceptGroups[concept]; !exists {
				conceptGroups[concept] = make(MetaGroup, 0)
			}
			conceptGroups[concept] = append(conceptGroups[concept], metaPick)
		}

		// pick popular concept groups
		concepts := make([]string, 0)
		for concept, _ := range conceptGroups {
			concepts = append(concepts, concept)
		}
		// sort concept groups (pickRate desc, winRate desc)
		sort.SliceStable(concepts, func(i, j int) bool {
			conceptI, conceptJ := concepts[i], concepts[j]
			groupI, groupJ := conceptGroups[conceptI], conceptGroups[conceptJ]
			pickCountI, pickCountJ := groupI.getTotalPickCount(), groupJ.getTotalPickCount()
			winRateI, winRateJ := groupI.getTotalWinRate(), groupJ.getTotalWinRate()
			if pickCountI != pickCountJ {
				return pickCountI > pickCountJ
			}
			return winRateI > winRateJ
		})

		popularConcepts := make([]string, 0)
		nonPopularConcepts := make([]string, 0)
		for ind, concept := range concepts {
			if ind < 5 {
				popularConcepts = append(popularConcepts, concept)
			} else {
				nonPopularConcepts = append(nonPopularConcepts, concept)
			}
		}

		var minorConcept *string // minor concept has low pick rate (with lower limit) but high win rate
		// sort non-popular concept groups (winRate desc, pickRate desc)
		sort.SliceStable(nonPopularConcepts, func(i, j int) bool {
			conceptI, conceptJ := concepts[i], concepts[j]
			groupI, groupJ := conceptGroups[conceptI], conceptGroups[conceptJ]
			pickCountI, pickCountJ := groupI.getTotalPickCount(), groupJ.getTotalPickCount()
			winRateI, winRateJ := groupI.getTotalWinRate(), groupJ.getTotalWinRate()
			if winRateI != winRateJ {
				return winRateI > winRateJ
			}
			return pickCountI > pickCountJ
		})
		if len(nonPopularConcepts) > 0 {
			minorConcept = &nonPopularConcepts[0]
		}

		pickMostPickRateMetas := func(metaGroup MetaGroup, count int) []MetaPick {
			metas := make([]MetaPick, 0)
			// sort meta picks (pickRate desc, winRate desc)
			sort.SliceStable(metaGroup, func(i, j int) bool {
				metaI, metaJ := metaGroup[i], metaGroup[j]
				if metaI.PickRate != metaJ.PickRate {
					return metaI.PickRate > metaJ.PickRate
				}
				return metaI.WinRate > metaJ.WinRate
			})
			// pick top {count} meta picks
			for ind, meta := range metaGroup {
				if ind < count {
					metas = append(metas, meta)
				} else {
					break
				}
			}
			return metas
		}

		majorMetaPicks := make([]MetaPick, 0)
		for _, concept := range popularConcepts {
			popularMetaGroup := conceptGroups[concept]
			popularMetas := pickMostPickRateMetas(popularMetaGroup, 3)
			if len(popularMetas) > 0 {
				majorMetaPicks = append(majorMetaPicks, popularMetas...)
			}
		}

		minorMetaPicks := make([]MetaPick, 0)
		if minorConcept != nil {
			minorMetaGroup := conceptGroups[*minorConcept]
			minorMetas := pickMostPickRateMetas(minorMetaGroup, 3)
			if len(minorMetas) > 0 {
				minorMetaPicks = append(minorMetaPicks, minorMetas...)
			}
		}

		counterMap := make(map[int]ChampionCounterStatistics)
		counters, exists := countersByPositionMap[teamPosition]
		if !exists {
			counters = make(map[int]statistics_models.ChampionCounterStatisticsMXDAO)
		}
		for counterChampionId, counter := range counters {
			counterChampion, exists := service.Champions[strconv.Itoa(counter.EnemyChampionId)]
			if !exists {
				log.Warnf("champion not found: %d", counter.EnemyChampionId)
				continue
			}

			majorItems := getValidItems([]*int{
				counter.Item0Id,
				counter.Item1Id,
				counter.Item2Id,
				counter.Item3Id,
				counter.Item4Id,
				counter.Item5Id,
			})

			if len(majorItems) < 3 {
				lack := 3 - len(majorItems)
				for i := 0; i < lack && i < len(positionItemCounts); i++ {
					majorItems = append(majorItems, positionItemCounts[i].itemId)
				}
			}

			startItems, basicItems, subItems, err := getItemTrees(positionItemCounts, lowDepthItemRecommends, majorItems)
			if err != nil {
				log.Error(err)
				return nil, err
			}

			c := ChampionCounterStatistics{
				CounterChampionId:   counter.EnemyChampionId,
				CounterChampionName: counterChampion.Name,
				AvgKills:            counter.AvgKills,
				AvgDeaths:           counter.AvgDeaths,
				AvgAssists:          counter.AvgAssists,
				CounterAvgKills:     counter.EnemyAvgKills,
				CounterAvgDeaths:    counter.EnemyAvgDeaths,
				CounterAvgAssists:   counter.EnemyAvgAssists,
				Summoner1Id:         counter.Summoner1Id,
				Summoner2Id:         counter.Summoner2Id,
				MajorPerkGroup:      nil,
				MinorPerkGroup:      nil,
				PerkExtra:           nil,
				MainSlots:           make([]PerkSlot, 0),
				SubSlots:            make([]PerkSlot, 0),
				StatSlots:           make([]PerkSlot, 0),
				StartItemTree:       startItems,
				BasicItemTree:       basicItems,
				ItemTree:            majorItems,
				SubItemTree:         subItems,
				Count:               counter.Total,
				Win:                 counter.Wins,
				WinRate:             counter.WinRate,
				ExpectedWinRate:     counter.TotalWinRate,
				IsValid:             true,
			}

			if counter.PrimaryStyle != nil {
				primaryPerkStyle, ok := service.PerkStyles[*counter.PrimaryStyle]
				if !ok {
					return nil, fmt.Errorf("primary perk style not found: %d", *counter.PrimaryStyle)
				}
				primaryPerks := make([]int, 0)
				for _, subPerk := range []*int{counter.PrimaryPerk0, counter.PrimaryPerk1, counter.PrimaryPerk2, counter.PrimaryPerk3} {
					if subPerk != nil {
						primaryPerks = append(primaryPerks, *subPerk)
					}
				}
				c.MajorPerkGroup = &PerkGroup{
					PerkStyleName: primaryPerkStyle.Name,
					PerkStyleId:   *counter.PrimaryStyle,
					SubPerks:      primaryPerks,
				}
			} else {
				c.IsValid = false
			}

			if counter.SubStyle != nil {
				subPerkStyle, ok := service.PerkStyles[*counter.SubStyle]
				if !ok {
					return nil, fmt.Errorf("sub perk style not found: %d", *counter.SubStyle)
				}
				subPerks := make([]int, 0)
				for _, subPerk := range []*int{counter.SubPerk0, counter.SubPerk1} {
					if subPerk != nil {
						subPerks = append(subPerks, *subPerk)
					}
				}
				c.MinorPerkGroup = &PerkGroup{
					PerkStyleName: subPerkStyle.Name,
					PerkStyleId:   *counter.SubStyle,
					SubPerks:      subPerks,
				}
			} else {
				c.IsValid = false
			}

			if counter.PrimaryStyle != nil && counter.SubStyle != nil {
				mainSlots, subSlots, statSlots, err := getSlotsFromStyle(*counter.PrimaryStyle, *counter.SubStyle)
				if err != nil {
					return nil, err
				}

				c.MainSlots = mainSlots
				c.SubSlots = subSlots
				c.StatSlots = statSlots
			}

			if counter.StatPerkDefense != nil && counter.StatPerkFlex != nil && counter.StatPerkOffense != nil {
				c.PerkExtra = &PerkExtra{
					StatDefenseId: *counter.StatPerkDefense,
					StatFlexId:    *counter.StatPerkFlex,
					StatOffenseId: *counter.StatPerkOffense,
				}
			} else {
				c.IsValid = false
			}

			if counter.EnemyWinRate == nil {
				c.IsValid = false
			}

			counterMap[counterChampionId] = c
		}

		metaTree := ChampionDetailStatisticsMetaTree{
			MajorMetaPicks: make([]ChampionDetailStatisticsMeta, 0),
			MinorMetaPicks: make([]ChampionDetailStatisticsMeta, 0),
			MetaPicks:      make([]ChampionDetailStatisticsMeta, 0),
			PickCount:      pickCount,
			WinCount:       winCount,
			CounterMap:     counterMap,
		}
		for _, metaPick := range majorMetaPicks {
			meta, err := metaPick.toRealMeta()
			if err != nil {
				log.Error(err)
				return nil, err
			}
			metaTree.MajorMetaPicks = append(metaTree.MajorMetaPicks, *meta)
		}
		for _, metaPick := range minorMetaPicks {
			meta, err := metaPick.toRealMeta()
			if err != nil {
				log.Error(err)
				return nil, err
			}
			metaTree.MinorMetaPicks = append(metaTree.MinorMetaPicks, *meta)
		}
		for _, metaPick := range metaGroup {
			meta, err := metaPick.toRealMeta()
			if err != nil {
				log.Error(err)
				return nil, err
			}
			metaTree.MetaPicks = append(metaTree.MetaPicks, *meta)
		}

		if teamPosition == types.TeamPositionTop {
			metaTrees.Top = &metaTree
		} else if teamPosition == types.TeamPositionJungle {
			metaTrees.Jungle = &metaTree
		} else if teamPosition == types.TeamPositionMid {
			metaTrees.Mid = &metaTree
		} else if teamPosition == types.TeamPositionAdc {
			metaTrees.Adc = &metaTree
		} else if teamPosition == types.TeamPositionSupport {
			metaTrees.Support = &metaTree
		} else {
			log.Warnf("teamPosition not matched: %s", teamPosition)
		}
	}

	return metaTrees, nil
}

func (cdsr *ChampionDetailStatisticsRepository) Save() error {
	return saveCurrentSnapshot(cdsr.key(), cdsr.getCache())
}

func (cdsr *ChampionDetailStatisticsRepository) Load() (*ChampionDetailStatistics, error) {
	if cached := cdsr.getCache(); cached != nil {
		return cached, nil
	}
	cached, err := loadSnapshot[ChampionDetailStatistics](cdsr.key())
	if err != nil {
		return nil, err
	}
	if cached != nil {
		cdsr.setCache(cached)
	}
	return cached, nil
}

func (cdsr *ChampionDetailStatisticsRepository) LoadMeta() (*MetaStatistics, error) {
	cdsr.mu.RLock()
	meta := cdsr.MetaCache
	cdsr.mu.RUnlock()
	if meta != nil {
		return meta, nil
	}

	data, err := cdsr.Load()
	if err != nil {
		return nil, err
	}
	meta = buildMetaStatistics(data)
	cdsr.mu.RLock()
	cachedMeta := cdsr.MetaCache
	cdsr.mu.RUnlock()
	if cachedMeta != nil {
		return cachedMeta, nil
	}
	cdsr.mu.Lock()
	if cdsr.MetaCache == nil {
		cdsr.MetaCache = meta
	}
	meta = cdsr.MetaCache
	cdsr.mu.Unlock()
	return meta, nil
}

func (cdsr *ChampionDetailStatisticsRepository) getCache() *ChampionDetailStatistics {
	cdsr.mu.RLock()
	defer cdsr.mu.RUnlock()
	return cdsr.Cache
}

func (cdsr *ChampionDetailStatisticsRepository) setCache(cache *ChampionDetailStatistics) {
	metaCache := buildMetaStatistics(cache)
	cdsr.mu.Lock()
	defer cdsr.mu.Unlock()
	cdsr.Cache = cache
	cdsr.MetaCache = metaCache
}

func buildMetaStatistics(source *ChampionDetailStatistics) *MetaStatistics {
	if source == nil {
		return nil
	}

	result := &MetaStatistics{
		UpdatedAt: source.UpdatedAt,
		Patches:   source.Patches,
		Data:      make([]MetaStatisticsItem, 0),
	}
	type laneMetaTree struct {
		lane string
		tree *ChampionDetailStatisticsMetaTree
	}

	for _, champion := range source.Data {
		lanes := []laneMetaTree{
			{lane: "top", tree: champion.MetaTree.Top},
			{lane: "jungle", tree: champion.MetaTree.Jungle},
			{lane: "mid", tree: champion.MetaTree.Mid},
			{lane: "adc", tree: champion.MetaTree.Adc},
			{lane: "support", tree: champion.MetaTree.Support},
		}
		totalPickCount := 0
		for _, lane := range lanes {
			if lane.tree != nil {
				totalPickCount += lane.tree.PickCount
			}
		}
		if totalPickCount == 0 {
			continue
		}

		for _, lane := range lanes {
			if lane.tree == nil || lane.tree.PickCount == 0 || float64(lane.tree.PickCount)/float64(totalPickCount) < 0.15 {
				continue
			}
			var highestWinRate *ChampionDetailStatisticsMeta
			var highestPickCount *ChampionDetailStatisticsMeta
			for i := range lane.tree.MetaPicks {
				meta := &lane.tree.MetaPicks[i]
				if meta.Count < 5 || float64(meta.Count)/float64(lane.tree.PickCount) < 0.1 {
					continue
				}
				if highestWinRate == nil || meta.WinRate > highestWinRate.WinRate {
					highestWinRate = meta
				}
				if highestPickCount == nil || meta.Count > highestPickCount.Count {
					highestPickCount = meta
				}
			}

			selected := []*ChampionDetailStatisticsMeta{highestWinRate, highestPickCount}
			for index, meta := range selected {
				if meta == nil || (index == 1 && highestWinRate != nil && meta.MetaKey == highestWinRate.MetaKey) {
					continue
				}
				items := meta.ItemTree
				if len(items) > 3 {
					items = items[:3]
				}
				result.Data = append(result.Data, MetaStatisticsItem{
					MetaKey:      meta.MetaKey,
					Lane:         lane.lane,
					ChampionId:   champion.ChampionId,
					ChampionName: champion.ChampionName,
					MajorTag:     meta.MajorTag,
					MinorTag:     meta.MinorTag,
					ItemTree:     items,
					Count:        meta.Count,
					WinRate:      meta.WinRate,
					AvgPickRate:  float64(meta.Count) / float64(lane.tree.PickCount),
					AvgBanRate:   champion.AvgBanRate,
				})
			}
		}
	}
	sort.SliceStable(result.Data, func(i, j int) bool {
		return result.Data[i].ChampionName < result.Data[j].ChampionName
	})
	return result
}
