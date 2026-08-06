package statistics_models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
)

type ChampionStatisticMXDAO struct {
	ChampionId int `db:"champion_id" json:"championId"`
	Win        int `db:"win" json:"win"`
	Total      int `db:"total" json:"total"`

	PickRate float64 `db:"pick_rate" json:"pickRate"`
	BanRate  float64 `db:"ban_rate" json:"banRate"`

	AvgMinionsKilled float64 `db:"avg_minions_killed" json:"avgMinionsKilled"`
	AvgKills         float64 `db:"avg_kills" json:"avgKills"`
	AvgDeaths        float64 `db:"avg_deaths" json:"avgDeaths"`
	AvgAssists       float64 `db:"avg_assists" json:"avgAssists"`
	AvgGoldEarned    float64 `db:"avg_gold_earned" json:"avgGoldEarned"`
}

func GetChampionStatisticMXDAOs(db db.Context) ([]*ChampionStatisticMXDAO, error) {
	var statistics []*ChampionStatisticMXDAO
	if err := db.Select(&statistics, `
		WITH ChampionStats AS (
			SELECT
				mp.champion_id AS champion_id,
				SUM(t.win) AS win,
				COUNT(*) AS total,
				AVG(mp.total_minions_killed) as avg_minions_killed,
				AVG(mp.kills) as avg_kills,
				AVG(mp.deaths) as avg_deaths,
				AVG(mp.assists) as avg_assists,
				AVG(mp.gold_earned) as avg_gold_earned
			FROM match_participants mp
			LEFT JOIN matches m ON mp.match_id = m.match_id
			LEFT JOIN match_teams t ON mp.team_id = t.team_id AND m.match_id = t.match_id
			LEFT JOIN match_team_bans b ON b.match_id = m.match_id AND b.champion_id = mp.champion_id
			GROUP BY mp.champion_id
		), BanStats AS (
			SELECT
				champion_id,
				COUNT(*) as total_bans
			FROM match_team_bans
			GROUP BY champion_id
		), MatchCount AS (
			SELECT
				COUNT(*) as matches
			FROM matches
		)
		SELECT
			cs.*,
    		IF(ISNULL(bs.total_bans), 0, bs.total_bans / mc.matches) as ban_rate,
			cs.total / mc.matches as pick_rate
		FROM ChampionStats cs
		LEFT JOIN BanStats bs ON cs.champion_id = bs.champion_id
		CROSS JOIN MatchCount mc;
	`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]*ChampionStatisticMXDAO, 0), nil
		}
		return nil, err
	}
	return statistics, nil
}
