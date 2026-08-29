package statistics_models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type MasteryStatisticsMXDAO struct {
	ChampionId    int     `db:"champion_id" json:"championId"`
	AvgMastery    float64 `db:"avg_mastery" json:"avgMastery"`
	MaxMastery    int64   `db:"max_mastery" json:"maxMastery"`
	TotalMastery  int64   `db:"total_mastery" json:"totalMastery"`
	MasteredCount int64   `db:"mastered_count" json:"masteredCount"`
	Count         int64   `db:"count" json:"count"`
}

func GetMasteryStatisticsMXDAOs(db db.Context) ([]*MasteryStatisticsMXDAO, error) {
	var statistics []*MasteryStatisticsMXDAO
	if err := db.Select(&statistics, `
		SELECT
			champion_id,
			max_mastery,
			IF(summoner_count = 0, 0, total_mastery / summoner_count) AS avg_mastery,
			total_mastery,
			mastered_count,
			summoner_count AS count
		FROM mastery_statistics_aggregates
		ORDER BY champion_id
	`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]*MasteryStatisticsMXDAO, 0), nil
		}
		return nil, err
	}
	return statistics, nil
}
