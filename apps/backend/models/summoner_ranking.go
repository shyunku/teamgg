package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
	"time"
)

type SummonerRankingDAO struct {
	Puuid       string    `db:"puuid" json:"puuid"`
	Ranking     int       `db:"ranking" json:"ranking"`
	RatingPoint int       `db:"rating_point" json:"ratingPoint"`
	Total       int       `db:"total" json:"total"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

func (s *SummonerRankingDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO summoner_rankings
		    (puuid, ranking, rating_point, total, updated_at) 
		VALUE
		    (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		    ranking = VALUES(ranking),
		    rating_point = VALUES(rating_point),
		    total = VALUES(total),
		    updated_at = VALUES(updated_at)`,
		s.Puuid, s.Ranking, s.RatingPoint, s.Total, s.UpdatedAt,
	); err != nil {
		return err
	}
	return nil
}

func GetSummonerRankingDAO(db db.Context, puuid string) (*SummonerRankingDAO, bool, error) {
	var rankingEntity SummonerRankingDAO
	if err := db.Get(&rankingEntity, "SELECT * FROM summoner_rankings WHERE puuid = ?", puuid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &rankingEntity, true, nil
}
