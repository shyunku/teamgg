package statistics_models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	log "github.com/shyunku-libraries/go-logger"
	"team.gg-server/libs/db"
)

const championDetailStatisticsSourceTable = "champion_detail_statistics_source"

const createRecentChampionBuildsQuery = `
	INSERT INTO champion_detail_statistics_source (
		match_id, match_participant_id, champion_id, champion_name, team_position,
		kills, deaths, assists, team_id, summoner1_id, summoner2_id, win,
		enemy_champion_id, enemy_champion_name, enemy_kills, enemy_deaths, enemy_assists, enemy_win,
		primary_style, primary_perk0, primary_perk1, primary_perk2, primary_perk3,
		sub_style, sub_perk0, sub_perk1,
		stat_perk_defense, stat_perk_flex, stat_perk_offense,
		item0_id, item0_name, item1_id, item1_name, item2_id, item2_name,
		item3_id, item3_name, item4_id, item4_name, item5_id, item5_name
	)
	WITH RecentMatches AS (
		SELECT match_id
		FROM matches FORCE INDEX (matches_game_version_index)
		WHERE game_version IN (?)
	), RecentParticipants AS (
		SELECT
			mp.match_id,
			mp.match_participant_id,
			mp.champion_id,
			mp.champion_name,
			mp.team_position,
			mp.kills,
			mp.deaths,
			mp.assists,
			mp.team_id,
			mp.summoner1_id,
			mp.summoner2_id,
			mp.item0,
			mp.item1,
			mp.item2,
			mp.item3,
			mp.item4,
			mp.item5,
			mp.item6,
			mp.win
		FROM RecentMatches rm
		INNER JOIN match_participants mp ON mp.match_id = rm.match_id
		WHERE mp.team_position != ''
	), ParticipantOpponents AS (
		SELECT
			rp.*,
			enemy.champion_id AS enemy_champion_id,
			enemy.champion_name AS enemy_champion_name,
			enemy.kills AS enemy_kills,
			enemy.deaths AS enemy_deaths,
			enemy.assists AS enemy_assists,
			enemy.win AS enemy_win
		FROM RecentParticipants rp
		LEFT JOIN RecentParticipants enemy
			ON enemy.match_id = rp.match_id
			AND enemy.team_id != rp.team_id
			AND enemy.team_position = rp.team_position
	), RankedPerkSelections AS (
		SELECT
			style.match_participant_id,
			style.style_id,
			style.description,
			style.style,
			selection.perk,
			ROW_NUMBER() OVER (
				PARTITION BY style.style_id
				ORDER BY selection.perk DESC
			) AS perk_rank
		FROM RecentParticipants rp
		INNER JOIN match_participant_perk_styles style
			ON style.match_participant_id = rp.match_participant_id
		INNER JOIN match_participant_perk_style_selections selection
			ON selection.style_id = style.style_id
	), ParticipantPerks AS (
		SELECT
			ranked.match_participant_id,
			MAX(CASE WHEN ranked.description = 'primaryStyle' THEN ranked.style END) AS primary_style,
			MAX(CASE WHEN ranked.description = 'primaryStyle' AND ranked.perk_rank = 1 THEN ranked.perk END) AS primary_perk0,
			MAX(CASE WHEN ranked.description = 'primaryStyle' AND ranked.perk_rank = 2 THEN ranked.perk END) AS primary_perk1,
			MAX(CASE WHEN ranked.description = 'primaryStyle' AND ranked.perk_rank = 3 THEN ranked.perk END) AS primary_perk2,
			MAX(CASE WHEN ranked.description = 'primaryStyle' AND ranked.perk_rank = 4 THEN ranked.perk END) AS primary_perk3,
			MAX(CASE WHEN ranked.description = 'subStyle' THEN ranked.style END) AS sub_style,
			MAX(CASE WHEN ranked.description = 'subStyle' AND ranked.perk_rank = 1 THEN ranked.perk END) AS sub_perk0,
			MAX(CASE WHEN ranked.description = 'subStyle' AND ranked.perk_rank = 2 THEN ranked.perk END) AS sub_perk1,
			MAX(perks.stat_perk_defense) AS stat_perk_defense,
			MAX(perks.stat_perk_flex) AS stat_perk_flex,
			MAX(perks.stat_perk_offense) AS stat_perk_offense
		FROM RankedPerkSelections ranked
		INNER JOIN match_participant_perks perks
			ON perks.match_participant_id = ranked.match_participant_id
		GROUP BY ranked.match_participant_id
	), RankedItems AS (
		SELECT
			rp.match_id,
			rp.match_participant_id,
			item.id AS item_id,
			item.name AS item_name,
			ROW_NUMBER() OVER (
				PARTITION BY rp.match_id, rp.match_participant_id
				ORDER BY item.depth DESC, item.gold_total DESC
			) AS item_rank
		FROM RecentParticipants rp
		INNER JOIN static_items item
			ON item.id IN (rp.item0, rp.item1, rp.item2, rp.item3, rp.item4, rp.item5, rp.item6)
		WHERE item.id != 0
			AND item.required_ally IS NULL
			AND item.gold_purchasable IS TRUE
			AND item.gold_total > 0
			AND item.depth >= 3
	)
	SELECT
		participant.match_id,
		participant.match_participant_id,
		participant.champion_id,
		participant.champion_name,
		participant.team_position,
		participant.kills,
		participant.deaths,
		participant.assists,
		participant.team_id,
		participant.summoner1_id,
		participant.summoner2_id,
		participant.win,
		participant.enemy_champion_id,
		participant.enemy_champion_name,
		participant.enemy_kills,
		participant.enemy_deaths,
		participant.enemy_assists,
		participant.enemy_win,
		perks.primary_style,
		perks.primary_perk0,
		perks.primary_perk1,
		perks.primary_perk2,
		perks.primary_perk3,
		perks.sub_style,
		perks.sub_perk0,
		perks.sub_perk1,
		perks.stat_perk_defense,
		perks.stat_perk_flex,
		perks.stat_perk_offense,
		MAX(CASE WHEN item.item_rank = 1 THEN item.item_id END) AS item0_id,
		MAX(CASE WHEN item.item_rank = 1 THEN item.item_name END) AS item0_name,
		MAX(CASE WHEN item.item_rank = 2 THEN item.item_id END) AS item1_id,
		MAX(CASE WHEN item.item_rank = 2 THEN item.item_name END) AS item1_name,
		MAX(CASE WHEN item.item_rank = 3 THEN item.item_id END) AS item2_id,
		MAX(CASE WHEN item.item_rank = 3 THEN item.item_name END) AS item2_name,
		MAX(CASE WHEN item.item_rank = 4 THEN item.item_id END) AS item3_id,
		MAX(CASE WHEN item.item_rank = 4 THEN item.item_name END) AS item3_name,
		MAX(CASE WHEN item.item_rank = 5 THEN item.item_id END) AS item4_id,
		MAX(CASE WHEN item.item_rank = 5 THEN item.item_name END) AS item4_name,
		MAX(CASE WHEN item.item_rank = 6 THEN item.item_id END) AS item5_id,
		MAX(CASE WHEN item.item_rank = 6 THEN item.item_name END) AS item5_name
	FROM ParticipantOpponents participant
	INNER JOIN ParticipantPerks perks
		ON perks.match_participant_id = participant.match_participant_id
	INNER JOIN RankedItems item
		ON item.match_participant_id = participant.match_participant_id
	WHERE perks.primary_style IS NOT NULL
		AND perks.sub_style IS NOT NULL
		AND perks.primary_style != 0
		AND perks.primary_perk1 IS NOT NULL
		AND perks.primary_perk2 IS NOT NULL
		AND perks.primary_perk3 IS NOT NULL
		AND perks.sub_style != 0
		AND perks.sub_perk0 IS NOT NULL
		AND perks.sub_perk1 IS NOT NULL
	GROUP BY
		participant.match_id,
		participant.match_participant_id,
		participant.champion_id,
		participant.champion_name,
		participant.team_position,
		participant.kills,
		participant.deaths,
		participant.assists,
		participant.team_id,
		participant.summoner1_id,
		participant.summoner2_id,
		participant.win,
		participant.enemy_champion_id,
		participant.enemy_champion_name,
		participant.enemy_kills,
		participant.enemy_deaths,
		participant.enemy_assists,
		participant.enemy_win,
		perks.primary_style,
		perks.primary_perk0,
		perks.primary_perk1,
		perks.primary_perk2,
		perks.primary_perk3,
		perks.sub_style,
		perks.sub_perk0,
		perks.sub_perk1,
		perks.stat_perk_defense,
		perks.stat_perk_flex,
		perks.stat_perk_offense
`

