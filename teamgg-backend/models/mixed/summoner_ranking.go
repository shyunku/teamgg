package mixed

import (
	"database/sql"
	"errors"
	"team.gg-server/core"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/types"
	"team.gg-server/util"
	"time"
)

type SummonerRankingMXDAO struct {
	Puuid        string `db:"puuid" json:"puuid"`
	RatingPoints int    `db:"rating_points" json:"ratingPoints"`
	Ranking      int    `db:"ranking" json:"ranking"`
	Total        int    `db:"total" json:"total"`
}

func GetSummonerSoloRankingMXDAO(db db.Context, puuid string) (*SummonerRankingMXDAO, error) {
	if core.DebugOnProd {
		defer util.InspectFunctionExecutionTime()()
	}

	var rankingMXDAO SummonerRankingMXDAO
	summonerRankingDAO, found, err := models.GetSummonerRankingDAO(db, puuid)
	if err != nil {
		return nil, err
	}

	needUpdate := !found
	if found {
		if time.Now().Sub(summonerRankingDAO.UpdatedAt) > types.SummonerRankingRevisionPeriod {
			needUpdate = true
		}
	}

	if needUpdate {
		// TODO :: optimize query (too long latency > 10s)
		if err := db.Get(&rankingMXDAO, `
			WITH filtered_leagues AS (
				SELECT *
				FROM leagues
				WHERE queue_type = ?
			),
			rank_data AS (
				SELECT
					s.puuid,
					IF(ISNULL(fl.league_rank), 0, (str.score + fl.league_points)) as rating_points,
					ROW_NUMBER() OVER (ORDER BY IF(ISNULL(fl.league_rank), 0, (str.score + fl.league_points)) DESC) as ranking
				FROM
					summoners s
				LEFT JOIN
					filtered_leagues fl ON s.puuid = fl.puuid
				LEFT JOIN
					static_tier_ranks str ON fl.tier = str.tier_label AND fl.league_rank = str.rank_label
			),
			total_rankers AS (
				SELECT COUNT(*) as total FROM rank_data
			)
			SELECT rank_data.*, total_rankers.total
			FROM rank_data, total_rankers
			WHERE rank_data.puuid = ?;
		`, types.RankTypeSolo, puuid); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}

		// upsert ranking
		summonerRankingDAO = &models.SummonerRankingDAO{
			Puuid:       puuid,
			RatingPoint: rankingMXDAO.RatingPoints,
			Ranking:     rankingMXDAO.Ranking,
			Total:       rankingMXDAO.Total,
			UpdatedAt:   time.Now(),
		}
		if err := summonerRankingDAO.Upsert(db); err != nil {
			return nil, err
		}
	} else {
		rankingMXDAO = SummonerRankingMXDAO{
			Puuid:        summonerRankingDAO.Puuid,
			RatingPoints: summonerRankingDAO.RatingPoint,
			Ranking:      summonerRankingDAO.Ranking,
			Total:        summonerRankingDAO.Total,
		}
	}

	return &rankingMXDAO, nil
}
