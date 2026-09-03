package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"team.gg-server/libs/db"
	"time"

	"github.com/jmoiron/sqlx"
)

type MasteryDAO struct {
	Puuid                        string        `db:"puuid" json:"puuid"`
	SummonerFk                   sql.NullInt64 `db:"summoner_fk" json:"-"`
	ChampionPointsUntilNextLevel int64         `db:"champion_points_until_next_level" json:"championPointsUntilNextLevel"`
	ChestGranted                 bool          `db:"chest_granted" json:"chestGranted"`
	ChampionId                   int64         `db:"champion_id" json:"championId"`
	LastPlayTime                 time.Time     `db:"last_play_time" json:"lastPlayTime"`
	ChampionLevel                int           `db:"champion_level" json:"championLevel"`
	ChampionPoints               int           `db:"champion_points" json:"championPoints"`
	ChampionPointsSinceLastLevel int64         `db:"champion_points_since_last_level" json:"championPointsSinceLastLevel"`
	TokensEarned                 int           `db:"tokens_earned" json:"tokensEarned"`
}

func (m *MasteryDAO) Upsert(db db.Context) error {
	if !m.SummonerFk.Valid {
		return errors.New("mastery numeric summoner key is required")
	}
	if MasteryNumericV2WritesEnabled() {
		if err := m.upsertNumeric(db); err != nil {
			return err
		}
		return m.upsertLegacy(db)
	}
	if err := m.upsertLegacy(db); err != nil {
		return err
	}
	return m.upsertNumeric(db)
}

func (m *MasteryDAO) upsertLegacy(database db.Context) error {
	_, err := database.Exec(`
		INSERT INTO masteries
		    (puuid, summoner_fk, champion_points_until_next_level, chest_granted, champion_id, last_play_time, champion_level, champion_points, champion_points_since_last_level, tokens_earned)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE 
			summoner_fk = VALUES(summoner_fk),
			champion_points_until_next_level = VALUES(champion_points_until_next_level),
			chest_granted = VALUES(chest_granted),
			last_play_time = VALUES(last_play_time),
			champion_level = VALUES(champion_level),
			champion_points = VALUES(champion_points),
			champion_points_since_last_level = VALUES(champion_points_since_last_level),
			tokens_earned = VALUES(tokens_earned)`,
		m.Puuid, m.SummonerFk, m.ChampionPointsUntilNextLevel, m.ChestGranted,
		m.ChampionId, m.LastPlayTime, m.ChampionLevel, m.ChampionPoints,
		m.ChampionPointsSinceLastLevel, m.TokensEarned,
	)
	return err
}

func (m *MasteryDAO) upsertNumeric(database db.Context) error {
	_, err := database.Exec(`
		INSERT INTO masteries_numeric_v2
		    (summoner_fk, champion_id, champion_points_until_next_level, chest_granted, last_play_time, champion_level, champion_points, champion_points_since_last_level, tokens_earned)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			champion_points_until_next_level = VALUES(champion_points_until_next_level),
			chest_granted = VALUES(chest_granted),
			last_play_time = VALUES(last_play_time),
			champion_level = VALUES(champion_level),
			champion_points = VALUES(champion_points),
			champion_points_since_last_level = VALUES(champion_points_since_last_level),
			tokens_earned = VALUES(tokens_earned)`,
		m.SummonerFk, m.ChampionId, m.ChampionPointsUntilNextLevel, m.ChestGranted,
		m.LastPlayTime, m.ChampionLevel, m.ChampionPoints,
		m.ChampionPointsSinceLastLevel, m.TokensEarned,
	)
	return err
}

type masteryTransactionStarter interface {
	BeginTxx(context.Context, *sql.TxOptions) (*sqlx.Tx, error)
}

