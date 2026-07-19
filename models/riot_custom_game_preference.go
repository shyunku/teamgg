package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
	"time"
)

type RiotCustomGamePreferenceDAO struct {
	Puuid         string    `db:"puuid" json:"puuid"`
	FlavorTop     int       `db:"flavor_top" json:"flavorTop"`
	FlavorJungle  int       `db:"flavor_jungle" json:"flavorJungle"`
	FlavorMid     int       `db:"flavor_mid" json:"flavorMid"`
	FlavorAdc     int       `db:"flavor_adc" json:"flavorAdc"`
	FlavorSupport int       `db:"flavor_support" json:"flavorSupport"`
	UpdatedAt     time.Time `db:"updated_at" json:"updatedAt"`
}

func EnsureRiotCustomGamePreferenceTable(database db.Context) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS riot_custom_game_preferences (
			puuid VARCHAR(255) NOT NULL PRIMARY KEY,
			flavor_top INT NOT NULL DEFAULT 0,
			flavor_jungle INT NOT NULL DEFAULT 0,
			flavor_mid INT NOT NULL DEFAULT 0,
			flavor_adc INT NOT NULL DEFAULT 0,
			flavor_support INT NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL,
			CONSTRAINT riot_custom_game_preferences_summoners_puuid_fk
				FOREIGN KEY (puuid) REFERENCES summoners (puuid)
				ON UPDATE CASCADE ON DELETE CASCADE
		)
	`)
	return err
}

func (p *RiotCustomGamePreferenceDAO) Upsert(database db.Context) error {
	_, err := database.Exec(`
		INSERT INTO riot_custom_game_preferences
			(puuid, flavor_top, flavor_jungle, flavor_mid, flavor_adc, flavor_support, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			flavor_top = VALUES(flavor_top), flavor_jungle = VALUES(flavor_jungle),
			flavor_mid = VALUES(flavor_mid), flavor_adc = VALUES(flavor_adc),
			flavor_support = VALUES(flavor_support), updated_at = VALUES(updated_at)
	`, p.Puuid, p.FlavorTop, p.FlavorJungle, p.FlavorMid, p.FlavorAdc, p.FlavorSupport, p.UpdatedAt)
	return err
}

func GetRiotCustomGamePreferenceDAO_byPuuid(database db.Context, puuid string) (*RiotCustomGamePreferenceDAO, bool, error) {
	var preference RiotCustomGamePreferenceDAO
	if err := database.Get(&preference, `SELECT * FROM riot_custom_game_preferences WHERE puuid = ?`, puuid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &preference, true, nil
}
