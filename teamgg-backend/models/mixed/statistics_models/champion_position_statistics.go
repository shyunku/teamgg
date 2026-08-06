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
			champion_id,
			team_position,
			SUM(win) AS win,
			COUNT(*) AS total
		FROM match_participants
		LEFT JOIN matches ON match_participants.match_id = matches.match_id
		WHERE team_position != '' AND game_version IN (?)
		GROUP BY champion_id, team_position;
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