func PrepareChampionDetailStatisticsSource(database db.Context, matchGameVersions []string) error {
	if len(matchGameVersions) == 0 {
		return errors.New("match game versions are required")
	}
	started := time.Now()
	if _, err := database.Exec("TRUNCATE TABLE " + championDetailStatisticsSourceTable); err != nil {
		return fmt.Errorf("truncate champion detail statistics source: %w", err)
	}
	query, args, err := sqlx.In(createRecentChampionBuildsQuery, matchGameVersions)
	if err != nil {
		return fmt.Errorf("bind recent champion build versions: %w", err)
	}
	if _, err := database.Exec(database.Rebind(query), args...); err != nil {
		return fmt.Errorf("populate champion detail statistics source: %w", err)
	}
	var rowCount int64
	if err := database.Get(&rowCount, "SELECT COUNT(*) FROM "+championDetailStatisticsSourceTable); err != nil {
		return fmt.Errorf("count champion detail statistics source: %w", err)
	}
	log.Infof(
		"champion detail filtered source ready: patches=%d rows=%d staging_tables=1 duration=%s",
		len(matchGameVersions), rowCount, time.Since(started),
	)
	return nil
}

const championDetailMetaQuery = `
	WITH SummonerSpellCounts AS (
		SELECT champion_id, team_position, primary_style, sub_style,
			summoner1_id, summoner2_id, COUNT(*) AS count
		FROM champion_detail_statistics_source
		GROUP BY champion_id, team_position, primary_style, sub_style, summoner1_id, summoner2_id
	), SummonerSpellRanks AS (
		SELECT *, ROW_NUMBER() OVER (
			PARTITION BY champion_id, team_position, primary_style, sub_style
			ORDER BY count DESC
		) AS spell_rank
		FROM SummonerSpellCounts
	), PerkCounts AS (
		SELECT champion_id, team_position, primary_style, sub_style,
			primary_perk0, primary_perk1, primary_perk2, primary_perk3,
			sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense,
			COUNT(*) AS count
		FROM champion_detail_statistics_source
		GROUP BY champion_id, team_position, primary_style, sub_style,
			primary_perk0, primary_perk1, primary_perk2, primary_perk3,
			sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense
	), PerkRanks AS (
		SELECT *, ROW_NUMBER() OVER (
			PARTITION BY champion_id, team_position, primary_style, sub_style
			ORDER BY count DESC
		) AS perk_rank
		FROM PerkCounts
	), FullItemTreeGroups AS (
		SELECT champion_id, champion_name, team_position, primary_style, sub_style,
			item0_id, item1_id, item2_id, item3_id, item4_id, item5_id,
			item0_name, item1_name, item2_name, item3_name, item4_name, item5_name,
			(item0_id IS NOT NULL) + (item1_id IS NOT NULL) + (item2_id IS NOT NULL)
				+ (item3_id IS NOT NULL) + (item4_id IS NOT NULL) + (item5_id IS NOT NULL) AS item_count,
			COUNT(*) AS full_item_tree_count
		FROM champion_detail_statistics_source
		WHERE item0_id IS NOT NULL AND item1_id IS NOT NULL AND item2_id IS NOT NULL
		GROUP BY champion_id, champion_name, team_position, primary_style, sub_style,
			item0_id, item1_id, item2_id, item3_id, item4_id, item5_id,
			item0_name, item1_name, item2_name, item3_name, item4_name, item5_name
	), FullItemTreeRanks AS (
		SELECT *, ROW_NUMBER() OVER (
			PARTITION BY champion_id, champion_name, team_position, primary_style, sub_style,
				item0_id, item1_id, item2_id
			ORDER BY item_count DESC, full_item_tree_count DESC
		) AS item_combo_rank
		FROM FullItemTreeGroups
	), RefinedMetaGroups AS (
		SELECT champion_id, champion_name, team_position, primary_style, sub_style,
			item0_id, item1_id, item2_id, SUM(win) AS wins, COUNT(*) AS total, AVG(win) AS win_rate
		FROM champion_detail_statistics_source
		WHERE item0_id IS NOT NULL AND item1_id IS NOT NULL AND item2_id IS NOT NULL
		GROUP BY champion_id, champion_name, team_position, primary_style, sub_style,
			item0_id, item1_id, item2_id
	), RankedMetas AS (
		SELECT *, ROW_NUMBER() OVER (
			PARTITION BY champion_id, champion_name, team_position
			ORDER BY total DESC, win_rate DESC
		) AS meta_rank
		FROM RefinedMetaGroups
	)
	SELECT
		ranked.champion_id, ranked.champion_name, ranked.team_position,
		ranked.primary_style, perks.primary_perk0, perks.primary_perk1,
		perks.primary_perk2, perks.primary_perk3,
		ranked.sub_style, perks.sub_perk0, perks.sub_perk1,
		perks.stat_perk_defense, perks.stat_perk_flex, perks.stat_perk_offense,
		spells.summoner1_id, spells.summoner2_id,
		ranked.item0_id, ranked.item1_id, ranked.item2_id,
		items.item3_id, items.item4_id, items.item5_id,
		items.item0_name, items.item1_name, items.item2_name,
		items.item3_name, items.item4_name, items.item5_name,
		ranked.wins, ranked.total, ranked.win_rate, ranked.meta_rank
	FROM RankedMetas ranked
	LEFT JOIN FullItemTreeRanks items
		ON ranked.champion_id = items.champion_id
		AND ranked.champion_name = items.champion_name
		AND ranked.team_position = items.team_position
		AND ranked.primary_style = items.primary_style
		AND ranked.sub_style = items.sub_style
		AND ranked.item0_id = items.item0_id
		AND ranked.item1_id = items.item1_id
		AND ranked.item2_id = items.item2_id
		AND items.item_combo_rank = 1
	LEFT JOIN SummonerSpellRanks spells
		ON ranked.champion_id = spells.champion_id
		AND ranked.team_position = spells.team_position
		AND ranked.primary_style = spells.primary_style
		AND ranked.sub_style = spells.sub_style
		AND spells.spell_rank = 1
	LEFT JOIN PerkRanks perks
		ON ranked.champion_id = perks.champion_id
		AND ranked.team_position = perks.team_position
		AND ranked.primary_style = perks.primary_style
		AND ranked.sub_style = perks.sub_style
		AND perks.perk_rank = 1
	WHERE ranked.meta_rank <= 15 OR (ranked.win_rate > 0.5 AND ranked.total >= 50)
	ORDER BY ranked.champion_name, ranked.team_position, ranked.meta_rank
`

