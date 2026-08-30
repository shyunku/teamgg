package statistics_models

import (
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"team.gg-server/libs/db"
)

type ChampionDetailStatisticMXDAO struct {
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

	AvgDamageDealtToChampions  float64 `db:"avg_damage_dealt_to_champions" json:"avgDamageDealtToChampions"`
	AvgDamageTaken             float64 `db:"avg_damage_taken" json:"avgDamageTaken"`
	AvgHeal                    float64 `db:"avg_heal" json:"avgHeal"`
	AvgVisionScore             float64 `db:"avg_vision_score" json:"avgVisionScore"`
	AvgTimeCCDealt             float64 `db:"avg_time_cc_dealt" json:"avgTimeCCDealt"`
	AvgDamageSelfMitigated     float64 `db:"avg_damage_self_mitigated" json:"avgDamageSelfMitigated"`
	AvgDamageDealtToBuildings  float64 `db:"avg_damage_dealt_to_buildings" json:"avgDamageDealtToBuildings"`
	AvgDamageDealtToObjectives float64 `db:"avg_damage_dealt_to_objectives" json:"avgDamageDealtToObjectives"`
	AvgDamageDealtToTurrets    float64 `db:"avg_damage_dealt_to_turrets" json:"avgDamageDealtToTurrets"`

	AvgHealPerSec                float64 `db:"avg_heal_per_sec" json:"avgHealPerSec"`
	AvgVisionScorePerSec         float64 `db:"avg_vision_score_per_sec" json:"avgVisionScorePerSec"`
	AvgDamageTakenPerSec         float64 `db:"avg_damage_taken_per_sec" json:"avgDamageTakenPerSec"`
	AvgTimeCCDealtPerSec         float64 `db:"avg_time_cc_dealt_per_sec" json:"avgTimeCCDealtPerSec"`
	AvgDamageSelfMitigatedPerSec float64 `db:"avg_damage_self_mitigated_per_sec" json:"avgDamageSelfMitigatedPerSec"`
}

func GetChampionDetailStatisticMXDAOs(database db.Context, versions []string) ([]ChampionDetailStatisticMXDAO, error) {
	var statistics []ChampionDetailStatisticMXDAO

	query, args, err := sqlx.In(`
		WITH ChampionStats AS (
			SELECT
				champion_id,
				SUM(win) AS win,
				COUNT(*) AS total,
				AVG(total_minions_killed) AS avg_minions_killed,
				AVG(kills) AS avg_kills,
				AVG(deaths) AS avg_deaths,
				AVG(assists) AS avg_assists,
				AVG(gold_earned) AS avg_gold_earned,
				AVG(total_damage_dealt_to_champions) AS avg_damage_dealt_to_champions,
				AVG(total_damage_taken) AS avg_damage_taken,
				AVG(total_heal) AS avg_heal,
				AVG(vision_score) AS avg_vision_score,
				AVG(total_time_cc_dealt) AS avg_time_cc_dealt,
				AVG(damage_self_mitigated) AS avg_damage_self_mitigated,
				AVG(damage_dealt_to_buildings) AS avg_damage_dealt_to_buildings,
				AVG(damage_dealt_to_objectives) AS avg_damage_dealt_to_objectives,
				AVG(damage_dealt_to_turrets) AS avg_damage_dealt_to_turrets,
				AVG(total_heal / NULLIF(game_duration, 0)) AS avg_heal_per_sec,
				AVG(vision_score / NULLIF(game_duration, 0)) AS avg_vision_score_per_sec,
				AVG(total_damage_taken / NULLIF(game_duration, 0)) AS avg_damage_taken_per_sec,
				AVG(total_time_cc_dealt / NULLIF(game_duration, 0)) AS avg_time_cc_dealt_per_sec,
				AVG(damage_self_mitigated / NULLIF(game_duration, 0)) AS avg_damage_self_mitigated_per_sec
			FROM champion_detail_statistics_participants
			WHERE game_duration > 0 AND game_version IN (?)
			GROUP BY champion_id
		), BanStats AS (
			SELECT champion_id, COUNT(*) AS total_bans
			FROM champion_detail_statistics_bans
			WHERE game_version IN (?)
			GROUP BY champion_id
		), MatchCount AS (
			SELECT COUNT(DISTINCT match_id) AS matches
			FROM champion_detail_statistics_participants
			WHERE game_version IN (?)
		)
		SELECT
			cs.*,
			IF(ISNULL(bs.total_bans), 0, bs.total_bans / NULLIF(mc.matches, 0)) AS ban_rate,
			cs.total / NULLIF(mc.matches, 0) AS pick_rate
		FROM ChampionStats cs
		LEFT JOIN BanStats bs ON cs.champion_id = bs.champion_id
		CROSS JOIN MatchCount mc;
	`, versions, versions, versions)
	if err != nil {
		return nil, err
	}

	query = database.Rebind(query)
	if err := database.Select(&statistics, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]ChampionDetailStatisticMXDAO, 0), nil
		}
		return nil, err
	}

	return statistics, nil
}
