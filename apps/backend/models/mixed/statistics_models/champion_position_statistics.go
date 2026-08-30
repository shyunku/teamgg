package statistics_models

import (
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
	"team.gg-server/libs/db"
)

type ChampionPositionStatisticsMXDAO struct {
	ChampionId   int    `db:"champion_id" json:"championId"`
	TeamPosition string `db:"team_position" json:"teamPosition"`
	Win          int    `db:"win" json:"win"`
	Total        int    `db:"total" json:"total"`
}

func GetChampionPositionStatisticsMXDAOs(db db.Context, versions []string) ([]ChampionPositionStatisticsMXDAO, error) {
	var statistics []ChampionPositionStatisticsMXDAO
	query, args, err := sqlx.In(`
		SELECT
			participant.champion_id,
			participant.team_position,
			SUM(participant.win) AS win,
			COUNT(*) AS total
		FROM matches match_row FORCE INDEX (matches_game_version_index)
		STRAIGHT_JOIN match_participants participant ON participant.match_id = match_row.match_id
		WHERE participant.team_position != '' AND match_row.game_version IN (?)
		GROUP BY participant.champion_id, participant.team_position;
	`, versions)
	if err != nil {
		return nil, err
	}

	query = db.Rebind(query)

	if err := db.Select(&statistics, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]ChampionPositionStatisticsMXDAO, 0), nil
		}
		return nil, err
	}

	return statistics, nil
}
