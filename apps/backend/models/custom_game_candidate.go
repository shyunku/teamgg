package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type CustomGameCandidateDAO struct {
	CustomGameConfigId string  `db:"custom_game_config_id" json:"customGameConfigId"`
	Puuid              string  `db:"puuid" json:"puuid"`
	CustomTier         *string `db:"custom_tier" json:"customTier"`
	CustomRank         *string `db:"custom_rank" json:"customRank"`
	FlavorTop          int     `db:"flavor_top" json:"flavorTop"`
	FlavorJungle       int     `db:"flavor_jungle" json:"flavorJungle"`
	FlavorMid          int     `db:"flavor_mid" json:"flavorMid"`
	FlavorAdc          int     `db:"flavor_adc" json:"flavorAdc"`
	FlavorSupport      int     `db:"flavor_support" json:"flavorSupport"`
	MasteryTop         int     `db:"mastery_top" json:"masteryTop"`
	MasteryJungle      int     `db:"mastery_jungle" json:"masteryJungle"`
	MasteryMid         int     `db:"mastery_mid" json:"masteryMid"`
	MasteryAdc         int     `db:"mastery_adc" json:"masteryAdc"`
	MasterySupport     int     `db:"mastery_support" json:"masterySupport"`
}

func (c *CustomGameCandidateDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
	INSERT INTO custom_game_candidates (
		custom_game_config_id, puuid, custom_tier, custom_rank, 
		flavor_top, flavor_jungle, flavor_mid, flavor_adc, flavor_support,
		mastery_top, mastery_jungle, mastery_mid, mastery_adc, mastery_support
	) VALUES (
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	) ON DUPLICATE KEY UPDATE
	    custom_game_config_id = ?,
		puuid = ?,
		custom_tier = ?,
		custom_rank = ?,
		flavor_top = ?,
		flavor_jungle = ?,
		flavor_mid = ?,
		flavor_adc = ?,
		flavor_support = ?,
		mastery_top = ?,
		mastery_jungle = ?,
		mastery_mid = ?,
		mastery_adc = ?,
		mastery_support = ?`,
		c.CustomGameConfigId, c.Puuid, c.CustomTier, c.CustomRank,
		c.FlavorTop, c.FlavorJungle, c.FlavorMid, c.FlavorAdc, c.FlavorSupport,
		c.MasteryTop, c.MasteryJungle, c.MasteryMid, c.MasteryAdc, c.MasterySupport,
		c.CustomGameConfigId, c.Puuid, c.CustomTier, c.CustomRank,
		c.FlavorTop, c.FlavorJungle, c.FlavorMid, c.FlavorAdc, c.FlavorSupport,
		c.MasteryTop, c.MasteryJungle, c.MasteryMid, c.MasteryAdc, c.MasterySupport,
	); err != nil {
		return err
	}
	return nil
}

func GetCustomGameCandidateDAOs_byCustomGameConfigId(db db.Context, customGameConfigId string) ([]CustomGameCandidateDAO, error) {
	var customGameCandidatesDAO []CustomGameCandidateDAO
	if err := db.Select(&customGameCandidatesDAO, `
		SELECT * FROM custom_game_candidates WHERE custom_game_config_id = ?
	`, customGameConfigId); err != nil {
		return nil, err
	}
	return customGameCandidatesDAO, nil
}

func GetCustomGameCandidateDAO_byPuuid(db db.Context, customGameConfigId, puuid string) (*CustomGameCandidateDAO, bool, error) {
	var customGameCandidateDAO CustomGameCandidateDAO
	if err := db.Get(&customGameCandidateDAO, `
		SELECT * FROM custom_game_candidates WHERE custom_game_config_id = ? AND puuid = ?
	`, customGameConfigId, puuid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &customGameCandidateDAO, true, nil
}

func GetCustomGameCandidateDAOs_byPuuid(db db.Context, puuid string) ([]CustomGameCandidateDAO, error) {
	var customGameCandidatesDAO []CustomGameCandidateDAO
	if err := db.Select(&customGameCandidatesDAO, `
		SELECT * FROM custom_game_candidates WHERE puuid = ?
	`, puuid); err != nil {
		return nil, err
	}
	return customGameCandidatesDAO, nil
}

func DeleteCustomGameCandidateDAO_byPuuid(db db.Context, customGameConfigId, puuid string) error {
	if _, err := db.Exec(`
		DELETE FROM custom_game_candidates WHERE custom_game_config_id = ? AND puuid = ?
	`, customGameConfigId, puuid); err != nil {
		return err
	}
	return nil
}