const championCounterQuery = `
	WITH CounterSummonerSpellCounts AS (
		SELECT champion_id, enemy_champion_id, team_position, primary_style, sub_style,
			summoner1_id, summoner2_id, COUNT(*) AS count, AVG(win) AS win_rate
		FROM champion_detail_statistics_source
		WHERE enemy_champion_id IS NOT NULL
		GROUP BY champion_id, enemy_champion_id, team_position, primary_style, sub_style,
			summoner1_id, summoner2_id
	), CounterSummonerSpellRanks AS (
		SELECT *, ROW_NUMBER() OVER (
			PARTITION BY champion_id, enemy_champion_id, team_position
			ORDER BY count DESC, win_rate DESC
		) AS spell_rank
		FROM CounterSummonerSpellCounts
	), CounterPerkCounts AS (
		SELECT champion_id, enemy_champion_id, team_position, primary_style, sub_style,
			primary_perk0, primary_perk1, primary_perk2, primary_perk3,
			sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense,
			COUNT(*) AS count, AVG(win) AS win_rate
		FROM champion_detail_statistics_source
		WHERE enemy_champion_id IS NOT NULL
		GROUP BY champion_id, enemy_champion_id, team_position, primary_style, sub_style,
			primary_perk0, primary_perk1, primary_perk2, primary_perk3,
			sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense
	), CounterPerkRanks AS (
		SELECT *, ROW_NUMBER() OVER (
			PARTITION BY champion_id, enemy_champion_id, team_position
			ORDER BY count DESC, win_rate DESC
		) AS perk_rank
		FROM CounterPerkCounts
	), CounterFullItemTreeGroups AS (
		SELECT champion_id, enemy_champion_id, team_position,
			item0_id, item1_id, item2_id, item3_id, item4_id, item5_id,
			(item0_id IS NOT NULL) + (item1_id IS NOT NULL) + (item2_id IS NOT NULL)
				+ (item3_id IS NOT NULL) + (item4_id IS NOT NULL) + (item5_id IS NOT NULL) AS item_count,
			AVG(win) AS win_rate, COUNT(*) AS full_item_tree_count
		FROM champion_detail_statistics_source
		WHERE enemy_champion_id IS NOT NULL
			AND item0_id IS NOT NULL AND item1_id IS NOT NULL AND item2_id IS NOT NULL
		GROUP BY champion_id, enemy_champion_id, team_position,
			item0_id, item1_id, item2_id, item3_id, item4_id, item5_id
	), CounterFullItemTreeRanks AS (
		SELECT *, ROW_NUMBER() OVER (
			PARTITION BY champion_id, enemy_champion_id, team_position
			ORDER BY item_count DESC, full_item_tree_count DESC
		) AS item_combo_rank
		FROM CounterFullItemTreeGroups
	), CounterMetaGroups AS (
		SELECT champion_id, champion_name, team_position,
			enemy_champion_id, enemy_champion_name,
			AVG(kills) AS avg_kills, AVG(deaths) AS avg_deaths, AVG(assists) AS avg_assists,
			SUM(win) AS wins, AVG(win) AS win_rate,
			AVG(enemy_kills) AS avg_enemy_kills,
			AVG(enemy_deaths) AS avg_enemy_deaths,
			AVG(enemy_assists) AS avg_enemy_assists,
			SUM(enemy_win) AS enemy_wins, AVG(enemy_win) AS enemy_win_rate,
			COUNT(DISTINCT match_id) AS total
		FROM champion_detail_statistics_source
		WHERE enemy_champion_id IS NOT NULL
		GROUP BY champion_id, champion_name, team_position,
			enemy_champion_id, enemy_champion_name
	)
	SELECT
		counter.champion_id, counter.champion_name, counter.team_position,
		counter.enemy_champion_id, counter.enemy_champion_name, counter.total,
		counter.avg_kills, counter.avg_deaths, counter.avg_assists,
		counter.wins, counter.win_rate,
		counter.avg_enemy_kills, counter.avg_enemy_deaths, counter.avg_enemy_assists,
		counter.enemy_wins, counter.enemy_win_rate,
		(spells.win_rate + perks.win_rate + items.win_rate) / 3 AS total_win_rate,
		spells.summoner1_id, spells.summoner2_id,
		perks.primary_style, perks.primary_perk0, perks.primary_perk1,
		perks.primary_perk2, perks.primary_perk3,
		perks.sub_style, perks.sub_perk0, perks.sub_perk1,
		perks.stat_perk_defense, perks.stat_perk_flex, perks.stat_perk_offense,
		items.item0_id, items.item1_id, items.item2_id,
		items.item3_id, items.item4_id, items.item5_id
	FROM CounterMetaGroups counter
	LEFT JOIN CounterFullItemTreeRanks items
		ON counter.champion_id = items.champion_id
		AND counter.enemy_champion_id = items.enemy_champion_id
		AND counter.team_position = items.team_position
		AND items.item_combo_rank = 1
	LEFT JOIN CounterSummonerSpellRanks spells
		ON counter.champion_id = spells.champion_id
		AND counter.enemy_champion_id = spells.enemy_champion_id
		AND counter.team_position = spells.team_position
		AND spells.spell_rank = 1
	LEFT JOIN CounterPerkRanks perks
		ON counter.champion_id = perks.champion_id
		AND counter.enemy_champion_id = perks.enemy_champion_id
		AND counter.team_position = perks.team_position
		AND perks.perk_rank = 1
`

