package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type CustomGameParticipantDAO struct {
	CustomGameConfigId string `db:"custom_game_config_id" json:"customGameConfigId"`
	Puuid              string `db:"puuid" json:"puuid"`
	Team               int    `db:"team" json:"team"`
	Position           string `db:"position" json:"position"`
}

func (s *CustomGameParticipantDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO custom_game_participants
		    (custom_game_config_id, puuid, team, position) 
		VALUES (?, ?, ?, ?) 
		ON DUPLICATE KEY UPDATE 
			custom_game_config_id = ?, puuid = ?, team = ?, position = ?`,
		s.CustomGameConfigId, s.Puuid, s.Team, s.Position,
		s.CustomGameConfigId, s.Puuid, s.Team, s.Position,
	); err != nil {
		return err
	}
	return nil
}

func (s *CustomGameParticipantDAO) Delete(db db.Context) error {
	if _, err := db.Exec(`
		DELETE FROM custom_game_participants 
		WHERE custom_game_config_id = ? AND puuid = ?`, s.CustomGameConfigId, s.Puuid); err != nil {
		return err
	}
	return nil
}

func GetCustomGameParticipantDAOs_byCustomGameConfigId(db db.Context, customGameConfigId string) ([]*CustomGameParticipantDAO, error) {
	var customGameParticipants []*CustomGameParticipantDAO
	if err := db.Select(&customGameParticipants, "SELECT * FROM custom_game_participants WHERE custom_game_config_id = ?", customGameConfigId); err != nil {
		return nil, err
	}
	return customGameParticipants, nil
}

func GetCustomGameParticipantDAO_byPosition(db db.Context, customGameConfigId string, team int, position string) (*CustomGameParticipantDAO, bool, error) {
	var customGameParticipant CustomGameParticipantDAO
	if err := db.Get(&customGameParticipant, `
		SELECT * FROM custom_game_participants 
		WHERE custom_game_config_id = ? AND team = ? AND position = ?`, customGameConfigId, team, position); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &customGameParticipant, true, nil
}

func GetCustomGameParticipantDAO_byPuuid(db db.Context, customGameConfigId, puuid string) (*CustomGameParticipantDAO, bool, error) {
	var customGameParticipant CustomGameParticipantDAO
	if err := db.Get(&customGameParticipant, `
		SELECT * FROM custom_game_participants 
		WHERE custom_game_config_id = ? AND puuid = ?`, customGameConfigId, puuid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &customGameParticipant, true, nil
}

func DeleteCustomGameParticipantDAO_byPuuid(db db.Context, customGameConfigId, puuid string) error {
	if _, err := db.Exec(`
		DELETE FROM custom_game_participants 
		WHERE custom_game_config_id = ? AND puuid = ?`, customGameConfigId, puuid); err != nil {
		return err
	}
	return nil
}

func DeleteCustomGameParticipantDAOs_byId(db db.Context, customGameConfigId string) error {
	if _, err := db.Exec(`
		DELETE FROM custom_game_participants 
		WHERE custom_game_config_id = ?`, customGameConfigId); err != nil {
		return err
	}
	return nil
}
