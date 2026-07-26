package statistics

import (
	"context"
	log "github.com/shyunku-libraries/go-logger"
	"sync"
	"team.gg-server/libs/db"
	"team.gg-server/models/mixed/statistics_models"
	"team.gg-server/service"
	"team.gg-server/util"
	"time"
)

/* ----------------------- Tier statistics_models ----------------------- */

type TierStatisticsTopSummoners struct {
	Puuid         string `json:"puuid"`
	ProfileIconId int    `json:"profileIconId"`
	GameName      string `json:"gameName"`
	TagLine       string `json:"tagLine"`

	LeaguePoints int `json:"leaguePoints"`
	Wins         int `json:"wins"`
	Losses       int `json:"losses"`
	Ranks        int `json:"ranks"`
}

type TierStatisticsRankGroup struct {
	Rank      service.Rank                 `json:"rank"`
	Level     int                          `json:"level"`
	Summoners int                          `json:"summoners"`
	Rankers   []TierStatisticsTopSummoners `json:"rankers"`
}

type TierStatisticsTierGroup struct {
	Tier       service.Tier              `json:"tier"`
	Level      int                       `json:"level"`
	RankGroups []TierStatisticsRankGroup `json:"rankGroups"`
}

type TierStatisticsQueueGroup struct {
	QueueType  string                    `json:"queueType"`
	TierGroups []TierStatisticsTierGroup `json:"rankGroups"`
}

type TierStatistics struct {
	UpdatedAt   time.Time                  `json:"updatedAt"`
	QueueGroups []TierStatisticsQueueGroup `json:"queueGroups"`
}

type TierStatisticsRepository struct {
	mu     sync.RWMutex
	Cache  *TierStatistics
	config RunnerConfig
}

func NewTierStatisticsRepository(config RunnerConfig) *TierStatisticsRepository {
	tsr := &TierStatisticsRepository{
		Cache:  nil,
		config: config,
	}
	value, err := tsr.Load()
	if err != nil {
		log.Warnf("Failed to preload %s: %v", tsr.key(), err)
	}
	logLoadedSnapshot(tsr.key(), value)
	return tsr
}

func (tsr *TierStatisticsRepository) key() string {
	return "tier_statistics"
}

func (tsr *TierStatisticsRepository) Period() time.Duration {
	return tsr.config.Period
}

func (tsr *TierStatisticsRepository) Loop(ctx context.Context) {
	runLoop(ctx, tsr.key(), tsr.config, func(ctx context.Context) (time.Duration, error) {
		_, nextDelay, err := tsr.collectCoordinatedScheduled(ctx)
		return nextDelay, err
	})
}

func (tsr *TierStatisticsRepository) Collect() (*TierStatistics, error) {
	return tsr.collectCoordinated(context.Background())
}

func (tsr *TierStatisticsRepository) collectCoordinated(ctx context.Context) (*TierStatistics, error) {
	value, _, err := tsr.collectCoordinatedScheduled(ctx)
	return value, err
}

func (tsr *TierStatisticsRepository) collectCoordinatedScheduled(ctx context.Context) (*TierStatistics, time.Duration, error) {
	return collectCoordinated(
		ctx,
		tsr.key(),
		tsr.config,
		tsr.collect,
		tsr.setCache,
	)
}

