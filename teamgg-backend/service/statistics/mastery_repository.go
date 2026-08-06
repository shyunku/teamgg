package statistics

import (
	"context"
	"fmt"
	log "github.com/shyunku-libraries/go-logger"
	"strconv"
	"sync"
	"team.gg-server/libs/db"
	"team.gg-server/models/mixed/statistics_models"
	"team.gg-server/service"
	"time"
)

/* ----------------------- Mastery statistics_models ----------------------- */

type MasteryStatisticsTopSummoners struct {
	Puuid         string `json:"puuid"`
	ProfileIconId int    `json:"profileIconId"`
	GameName      string `json:"gameName"`
	TagLine       string `json:"tagLine"`

	ChampionPoints int `json:"championPoints"`
	Ranks          int `json:"ranks"`
}

type MasteryStatisticsItem struct {
	ChampionId   int    `json:"championId"`
	ChampionName string `json:"championName"`

	AvgMastery   float64 `json:"avgMastery"`
	MaxMastery   int64   `json:"maxMastery"`
	TotalMastery int64   `json:"totalMastery"`

	MasteredCount int                             `json:"masteredCount"`
	Summoners     int                             `json:"summoners"`
	Rankers       []MasteryStatisticsTopSummoners `json:"rankers"`
}

type MasteryStatistics struct {
	UpdatedAt     time.Time               `json:"updatedAt"`
	MasteryGroups []MasteryStatisticsItem `json:"masteryGroups"`
}

type MasteryStatisticsRepository struct {
	mu     sync.RWMutex
	Cache  *MasteryStatistics
	config RunnerConfig
}

func NewMasteryStatisticsRepository(config RunnerConfig) *MasteryStatisticsRepository {
	msr := &MasteryStatisticsRepository{
		Cache:  nil,
		config: config,
	}
	value, err := msr.Load()
	if err != nil {
		log.Warnf("Failed to preload %s: %v", msr.key(), err)
	}
	logLoadedSnapshot(msr.key(), value)
	return msr
}

func (msr *MasteryStatisticsRepository) key() string {
	return "mastery_statistics"
}

func (msr *MasteryStatisticsRepository) Period() time.Duration {
	return msr.config.Period
}

func (msr *MasteryStatisticsRepository) Loop(ctx context.Context) {
	runLoop(ctx, msr.key(), msr.config, func(ctx context.Context) (time.Duration, error) {
		_, nextDelay, err := msr.collectCoordinatedScheduled(ctx)
		return nextDelay, err
	})
}

func (msr *MasteryStatisticsRepository) Collect() (*MasteryStatistics, error) {
	return msr.collectCoordinated(context.Background())
}

func (msr *MasteryStatisticsRepository) collectCoordinated(ctx context.Context) (*MasteryStatistics, error) {
	value, _, err := msr.collectCoordinatedScheduled(ctx)
	return value, err
}

func (msr *MasteryStatisticsRepository) collectCoordinatedScheduled(ctx context.Context) (*MasteryStatistics, time.Duration, error) {
	return collectCoordinated(
		ctx,
		msr.key(),
		msr.config,
		msr.collect,
		msr.setCache,
	)
}

func (msr *MasteryStatisticsRepository) collect(database db.Context) (*MasteryStatistics, error) {
	log.Infof("Collecting %s statistics...", msr.key())

	// collect data
	masteryMXDAOs, err := statistics_models.GetMasteryStatisticsMXDAOs(database)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	masteryTopRankersMXDAO, err := statistics_models.GetMasteryStatisticsTopRankersMXDAOs(database, 30)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	masteryMap := make(map[int]MasteryStatisticsItem)
	for _, masteryMXDAO := range masteryMXDAOs {
		if _, exists := masteryMap[masteryMXDAO.ChampionId]; !exists {
			champion, exists := service.Champions[strconv.Itoa(masteryMXDAO.ChampionId)]
			if !exists {
				log.Errorf("champion not found: %d", masteryMXDAO.ChampionId)
				return nil, fmt.Errorf("champion not found: %d", masteryMXDAO.ChampionId)
			}

			masteryMap[masteryMXDAO.ChampionId] = MasteryStatisticsItem{
				ChampionId:    masteryMXDAO.ChampionId,
				ChampionName:  champion.Name,
				AvgMastery:    masteryMXDAO.AvgMastery,
				MaxMastery:    int64(masteryMXDAO.MaxMastery),
				TotalMastery:  int64(masteryMXDAO.TotalMastery),
				MasteredCount: masteryMXDAO.MasteredCount,
				Summoners:     masteryMXDAO.Count,
				Rankers:       make([]MasteryStatisticsTopSummoners, 0),
			}
		}
	}

	for _, masteryTopRankerMXDAO := range masteryTopRankersMXDAO {
		mastery, exists := masteryMap[masteryTopRankerMXDAO.ChampionId]
		if !exists {
			log.Errorf("mastery not found: %d", masteryTopRankerMXDAO.ChampionId)
			return nil, fmt.Errorf("mastery not found: %d", masteryTopRankerMXDAO.ChampionId)
		}
		mastery.Rankers = append(mastery.Rankers, MasteryStatisticsTopSummoners{
			Puuid:          masteryTopRankerMXDAO.Puuid,
			ProfileIconId:  masteryTopRankerMXDAO.ProfileIconId,
			GameName:       masteryTopRankerMXDAO.GameName,
			TagLine:        masteryTopRankerMXDAO.TagLine,
			ChampionPoints: masteryTopRankerMXDAO.ChampionPoints,
			Ranks:          masteryTopRankerMXDAO.Ranks,
		})
		masteryMap[masteryTopRankerMXDAO.ChampionId] = mastery
	}

	collected := &MasteryStatistics{
		UpdatedAt:     time.Now(),
		MasteryGroups: make([]MasteryStatisticsItem, 0),
	}
	for _, mastery := range masteryMap {
		collected.MasteryGroups = append(collected.MasteryGroups, mastery)
	}

	log.Infof("%s statistics collected successfully", msr.key())
	return collected, nil
}

func (msr *MasteryStatisticsRepository) Save() error {
	return saveCurrentSnapshot(msr.key(), msr.getCache())
}

func (msr *MasteryStatisticsRepository) Load() (*MasteryStatistics, error) {
	if cached := msr.getCache(); cached != nil {
		return cached, nil
	}
	cached, err := loadSnapshot[MasteryStatistics](msr.key())
	if err != nil {
		return nil, err
	}
	if cached != nil {
		msr.setCache(cached)
	}
	return cached, nil
}

func (msr *MasteryStatisticsRepository) getCache() *MasteryStatistics {
	msr.mu.RLock()
	defer msr.mu.RUnlock()
	return msr.Cache
}

func (msr *MasteryStatisticsRepository) setCache(cache *MasteryStatistics) {
	msr.mu.Lock()
	defer msr.mu.Unlock()
	msr.Cache = cache
}
