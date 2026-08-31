package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
	"team.gg-server/util"
	"time"
)

type SummonerDAO struct {
	ProfileIconId   int       `db:"profile_icon_id" json:"profileIconId"`
	RevisionDate    int64     `db:"revision_date" json:"revisionDate"`
	GameName        string    `db:"game_name" json:"gameName"`
	TagLine         string    `db:"tag_line" json:"tagLine"`
	Puuid           string    `db:"puuid" json:"puuid"`
	SummonerLevel   int64     `db:"summoner_level" json:"summonerLevel"`
	ShortenGameName string    `db:"shorten_game_name" json:"shortenGameName"`
	LastUpdatedAt   time.Time `db:"last_updated_at" json:"lastUpdatedAt"`

	SummonerPk sql.NullInt64 `db:"summoner_pk" json:"-"`
}

func (s *SummonerDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO summoners
		    (profile_icon_id, revision_date, game_name, tag_line, puuid, summoner_level, shorten_game_name, last_updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) 
		ON DUPLICATE KEY UPDATE 
			profile_icon_id = ?, revision_date = ?, 
		    game_name = ?, tag_line = ?,
		    puuid = ?, summoner_level = ?, 
		    shorten_game_name = ?, last_updated_at = ?`,
		s.ProfileIconId, s.RevisionDate,
		s.GameName, s.TagLine, s.Puuid,
		s.SummonerLevel, s.ShortenGameName, s.LastUpdatedAt,
		s.ProfileIconId, s.RevisionDate,
		s.GameName, s.TagLine, s.Puuid,
		s.SummonerLevel, s.ShortenGameName, s.LastUpdatedAt,
	); err != nil {
		return err
	}
	return nil
}

func GetSummonerDAO_byNameTag(db db.Context, gameName string, tagLine string) (*SummonerDAO, bool, error) {
	shortenName := util.ShortenSummonerName(gameName)
	// check if summoner exists in db
	var summonerEntity SummonerDAO
	if err := db.Get(&summonerEntity,
		"SELECT * FROM summoners WHERE shorten_game_name = ? AND tag_line = ?",
		shortenName, tagLine); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &summonerEntity, true, nil
}

func GetSummonerDAO_byPuuid(db db.Context, puuid string) (*SummonerDAO, bool, error) {
	// check if summoner exists in db
	var summonerEntity SummonerDAO
	if err := db.Get(&summonerEntity, "SELECT * FROM summoners WHERE puuid = ?", puuid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &summonerEntity, true, nil
}

func FindSummonerDAO_byKeyword(db db.Context, keyword string, count int) ([]*SummonerDAO, error) {
	shortenKeyword := util.ShortenSummonerName(keyword)
	shortenQuery := "%" + shortenKeyword + "%"
	keywordQuery := "%" + keyword + "%"
	var summoners []*SummonerDAO
	if err := db.Select(&summoners, `
		SELECT * FROM summoners 
			 WHERE shorten_game_name LIKE ? 
			    OR game_name LIKE ?
				OR tag_line LIKE ? 
		LIMIT ?`,
		shortenQuery,
		keywordQuery, keywordQuery, count); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]*SummonerDAO, 0), nil
		}
		return nil, err
	}
	return summoners, nil
}
