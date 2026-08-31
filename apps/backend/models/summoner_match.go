package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type SummonerMatchDAO struct {
	Puuid      string        `db:"puuid" json:"puuid"`
	MatchId    string        `db:"match_id" json:"matchId"`
	SummonerFk sql.NullInt64 `db:"summoner_fk" json:"-"`
	MatchFk    sql.NullInt64 `db:"match_fk" json:"-"`
}

func (s *SummonerMatchDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO summoner_matches
		    (puuid, match_id)
		SELECT ?, ?
		WHERE NOT EXISTS (
			SELECT 1 FROM summoner_matches WHERE puuid = ? AND match_id = ?
		)
		ON DUPLICATE KEY UPDATE 
			puuid = VALUES(puuid), match_id = VALUES(match_id)`,
		s.Puuid, s.MatchId, s.Puuid, s.MatchId,
	); err != nil {
		return err
	}
	return nil
}

func GetSummonerMatchDAO(db db.Context, puuid string, matchId string) (*SummonerMatchDAO, bool, error) {
	// check if summoner exists in db
	var summonerMatch SummonerMatchDAO
	if err := db.Get(&summonerMatch, "SELECT * FROM summoner_matches WHERE puuid = ? AND match_id = ?", puuid, matchId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &summonerMatch, true, nil
}
