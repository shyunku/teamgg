package statistics_models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type TierStatisticsTopRankersMXDAO struct {
	QueueType  string `db:"queue_type" json:"queueType"`
	Tier       string `db:"tier" json:"tier"`
	LeagueRank string `db:"league_rank" json:"leagueRank"`

	Puuid         string `db:"puuid" json:"puuid"`
	ProfileIconId int    `db:"profile_icon_id" json:"profileIconId"`
	GameName      string `db:"game_name" json:"gameName"`
	TagLine       string `db:"tag_line" json:"tagLine"`

	LeaguePoints int `db:"league_points" json:"leaguePoints"`
	Wins         int `db:"wins" json:"wins"`
	Losses       int `db:"losses" json:"losses"`
	Ranks        int `db:"ranks" json:"ranks"`
}

func GetTierStatisticsTopRankersMXDAOs(db db.Context, topRanks int) ([]*TierStatisticsTopRankersMXDAO, error) {
	var topRankers []*TierStatisticsTopRankersMXDAO
	if err := db.Select(&topRankers, `
		WITH RankedLeagues AS (
			SELECT
				l.queue_type,
				l.tier,
				l.league_rank,
				l.puuid,
				l.league_points,
				l.wins,
				l.losses,
				ROW_NUMBER() OVER (
					PARTITION BY l.queue_type, l.tier, l.league_rank
					ORDER BY l.league_points DESC, l.wins DESC, l.losses ASC
				) AS ranks
			FROM leagues l
			WHERE l.queue_type = 'RANKED_SOLO_5x5' OR l.queue_type = 'RANKED_FLEX_SR'
		)
		SELECT
			rl.queue_type,
			rl.tier,
			rl.league_rank,
			s.puuid,
			s.profile_icon_id,
			s.game_name,
			s.tag_line,
			rl.league_points,
			rl.wins,
			rl.losses,
			rl.ranks
		FROM RankedLeagues rl
		LEFT JOIN summoners s ON rl.puuid = s.puuid
		WHERE rl.ranks <= ?
		ORDER BY rl.queue_type, rl.tier, rl.league_rank, rl.ranks ASC;
	`, topRanks); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]*TierStatisticsTopRankersMXDAO, 0), nil
		}
		return nil, err
	}
	return topRankers, nil
}