func (tsr *TierStatisticsRepository) collect(database db.Context) (*TierStatistics, error) {
	log.Infof("Collecting %s statistics...", tsr.key())
	timer := util.NewTimerWithName("TierStatisticsRepository")
	timer.Start()

	// collect data
	tierCountMXDAOs, err := statistics_models.GetTierStatisticsTierCountMXDAOs(database)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	topRankersMXDAOs, err := statistics_models.GetTierStatisticsTopRankersMXDAOs(database, 30)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	countMap := make(map[string]map[string]map[string]int)
	for _, tierCountMXDAO := range tierCountMXDAOs {
		if _, exists := countMap[tierCountMXDAO.QueueType]; !exists {
			countMap[tierCountMXDAO.QueueType] = make(map[string]map[string]int)
		}
		if _, exists := countMap[tierCountMXDAO.QueueType][tierCountMXDAO.Tier]; !exists {
			countMap[tierCountMXDAO.QueueType][tierCountMXDAO.Tier] = make(map[string]int)
		}
		countMap[tierCountMXDAO.QueueType][tierCountMXDAO.Tier][tierCountMXDAO.LeagueRank] = tierCountMXDAO.Count
	}

	statisticsMap := make(map[string]map[string]map[string][]TierStatisticsTopSummoners)
	for _, topRankerMXDAO := range topRankersMXDAOs {
		if _, exists := statisticsMap[topRankerMXDAO.QueueType]; !exists {
			statisticsMap[topRankerMXDAO.QueueType] = make(map[string]map[string][]TierStatisticsTopSummoners)
		}
		if _, exists := statisticsMap[topRankerMXDAO.QueueType][topRankerMXDAO.Tier]; !exists {
			statisticsMap[topRankerMXDAO.QueueType][topRankerMXDAO.Tier] = make(map[string][]TierStatisticsTopSummoners, 0)
		}
		if _, exists := statisticsMap[topRankerMXDAO.QueueType][topRankerMXDAO.Tier][topRankerMXDAO.LeagueRank]; !exists {
			statisticsMap[topRankerMXDAO.QueueType][topRankerMXDAO.Tier][topRankerMXDAO.LeagueRank] = make([]TierStatisticsTopSummoners, 0)
		}
		statisticsMap[topRankerMXDAO.QueueType][topRankerMXDAO.Tier][topRankerMXDAO.LeagueRank] = append(
			statisticsMap[topRankerMXDAO.QueueType][topRankerMXDAO.Tier][topRankerMXDAO.LeagueRank],
			TierStatisticsTopSummoners{
				ProfileIconId: topRankerMXDAO.ProfileIconId,
				GameName:      topRankerMXDAO.GameName,
				TagLine:       topRankerMXDAO.TagLine,
				Puuid:         topRankerMXDAO.Puuid,
				LeaguePoints:  topRankerMXDAO.LeaguePoints,
				Wins:          topRankerMXDAO.Wins,
				Losses:        topRankerMXDAO.Losses,
				Ranks:         topRankerMXDAO.Ranks,
			},
		)
	}

	queueGroups := make([]TierStatisticsQueueGroup, 0)
	for queueType, tierMap := range statisticsMap {
		tierGroups := make([]TierStatisticsTierGroup, 0)
		tierCountMap, exists := countMap[queueType]
		if !exists {
			log.Errorf("tier count map not found for queue type: %s", queueType)
			continue
		}

		for tier, rankMap := range tierMap {
			rankGroups := make([]TierStatisticsRankGroup, 0)
			rankCountMap, exists := tierCountMap[tier]
			if !exists {
				log.Errorf("tier count map not found for tier: %s", tier)
				continue
			}

			for rank, topSummoners := range rankMap {
				count, exists := rankCountMap[rank]
				if !exists {
					log.Errorf("tier count not found for rank: %s", rank)
					continue
				}

				rankLevel, err := service.GetRankLevel(service.Tier(tier), service.Rank(rank))
				if err != nil {
					log.Error(err)
					return nil, err
				}
				rankGroups = append(rankGroups, TierStatisticsRankGroup{
					Rank:      service.Rank(rank),
					Level:     rankLevel,
					Summoners: count,
					Rankers:   topSummoners,
				})
			}

			tierLevel, err := service.GetTierLevel(service.Tier(tier))
			if err != nil {
				log.Error(err)
				return nil, err
			}
			tierGroups = append(tierGroups, TierStatisticsTierGroup{
				Tier:       service.Tier(tier),
				Level:      tierLevel,
				RankGroups: rankGroups,
			})
		}
		queueGroups = append(queueGroups, TierStatisticsQueueGroup{
			QueueType:  queueType,
			TierGroups: tierGroups,
		})
	}

	collected := &TierStatistics{
		UpdatedAt:   time.Now(),
		QueueGroups: queueGroups,
	}

	log.Infof("%s statistics collected successfully in %s", tsr.key(), timer.GetDurationString())
	return collected, nil
}

func (tsr *TierStatisticsRepository) Save() error {
	return saveCurrentSnapshot(tsr.key(), tsr.getCache())
}

func (tsr *TierStatisticsRepository) Load() (*TierStatistics, error) {
	if cached := tsr.getCache(); cached != nil {
		return cached, nil
	}
	cached, err := loadSnapshot[TierStatistics](tsr.key())
	if err != nil {
		return nil, err
	}
	if cached != nil {
		tsr.setCache(cached)
	}
	return cached, nil
}

func (tsr *TierStatisticsRepository) getCache() *TierStatistics {
	tsr.mu.RLock()
	defer tsr.mu.RUnlock()
	return tsr.Cache
}

func (tsr *TierStatisticsRepository) setCache(cache *TierStatistics) {
	tsr.mu.Lock()
	defer tsr.mu.Unlock()
	tsr.Cache = cache
}