type ChampionDetailStatisticsMetaMXDAO struct {
	ChampionId   int    `db:"champion_id" json:"championId"`
	ChampionName string `db:"champion_name" json:"championName"`
	TeamPosition string `db:"team_position" json:"teamPosition"`

	PrimaryStyle    int `db:"primary_style" json:"primaryStyle"`
	PrimaryPerk0    int `db:"primary_perk0" json:"primaryPerk0"`
	PrimaryPerk1    int `db:"primary_perk1" json:"primaryPerk1"`
	PrimaryPerk2    int `db:"primary_perk2" json:"primaryPerk2"`
	PrimaryPerk3    int `db:"primary_perk3" json:"primaryPerk3"`
	SubStyle        int `db:"sub_style" json:"subStyle"`
	SubPerk0        int `db:"sub_perk0" json:"subPerk0"`
	SubPerk1        int `db:"sub_perk1" json:"subPerk1"`
	StatPerkDefense int `db:"stat_perk_defense" json:"statPerkDefense"`
	StatPerkFlex    int `db:"stat_perk_flex" json:"statPerkFlex"`
	StatPerkOffense int `db:"stat_perk_offense" json:"statPerkOffense"`

	Summoner1Id int `db:"summoner1_id" json:"summoner1Id"`
	Summoner2Id int `db:"summoner2_id" json:"summoner2Id"`

	Item0Id int  `db:"item0_id" json:"item0Id"`
	Item1Id int  `db:"item1_id" json:"item1Id"`
	Item2Id int  `db:"item2_id" json:"item2Id"`
	Item3Id *int `db:"item3_id" json:"item3Id"`
	Item4Id *int `db:"item4_id" json:"item4Id"`
	Item5Id *int `db:"item5_id" json:"item5Id"`

	Item0Name string  `db:"item0_name" json:"item0Name"`
	Item1Name string  `db:"item1_name" json:"item1Name"`
	Item2Name string  `db:"item2_name" json:"item2Name"`
	Item3Name *string `db:"item3_name" json:"item3Name"`
	Item4Name *string `db:"item4_name" json:"item4Name"`
	Item5Name *string `db:"item5_name" json:"item5Name"`

	ItemCount int     `db:"item_count" json:"itemCount"`
	Wins      int     `db:"wins" json:"wins"`
	Total     int     `db:"total" json:"total"`
	WinRate   float64 `db:"win_rate" json:"winRate"`

	MetaRank int `db:"meta_rank" json:"metaRank"`
}