// ReplaceMasteries writes a complete Riot mastery snapshot to the configured
// primary store and mirrors it to the rollback store in the same transaction.
func ReplaceMasteries(database db.Context, puuid string, masteries []*MasteryDAO) error {
	if starter, ok := database.(masteryTransactionStarter); ok {
		tx, err := starter.BeginTxx(context.Background(), nil)
		if err != nil {
			return err
		}
		if err := replaceMasteries(tx, puuid, masteries); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	return replaceMasteries(database, puuid, masteries)
}

func replaceMasteries(database db.Context, puuid string, masteries []*MasteryDAO) error {
	var summonerFk int64
	if err := database.Get(&summonerFk, `
		SELECT summoner_id FROM summoner_numeric_keys WHERE puuid = ?
	`, puuid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("mastery numeric key is missing for summoner %s", puuid)
		}
		return err
	}

	championIds := make([]int64, 0, len(masteries))
	seen := make(map[int64]struct{}, len(masteries))
	for _, mastery := range masteries {
		if mastery == nil || mastery.Puuid != puuid {
			return errors.New("mastery snapshot contains an invalid summoner")
		}
		if _, duplicate := seen[mastery.ChampionId]; duplicate {
			return fmt.Errorf("mastery snapshot contains duplicate champion %d", mastery.ChampionId)
		}
		seen[mastery.ChampionId] = struct{}{}
		mastery.SummonerFk = sql.NullInt64{Int64: summonerFk, Valid: true}
		if err := mastery.Upsert(database); err != nil {
			return err
		}
		championIds = append(championIds, mastery.ChampionId)
	}

	return deleteStaleMasteries(database, puuid, summonerFk, championIds)
}

func deleteStaleMasteries(database db.Context, puuid string, summonerFk int64, championIds []int64) error {
	legacyQuery := "DELETE FROM masteries WHERE puuid = ?"
	numericQuery := "DELETE FROM masteries_numeric_v2 WHERE summoner_fk = ?"
	legacyArgs := []interface{}{puuid}
	numericArgs := []interface{}{summonerFk}
	if len(championIds) > 0 {
		placeholders := "?"
		for index := 1; index < len(championIds); index++ {
			placeholders += ", ?"
		}
		legacyQuery += " AND champion_id NOT IN (" + placeholders + ")"
		numericQuery += " AND champion_id NOT IN (" + placeholders + ")"
		for _, championId := range championIds {
			legacyArgs = append(legacyArgs, championId)
			numericArgs = append(numericArgs, championId)
		}
	}

	deleteLegacy := func() error {
		_, err := database.Exec(legacyQuery, legacyArgs...)
		return err
	}
	deleteNumeric := func() error {
		_, err := database.Exec(numericQuery, numericArgs...)
		return err
	}
	if MasteryNumericV2WritesEnabled() {
		if err := deleteNumeric(); err != nil {
			return err
		}
		return deleteLegacy()
	}
	if err := deleteLegacy(); err != nil {
		return err
	}
	return deleteNumeric()
}

func GetMasteryDAOs(db db.Context, puuid string) ([]*MasteryDAO, error) {
	var masteries []*MasteryDAO
	query := `
		SELECT puuid, summoner_fk, champion_points_until_next_level,
			chest_granted, champion_id, last_play_time, champion_level,
			champion_points, champion_points_since_last_level, tokens_earned
		FROM masteries
		WHERE puuid = ?
	`
	if MasteryNumericV2ReadsEnabled() {
		query = `
			SELECT numeric_key.puuid, mastery.summoner_fk,
				mastery.champion_points_until_next_level, mastery.chest_granted,
				mastery.champion_id, mastery.last_play_time, mastery.champion_level,
				mastery.champion_points, mastery.champion_points_since_last_level,
				mastery.tokens_earned
			FROM summoner_numeric_keys numeric_key
			INNER JOIN masteries_numeric_v2 mastery
				ON mastery.summoner_fk = numeric_key.summoner_id
			WHERE numeric_key.puuid = ?
		`
	}
	if err := db.Select(&masteries, query, puuid); err != nil {
		return nil, err
	}
	return masteries, nil
}
