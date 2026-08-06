package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type ThirdPartyIntegrationDAO struct {
	Puuid    string `db:"puuid" json:"puuid"`
	Platform string `db:"platform" json:"platform"`
	Token    string `db:"token" json:"token"`
}

func (t *ThirdPartyIntegrationDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO third_party_integrations
		    (puuid, platform, token)
		VALUE
		    (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			puuid = VALUES(puuid),
			platform = VALUES(platform),
			token = VALUES(token)
		`,
		t.Puuid, t.Platform, t.Token,
	); err != nil {
		return err
	}
	return nil
}

func GetThirdPartyIntegrationDAOs_byPuuid(db db.Context, puuid string) ([]ThirdPartyIntegrationDAO, bool, error) {
	var integrations []ThirdPartyIntegrationDAO
	if err := db.Select(&integrations, "SELECT * FROM third_party_integrations WHERE puuid = ?", puuid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return integrations, true, nil
}

func GetThirdPartyIntegrationDAOs_byPlatformAndToken(db db.Context, platform, token string) ([]ThirdPartyIntegrationDAO, error) {
	var integrations []ThirdPartyIntegrationDAO
	if err := db.Select(&integrations, "SELECT * FROM third_party_integrations WHERE platform = ? AND token = ?", platform, token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []ThirdPartyIntegrationDAO{}, nil
		}
		return nil, err
	}
	return integrations, nil
}
