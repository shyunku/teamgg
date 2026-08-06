package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type UserDAO struct {
	Uid               string `db:"uid" json:"uid"`
	UserId            string `db:"user_id" json:"userId"`
	EncryptedPassword string `db:"encrypted_pw" json:"encryptedPassword"`
}

func (u *UserDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO users
		    (uid, user_id, encrypted_pw) 
		VALUE
		    (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			user_id = VALUES(user_id),
			encrypted_pw = VALUES(encrypted_pw)
		`,
		u.Uid, u.UserId, u.EncryptedPassword,
	); err != nil {
		return err
	}
	return nil
}

func GetUserDAO_byUid(db db.Context, uid string) (*UserDAO, bool, error) {
	var user UserDAO
	if err := db.Get(&user, "SELECT * FROM users WHERE uid = ?", uid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &user, true, nil
}

func GetUserDAO_byUserId(db db.Context, userId string) (*UserDAO, bool, error) {
	var user UserDAO
	if err := db.Get(&user, "SELECT * FROM users WHERE user_id = ?", userId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &user, true, nil
}

func GetUserDAO_byUserId_withPassword(db db.Context, userId, encryptedPw string) (*UserDAO, bool, error) {
	var user UserDAO
	if err := db.Get(&user, "SELECT * FROM users WHERE user_id = ? AND encrypted_pw = ?", userId, encryptedPw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &user, true, nil
}
