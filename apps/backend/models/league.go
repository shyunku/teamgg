package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
	"time"
)

type LeagueDAO struct {
	Puuid        string     `db:"puuid" json:"puuid"`
	LeagueId     string     `db:"league_id" json:"leagueId"`
	QueueType    string     `db:"queue_type" json:"queueType"`
	UpdatedAt    *time.Time `db:"updated_at" json:"updatedAt"`
	Tier         string     `db:"tier" json:"tier"`
	Rank         string     `db:"league_rank" json:"rank"`
	LeaguePoints int        `db:"league_points" json:"leaguePoints"`
	Wins         int        `db:"wins" json:"wins"`
	Losses       int        `db:"losses" json:"losses"`
	HotStreak    bool       `db:"hot_streak" json:"hotStreak"`
	Veteran      bool       `db:"veteran" json:"veteran"`
	FreshBlood   bool       `db:"fresh_blood" json:"freshBlood"`
	Inactive     bool       `db:"inactive" json:"inactive"`
	MsTarget     int        `db:"ms_target" json:"msTarget"`
	MsWins       int        `db:"ms_wins" json:"msWins"`
	MsLosses     int        `db:"ms_losses" json:"msLosses"`
	MsProgress   string     `db:"ms_progress" json:"msProgress"`
}

func (l *LeagueDAO) Upsert(db db.Context) error {
	if _, err := db.Exec(`
		INSERT INTO leagues
		    (puuid, league_id, queue_type, updated_at, tier, league_rank, league_points, wins, losses, hot_streak, veteran, fresh_blood, inactive, ms_target, ms_wins, ms_losses, ms_progress) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		    			league_id = ?, queue_type = ?, updated_at = ?, tier = ?, league_rank = ?, league_points = ?, wins = ?, losses = ?, hot_streak = ?, veteran = ?, fresh_blood = ?, inactive = ?, ms_target = ?, ms_wins = ?, ms_losses = ?, ms_progress = ?`,
		l.Puuid, l.LeagueId, l.QueueType, l.UpdatedAt, l.Tier, l.Rank, l.LeaguePoints, l.Wins, l.Losses, l.HotStreak, l.Veteran, l.FreshBlood, l.Inactive, l.MsTarget, l.MsWins, l.MsLosses, l.MsProgress,
		l.LeagueId, l.QueueType, l.UpdatedAt, l.Tier, l.Rank, l.LeaguePoints, l.Wins, l.Losses, l.HotStreak, l.Veteran, l.FreshBlood, l.Inactive, l.MsTarget, l.MsWins, l.MsLosses, l.MsProgress,
	); err != nil {
		return err
	}
	return nil
}

func GetLeagueDAO(db db.Context, puuid string, queueType string) (*LeagueDAO, bool, error) {
	// check if league exists in db
	var leagueEntity LeagueDAO
	if err := db.Get(&leagueEntity, `
		SELECT *
		FROM leagues
		WHERE puuid = ? AND queue_type = ? AND updated_at IS NOT NULL
		ORDER BY updated_at DESC`, puuid, queueType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &leagueEntity, true, nil
}
