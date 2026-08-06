package models

import (
	"team.gg-server/libs/db"
)

type MatchTeamBanDAO struct {
	MatchId string `db:"match_id" json:"matchId"`
	TeamId  int    `db:"team_id" json:"teamId"`

	ChampionId int `db:"champion_id" json:"championId"`
	PickTurn   int `db:"pick_turn" json:"pickTurn"`
}

func (s *MatchTeamBanDAO) Insert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO match_team_bans
		    (match_id, team_id, champion_id, pick_turn) 
		VALUES (?, ?, ?, ?)`,
		s.MatchId, s.TeamId, s.ChampionId, s.PickTurn,
	); err != nil {
		return err
	}
	return nil
}
