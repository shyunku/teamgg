package statistics_models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
	"team.gg-server/models"
)

type MasteryStatisticsTopRankersMXDAO struct {
	Puuid         string `db:"puuid" json:"puuid"`
	ProfileIconId int    `db:"profile_icon_id" json:"profileIconId"`
	GameName      string `db:"game_name" json:"gameName"`
	TagLine       string `db:"tag_line" json:"tagLine"`

	Ranks int `db:"ranks" json:"ranks"`

	ChampionId     int `db:"champion_id" json:"championId"`
	ChampionPoints int `db:"champion_points" json:"championPoints"`
}

func GetMasteryStatisticsTopRankersMXDAOs(db db.Context, topRanks int) ([]*MasteryStatisticsTopRankersMXDAO, error) {
	if topRanks <= 0 {
		return make([]*MasteryStatisticsTopRankersMXDAO, 0), nil
	}

	// Read the materialized champion key set, then perform one bounded covering
	// index lookup per champion instead of sorting the complete mastery table.
	var championIds []int
	if err := db.Select(&championIds, `
		SELECT champion_id
		FROM mastery_statistics_aggregates
		ORDER BY champion_id
	`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]*MasteryStatisticsTopRankersMXDAO, 0), nil
		}
		return nil, err
	}

	topRankers := make([]*MasteryStatisticsTopRankersMXDAO, 0, len(championIds)*topRanks)
	for _, championId := range championIds {
		var championRankers []*MasteryStatisticsTopRankersMXDAO
		rankerQuery := `
			SELECT
				s.puuid,
				s.game_name,
				s.tag_line,
				s.profile_icon_id,
				m.champion_id,
				m.champion_points
			FROM masteries m FORCE INDEX (masteries_champion_points_level_covering_index)
			LEFT JOIN summoners s ON m.puuid = s.puuid
			WHERE m.champion_id = ?
			ORDER BY m.champion_points DESC
			LIMIT ?
		`
		if models.MasteryNumericV2ReadsEnabled() {
			rankerQuery = `
				SELECT
					s.puuid,
					s.game_name,
					s.tag_line,
					s.profile_icon_id,
					m.champion_id,
					m.champion_points
				FROM masteries_numeric_v2 m
					FORCE INDEX (masteries_numeric_champion_points_level_covering_index)
				INNER JOIN summoner_numeric_keys numeric_key
					ON m.summoner_fk = numeric_key.summoner_id
				LEFT JOIN summoners s ON numeric_key.puuid = s.puuid
				WHERE m.champion_id = ?
				ORDER BY m.champion_points DESC
				LIMIT ?
			`
		}
		if err := db.Select(&championRankers, rankerQuery, championId, topRanks); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		for index, ranker := range championRankers {
			ranker.Ranks = index + 1
			topRankers = append(topRankers, ranker)
		}
	}
	return topRankers, nil
}
