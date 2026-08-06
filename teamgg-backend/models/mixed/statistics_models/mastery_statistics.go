package statistics_models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type MasteryStatisticsMXDAO struct {
	ChampionId    int     `db:"champion_id" json:"championId"`
	AvgMastery    float64 `db:"avg_mastery" json:"avgMastery"`
	MaxMastery    int     `db:"max_mastery" json:"maxMastery"`
	TotalMastery  int     `db:"total_mastery" json:"totalMastery"`
	MasteredCount int     `db:"mastered_count" json:"masteredCount"`
	Count         int     `db:"count" json:"count"`
}

func GetMasteryStatisticsMXDAOs(db db.Context) ([]*MasteryStatisticsMXDAO, error) {
	var statistics []*MasteryStatisticsMXDAO
	if err := db.Select(&statistics, `
		SELECT
			m.champion_id,
			MAX(m.champion_points) as max_mastery,
			AVG(m.champion_points) as avg_mastery,
			SUM(m.champion_points) as total_mastery,
			SUM(IF(m.champion_level >= 7, 1, 0)) as mastered_count,
			COUNT(*) as count
		FROM masteries m
		GROUP BY m.champion_id;
	`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]*MasteryStatisticsMXDAO, 0), nil
		}
		return nil, err
	}
	return statistics, nil
}
