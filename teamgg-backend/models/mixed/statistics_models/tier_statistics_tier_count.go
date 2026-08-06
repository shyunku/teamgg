package statistics_models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type TierStatisticsTierCountMXDAO struct {
	QueueType  string `db:"queue_type" json:"queueType"`
	Tier       string `db:"tier" json:"tier"`
	LeagueRank string `db:"league_rank" json:"leagueRank"`
	Count      int    `db:"count" json:"count"`
}

func GetTierStatisticsTierCountMXDAOs(db db.Context) ([]*TierStatisticsTierCountMXDAO, error) {
	var tierCounts []*TierStatisticsTierCountMXDAO
	if err := db.Select(&tierCounts, `
		SELECT l.queue_type, l.tier, l.league_rank, COUNT(*) AS count
		FROM leagues l
		LEFT JOIN summoners s ON l.puuid = s.puuid
		WHERE l.queue_type = 'RANKED_SOLO_5x5' OR l.queue_type = 'RANKED_FLEX_SR'
		GROUP BY l.queue_type, l.tier, l.league_rank;
	`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]*TierStatisticsTierCountMXDAO, 0), nil
		}
		return nil, err
	}
	return tierCounts, nil
}
