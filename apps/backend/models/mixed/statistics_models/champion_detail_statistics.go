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

func GetChampionDetailStatisticMXDAOs(db db.Context, versions []string) ([]ChampionDetailStatisticMXDAO, error) {
	var statistics []ChampionDetailStatisticMXDAO

	query, args, err := sqlx.In(`
		WITH ChampionStats AS (
			SELECT
				mp.champion_id AS champion_id,
				SUM(mp.win) AS win,
				COUNT(*) AS total,
				AVG(mp.total_minions_killed) as avg_minions_killed,
				AVG(mp.kills) as avg_kills,
				AVG(mp.deaths) as avg_deaths,
				AVG(mp.assists) as avg_assists,
				AVG(mp.gold_earned) as avg_gold_earned,
				AVG(mp.total_damage_dealt_to_champions) as avg_damage_dealt_to_champions,
				AVG(mp.total_damage_taken) as avg_damage_taken,
				AVG(mp.total_heal) as avg_heal,
				AVG(mp.vision_score) as avg_vision_score,
				AVG(mp.total_time_cc_dealt) as avg_time_cc_dealt,
				AVG(mpd.damage_self_mitigated) as avg_damage_self_mitigated,
				AVG(mpd.damage_dealt_to_buildings) as avg_damage_dealt_to_buildings,
				AVG(mpd.damage_dealt_to_objectives) as avg_damage_dealt_to_objectives,
				AVG(mpd.damage_dealt_to_turrets) as avg_damage_dealt_to_turrets,
				AVG(mp.total_heal / m.game_duration) as avg_heal_per_sec,
				AVG(mp.vision_score / m.game_duration) as avg_vision_score_per_sec,
				AVG(mp.total_damage_taken / m.game_duration) as avg_damage_taken_per_sec,
				AVG(mp.total_time_cc_dealt / m.game_duration) as avg_time_cc_dealt_per_sec,
				AVG(mpd.damage_self_mitigated / m.game_duration) as avg_damage_self_mitigated_per_sec
			FROM matches m FORCE INDEX (matches_game_version_index)
			STRAIGHT_JOIN match_participants mp ON mp.match_id = m.match_id
			LEFT JOIN match_participant_details mpd ON mp.match_id = mpd.match_id
				   AND mp.match_participant_id = mpd.match_participant_id
			WHERE game_duration > 0 AND game_version IN (?)
			GROUP BY mp.champion_id
		), BanStats AS (
			SELECT
				mtb.champion_id,
				COUNT(*) AS total_bans
			FROM match_team_bans mtb
			INNER JOIN matches m ON mtb.match_id = m.match_id
			WHERE m.game_version IN (?)
			GROUP BY mtb.champion_id
		), MatchCount AS (
			SELECT
				COUNT(*) AS matches
			FROM matches
			WHERE game_version IN (?)
		)
		SELECT
			cs.*,
			IF(ISNULL(bs.total_bans), 0, bs.total_bans / mc.matches) as ban_rate,
			cs.total / mc.matches as pick_rate
		FROM ChampionStats cs
		LEFT JOIN BanStats bs ON cs.champion_id = bs.champion_id
		CROSS JOIN MatchCount mc;
	`, versions, versions, versions)
	if err != nil {
		return nil, err
	}

	query = db.Rebind(query)
	if err := db.Select(&statistics, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return make([]ChampionDetailStatisticMXDAO, 0), nil
		}
		return nil, err
	}

	return statistics, nil
}
