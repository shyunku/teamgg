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
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (provider, provider_subject),
			UNIQUE KEY user_identities_provider_uid_uindex (provider, uid),
			CONSTRAINT user_identities_users_uid_fk FOREIGN KEY (uid) REFERENCES users (uid)
				ON UPDATE CASCADE ON DELETE CASCADE
		)
	`)
	return err
}

func (u *UserIdentityDAO) Upsert(database db.Context) error {
	_, err := database.Exec(`
		INSERT INTO user_identities
			(provider, provider_subject, uid, display_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			display_name = VALUES(display_name),
			updated_at = VALUES(updated_at)
	`, u.Provider, u.ProviderSubject, u.Uid, u.DisplayName, u.CreatedAt, u.UpdatedAt)
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
	`, provider, uid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &identity, true, nil
}

func DeleteUserIdentityDAO(database db.Context, provider, uid string) error {
	_, err := database.Exec(`DELETE FROM user_identities WHERE provider = ? AND uid = ?`, provider, uid)
	return err
}
