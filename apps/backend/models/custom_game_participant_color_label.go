package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type CustomGameParticipantColorLabelDAO struct {
	CustomGameConfigId string `db:"custom_game_config_id" json:"customGameConfigId"`
	Puuid              string `db:"puuid" json:"puuid"`
	ColorCode          int    `db:"color_code" json:"colorCode"`
}

func (c *CustomGameParticipantColorLabelDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
	INSERT INTO custom_game_participant_color_labels (
		custom_game_config_id, puuid, color_code
	) VALUES (
		?, ?, ?
	) ON DUPLICATE KEY UPDATE
	    custom_game_config_id = ?,
		puuid = ?,
		color_code = ?
	`, c.CustomGameConfigId, c.Puuid, c.ColorCode,
		c.CustomGameConfigId, c.Puuid, c.ColorCode,
	); err != nil {
		return err
	}
	return nil
}

func GetCustomGameParticipantColorLabelDAOs_byCustomGameConfigId(db db.Context, customGameConfigId string) ([]CustomGameParticipantColorLabelDAO, error) {
	var customGameParticipantColorLabelsDAO []CustomGameParticipantColorLabelDAO
	if err := db.Select(&customGameParticipantColorLabelsDAO, `
		SELECT * FROM custom_game_participant_color_labels WHERE custom_game_config_id = ?
	`, customGameConfigId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]CustomGameParticipantColorLabelDAO, 0), nil
		}
		return nil, err
	}
	return customGameParticipantColorLabelsDAO, nil
}

func GetCustomGameParticipantColorLabelDAO_byPuuid(db db.Context, customGameConfigId, puuid string) (*CustomGameParticipantColorLabelDAO, bool, error) {
	var customGameParticipantColorLabelDAO CustomGameParticipantColorLabelDAO
	if err := db.Get(&customGameParticipantColorLabelDAO, `
		SELECT * FROM custom_game_participant_color_labels WHERE custom_game_config_id = ? AND puuid = ?
	`, customGameConfigId, puuid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &customGameParticipantColorLabelDAO, true, nil
}

func DeleteCustomGameParticipantColorLabelDAO_byPuuid(db db.Context, customGameConfigId string, puuid string) error {
	if _, err := db.Exec(`
		DELETE FROM custom_game_participant_color_labels WHERE custom_game_config_id = ? AND puuid = ?
	`, customGameConfigId, puuid); err != nil {
		return err
	}
	return nil
}

func DeleteCustomGameParticipantColorLabels_byCustomGameConfigId(db db.Context, customGameConfigId string) error {
	if _, err := db.Exec(`
		DELETE FROM custom_game_participant_color_labels WHERE custom_game_config_id = ?
	`, customGameConfigId); err != nil {
		return err
	}
	return nil
}
