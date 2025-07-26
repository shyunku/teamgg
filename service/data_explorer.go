package service

import (
	"context"
	log "github.com/shyunku-libraries/go-logger"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/third_party/riot/api"
	"team.gg-server/types"
	"time"
)

type DataExplorer struct {
	lastExploredTime *time.Time
	exploreCaches    int
	cacheHit         int
}

func NewDataExplorer() *DataExplorer {
	return &DataExplorer{
		lastExploredTime: nil,
		exploreCaches:    0,
	}
}

func (de *DataExplorer) Loop() {
	loopInterval := GetDataExplorerLoopPeriod()
	for {
		if de.Explore() {
			time.Sleep(loopInterval)
		}
	}
}

func (de *DataExplorer) finalizeExploration(meaningful, success bool) {
	now := time.Now()
	de.lastExploredTime = &now
	if meaningful {
		de.exploreCaches++
		if success {
			de.cacheHit++
		}
	}
	if de.exploreCaches%30 == 0 {
		log.Debugf("DataExplorer: explored %d/%d", de.cacheHit, de.exploreCaches)
	}
}

func (de *DataExplorer) Explore() bool {
	var err error
	var meaningful bool
	// update something new
	meaningful, err = de.fetchNewSummoner()

	de.finalizeExploration(meaningful, err == nil)
	return meaningful
}

func (de *DataExplorer) GetExploreCaches() int {
	return de.exploreCaches
}

func (de *DataExplorer) fetchNewSummoner() (bool, error) {
	ctx := context.Background()
	tx, err := db.Root.BeginTxx(ctx, nil)
	if err != nil {
		log.Error(err)
		return false, err
	}

	// get random summoner match
	participant, exists, err := models.GetRandomMatchParticipantDAO(tx)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return false, err
	}
	if !exists {
		_ = tx.Rollback()
		return true, nil
	}

	// search for original summoner
	_, exists, err = models.GetSummonerDAO_byPuuid(tx, participant.Puuid)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return false, err
	}
	if exists {
		_ = tx.Rollback()
		return true, nil
	}

	// get summoner info
	summonerDAO, _, err := RenewSummonerInfoByPuuid(tx, participant.Puuid)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return true, err
	}

	// get summoner rank
	if err := RenewSummonerLeague(tx, summonerDAO.Puuid); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return true, err
	}

	// get summoner recent matches
	if err := RenewSummonerMatches(tx, summonerDAO.Puuid, &api.MatchIdsReqOption{
		QueueId: types.QueueTypeAll,
		Count:   types.DataExplorerLoadMatchesCount,
	}); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return true, err
	}

	// get summoner mastery
	if err := RenewSummonerMastery(tx, summonerDAO.Puuid); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return true, err
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return true, err
	}

	//log.Debugf("DataExplorer: fetched new summoner %s#%s", summonerDAO.GameName, summonerDAO.TagLine)
	return true, nil
}
