package models

import (
	"team.gg-server/libs/db"
)

type MatchParticipantPerkDAO struct {
	MatchParticipantId string `db:"match_participant_id" json:"matchParticipantId"`

	StatPerkDefense int `db:"stat_perk_defense" json:"statPerkDefense"`
	StatPerkFlex    int `db:"stat_perk_flex" json:"statPerkFlex"`
	StatPerkOffense int `db:"stat_perk_offense" json:"statPerkOffense"`
}

func (m *MatchParticipantPerkDAO) InsertTx(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO match_participant_perks
		    (match_participant_id, stat_perk_defense, stat_perk_flex, stat_perk_offense) 
		VALUES (?, ?, ?, ?)`,
		m.MatchParticipantId, m.StatPerkDefense, m.StatPerkFlex, m.StatPerkOffense,
	); err != nil {
		return err
	}
	return nil
}
