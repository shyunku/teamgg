package statistics_models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
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

	// A ROW_NUMBER() partition over the complete mastery table forces MySQL to
	// sort every mastery row. Read the small champion key set first, then use
	// the (champion_id, champion_points DESC) index for a bounded top-N lookup.
	var championIds []int
	if err := db.Select(&championIds, `
		SELECT DISTINCT champion_id
		FROM masteries
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
		if err := db.Select(&championRankers, `
			SELECT
				s.puuid,
				s.game_name,
				s.tag_line,
				s.profile_icon_id,
				m.champion_id,
				m.champion_points
			FROM masteries m
			LEFT JOIN summoners s ON m.puuid = s.puuid
			WHERE m.champion_id = ?
			ORDER BY m.champion_points DESC
			LIMIT ?
		`, championId, topRanks); err != nil {
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
