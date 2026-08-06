package models

import (
	"team.gg-server/libs/db"
	"time"
)

type MasteryDAO struct {
	Puuid                        string    `db:"puuid" json:"puuid"`
	ChampionPointsUntilNextLevel int64     `db:"champion_points_until_next_level" json:"championPointsUntilNextLevel"`
	ChestGranted                 bool      `db:"chest_granted" json:"chestGranted"`
	ChampionId                   int64     `db:"champion_id" json:"championId"`
	LastPlayTime                 time.Time `db:"last_play_time" json:"lastPlayTime"`
	ChampionLevel                int       `db:"champion_level" json:"championLevel"`
	ChampionPoints               int       `db:"champion_points" json:"championPoints"`
	ChampionPointsSinceLastLevel int64     `db:"champion_points_since_last_level" json:"championPointsSinceLastLevel"`
	TokensEarned                 int       `db:"tokens_earned" json:"tokensEarned"`
}

func (m *MasteryDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO masteries
		    (puuid, champion_points_until_next_level, chest_granted, champion_id, last_play_time, champion_level, champion_points, champion_points_since_last_level, tokens_earned) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) 
		ON DUPLICATE KEY UPDATE 
			puuid = ?, champion_points_until_next_level = ?, chest_granted = ?, champion_id = ?, last_play_time = ?, champion_level = ?, champion_points = ?, champion_points_since_last_level = ?, tokens_earned = ?`,
		m.Puuid, m.ChampionPointsUntilNextLevel, m.ChestGranted, m.ChampionId, m.LastPlayTime, m.ChampionLevel, m.ChampionPoints, m.ChampionPointsSinceLastLevel, m.TokensEarned, m.Puuid, m.ChampionPointsUntilNextLevel, m.ChestGranted, m.ChampionId, m.LastPlayTime, m.ChampionLevel, m.ChampionPoints, m.ChampionPointsSinceLastLevel, m.TokensEarned,
	); err != nil {
		return err
	}
	return nil
}

func GetMasteryDAOs(db db.Context, puuid string) ([]*MasteryDAO, error) {
	var masteries []*MasteryDAO
	if err := db.Select(&masteries, "SELECT * FROM masteries WHERE puuid = ?", puuid); err != nil {
		return nil, err
	}
	return masteries, nil
}