func GetChampionDetailStatisticsMetaMXDAOs(database db.Context) ([]ChampionDetailStatisticsMetaMXDAO, error) {
	var result []ChampionDetailStatisticsMetaMXDAO
	if err := database.Select(&result, championDetailMetaQuery); err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return make([]ChampionDetailStatisticsMetaMXDAO, 0), nil
		}
		return nil, err
	}
	return result, nil
}

type ChampionCounterStatisticsMXDAO struct {
	ChampionId        int    `db:"champion_id" json:"championId"`
	ChampionName      string `db:"champion_name" json:"championName"`
	TeamPosition      string `db:"team_position" json:"teamPosition"`
	EnemyChampionId   int    `db:"enemy_champion_id" json:"enemyChampionId"`
	EnemyChampionName string `db:"enemy_champion_name" json:"enemyChampionName"`

	Total int `db:"total" json:"total"` // total game counts

	Summoner1Id int `db:"summoner1_id" json:"summoner1Id"`
	Summoner2Id int `db:"summoner2_id" json:"summoner2Id"`

	PrimaryStyle    *int `db:"primary_style" json:"primaryStyle"`
	PrimaryPerk0    *int `db:"primary_perk0" json:"primaryPerk0"`
	PrimaryPerk1    *int `db:"primary_perk1" json:"primaryPerk1"`
	PrimaryPerk2    *int `db:"primary_perk2" json:"primaryPerk2"`
	PrimaryPerk3    *int `db:"primary_perk3" json:"primaryPerk3"`
	SubStyle        *int `db:"sub_style" json:"subStyle"`
	SubPerk0        *int `db:"sub_perk0" json:"subPerk0"`
	SubPerk1        *int `db:"sub_perk1" json:"subPerk1"`
	StatPerkDefense *int `db:"stat_perk_defense" json:"statPerkDefense"`
	StatPerkFlex    *int `db:"stat_perk_flex" json:"statPerkFlex"`
	StatPerkOffense *int `db:"stat_perk_offense" json:"statPerkOffense"`

	Item0Id *int `db:"item0_id" json:"item0Id"`
	Item1Id *int `db:"item1_id" json:"item1Id"`
	Item2Id *int `db:"item2_id" json:"item2Id"`
	Item3Id *int `db:"item3_id" json:"item3Id"`
	Item4Id *int `db:"item4_id" json:"item4Id"`
	Item5Id *int `db:"item5_id" json:"item5Id"`

	AvgKills   float64 `db:"avg_kills" json:"avgKills"`
	AvgDeaths  float64 `db:"avg_deaths" json:"avgDeaths"`
	AvgAssists float64 `db:"avg_assists" json:"avgAssists"`
	Wins       int     `db:"wins" json:"wins"`
	WinRate    float64 `db:"win_rate" json:"winRate"`

	TotalWinRate *float64 `db:"total_win_rate" json:"totalWinRate"`

	EnemyAvgKills   *float64 `db:"avg_enemy_kills" json:"enemyAvgKills"`
	EnemyAvgDeaths  *float64 `db:"avg_enemy_deaths" json:"enemyAvgDeaths"`
	EnemyAvgAssists *float64 `db:"avg_enemy_assists" json:"enemyAvgAssists"`
	EnemyWins       *int     `db:"enemy_wins" json:"enemyWins"`
	EnemyWinRate    *float64 `db:"enemy_win_rate" json:"enemyWinRate"`
}

func GetChampionCounterStatisticsMXDAOs(database db.Context) ([]ChampionCounterStatisticsMXDAO, error) {
	var result []ChampionCounterStatisticsMXDAO
	if err := database.Select(&result, championCounterQuery); err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return make([]ChampionCounterStatisticsMXDAO, 0), nil
		}
		return nil, err
	}
	return result, nil
}
