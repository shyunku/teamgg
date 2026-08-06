package models

import (
	"team.gg-server/libs/db"
)

type MatchTeamDAO struct {
	MatchId string `db:"match_id" json:"matchId"`
	TeamId  int    `db:"team_id" json:"teamId"`
	Win     bool   `db:"win" json:"win"`

	BaronFirst      bool `db:"baron_first" json:"baronFirst"`
	BaronKills      int  `db:"baron_kills" json:"baronKills"`
	ChampionFirst   bool `db:"champion_first" json:"championFirst"`
	ChampionKills   int  `db:"champion_kills" json:"championKills"`
	DragonFirst     bool `db:"dragon_first" json:"dragonFirst"`
	DragonKills     int  `db:"dragon_kills" json:"dragonKills"`
	InhibitorFirst  bool `db:"inhibitor_first" json:"inhibitorFirst"`
	InhibitorKills  int  `db:"inhibitor_kills" json:"inhibitorKills"`
	RiftHeraldFirst bool `db:"rift_herald_first" json:"riftHeraldFirst"`
	RiftHeraldKills int  `db:"rift_herald_kills" json:"riftHeraldKills"`
	TowerFirst      bool `db:"tower_first" json:"towerFirst"`
	TowerKills      int  `db:"tower_kills" json:"towerKills"`
}

func (s *MatchTeamDAO) Insert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO match_teams
		    (match_id, team_id, win, baron_first, baron_kills, champion_first, 
		     champion_kills, dragon_first, dragon_kills, inhibitor_first, 
		     inhibitor_kills, rift_herald_first, rift_herald_kills, 
		     tower_first, tower_kills) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.MatchId, s.TeamId, s.Win, s.BaronFirst, s.BaronKills, s.ChampionFirst,
		s.ChampionKills, s.DragonFirst, s.DragonKills, s.InhibitorFirst,
		s.InhibitorKills, s.RiftHeraldFirst, s.RiftHeraldKills,
		s.TowerFirst, s.TowerKills,
	); err != nil {
		return err
	}
	return nil
}
