package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
	"time"
)

const UserIdentityProviderRiot = "riot"

type UserIdentityDAO struct {
	Provider        string    `db:"provider" json:"provider"`
	ProviderSubject string    `db:"provider_subject" json:"providerSubject"`
	Uid             string    `db:"uid" json:"uid"`
	DisplayName     string    `db:"display_name" json:"displayName"`
	IsPrimary       bool      `db:"is_primary" json:"isPrimary"`
	CreatedAt       time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt       time.Time `db:"updated_at" json:"updatedAt"`
}

func EnsureUserIdentityTable(database db.Context) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS user_identities (
			provider VARCHAR(32) NOT NULL,
			provider_subject VARCHAR(255) NOT NULL,
			uid VARCHAR(255) NOT NULL,
			display_name VARCHAR(255) NOT NULL,
			is_primary TINYINT(1) NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (provider, provider_subject),
			KEY user_identities_provider_uid_index (provider, uid),
			CONSTRAINT user_identities_users_uid_fk FOREIGN KEY (uid) REFERENCES users (uid)
				ON UPDATE CASCADE ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	var columnCount int
	if err := database.Get(&columnCount, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = 'user_identities' AND column_name = 'is_primary'
	`); err != nil {
		return err
	}
	if columnCount == 0 {
		if _, err := database.Exec(`ALTER TABLE user_identities ADD COLUMN is_primary TINYINT(1) NOT NULL DEFAULT 0 AFTER display_name`); err != nil {
			return err
		}
		// Before this migration a user could only have one identity per provider.
		if _, err := database.Exec(`UPDATE user_identities SET is_primary = 1`); err != nil {
			return err
		}
	}

	var uniqueIndexCount int
	if err := database.Get(&uniqueIndexCount, `
		SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = 'user_identities'
		  AND index_name = 'user_identities_provider_uid_uindex' AND non_unique = 0
	`); err != nil {
		return err
	}
	if uniqueIndexCount > 0 {
		if _, err := database.Exec(`ALTER TABLE user_identities DROP INDEX user_identities_provider_uid_uindex`); err != nil {
			return err
		}
	}

	var indexCount int
	if err := database.Get(&indexCount, `
		SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = 'user_identities'
		  AND index_name = 'user_identities_provider_uid_index'
	`); err != nil {
		return err
	}
	if indexCount == 0 {
		_, err = database.Exec(`ALTER TABLE user_identities ADD INDEX user_identities_provider_uid_index (provider, uid)`)
	}
	return err
}

func (u *UserIdentityDAO) Upsert(database db.Context) error {
	_, err := database.Exec(`
		INSERT INTO user_identities
			(provider, provider_subject, uid, display_name, is_primary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			display_name = VALUES(display_name),
			is_primary = VALUES(is_primary),
			updated_at = VALUES(updated_at)
	`, u.Provider, u.ProviderSubject, u.Uid, u.DisplayName, u.IsPrimary, u.CreatedAt, u.UpdatedAt)
	return err
}

func GetUserIdentityDAO(database db.Context, provider, providerSubject string) (*UserIdentityDAO, bool, error) {
	var identity UserIdentityDAO
	if err := database.Get(&identity, `
		SELECT * FROM user_identities
		WHERE provider = ? AND provider_subject = ?
	`, provider, providerSubject); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &identity, true, nil
}

func GetUserIdentityDAO_byUid(database db.Context, provider, uid string) (*UserIdentityDAO, bool, error) {
	var identity UserIdentityDAO
	if err := database.Get(&identity, `
		SELECT * FROM user_identities
		WHERE provider = ? AND uid = ?
		ORDER BY is_primary DESC, created_at ASC
		LIMIT 1
	`, provider, uid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &identity, true, nil
}

func GetUserIdentityDAOs_byUid(database db.Context, provider, uid string) ([]UserIdentityDAO, error) {
	identities := make([]UserIdentityDAO, 0)
	if err := database.Select(&identities, `
		SELECT * FROM user_identities
		WHERE provider = ? AND uid = ?
		ORDER BY is_primary DESC, created_at ASC
	`, provider, uid); err != nil {
		return nil, err
	}
	return identities, nil
}

func SetPrimaryUserIdentityDAO(database db.Context, provider, uid, providerSubject string, updatedAt time.Time) error {
	_, err := database.Exec(`
		UPDATE user_identities
		SET is_primary = (provider_subject = ?),
			updated_at = IF(provider_subject = ?, ?, updated_at)
		WHERE provider = ? AND uid = ?
	`, providerSubject, providerSubject, updatedAt, provider, uid)
	return err
}

func CountUserIdentityDAOs_byUid(database db.Context, provider, uid string) (int, error) {
	var count int
	err := database.Get(&count, `SELECT COUNT(*) FROM user_identities WHERE provider = ? AND uid = ?`, provider, uid)
	return count, err
}

func DeleteUserIdentityDAO_bySubject(database db.Context, provider, uid, providerSubject string) error {
	_, err := database.Exec(`
		DELETE FROM user_identities WHERE provider = ? AND uid = ? AND provider_subject = ?
	`, provider, uid, providerSubject)
	return err
}

func DeleteUserIdentityDAO(database db.Context, provider, uid string) error {
	_, err := database.Exec(`DELETE FROM user_identities WHERE provider = ? AND uid = ?`, provider, uid)
	return err
}
