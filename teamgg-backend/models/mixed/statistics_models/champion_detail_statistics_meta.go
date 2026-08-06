package statistics_models

import (
	"database/sql"
	"errors"
	log "github.com/shyunku-libraries/go-logger"
	"strings"
	"team.gg-server/libs/db"
)

func CreateTemporaryTables(db db.Context, matchGameVersions []string) error {
	if len(matchGameVersions) == 0 {
		return errors.New("match game versions are required")
	}

	commonSqls := []string{
		`CREATE TEMPORARY TABLE IF NOT EXISTS RecentParticipants AS
			SELECT mp.match_id,
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
			FROM matches m
			INNER JOIN match_participants mp ON m.match_id = mp.match_id
			WHERE m.game_version IN ('` + strings.Join(matchGameVersions, `', '`) + `')
			  AND mp.team_position != '';`,
		`CREATE INDEX recent_participants_participant_index
			ON RecentParticipants (match_participant_id);`,
		`CREATE INDEX recent_participants_match_position_index
			ON RecentParticipants (match_id, team_position, team_id);`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS RecentPerkStyles AS
			SELECT mpps.match_participant_id,
				   mpps.style_id,
				   mpps.description,
				   mpps.style
			FROM match_participant_perk_styles mpps
			INNER JOIN RecentParticipants rp
				ON mpps.match_participant_id = rp.match_participant_id;`,
		`CREATE INDEX recent_perk_styles_style_index
			ON RecentPerkStyles (style_id);`,
		`CREATE INDEX recent_perk_styles_participant_index
			ON RecentPerkStyles (match_participant_id);`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS SortedPerks AS
			SELECT rps.style_id,
				   selection.perk,
				   ROW_NUMBER() OVER (PARTITION BY rps.style_id ORDER BY selection.perk DESC) AS perk_rank
			FROM RecentPerkStyles rps
			INNER JOIN match_participant_perk_style_selections selection
				ON rps.style_id = selection.style_id;`,
		`CREATE INDEX sorted_perks_style_index
			ON SortedPerks (style_id, perk_rank);`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS PerkGroups AS
			SELECT rps.match_participant_id, rps.style_id,
				   MAX(CASE WHEN sp.perk_rank = 1 THEN sp.perk END) AS perk0,
				   MAX(CASE WHEN sp.perk_rank = 2 THEN sp.perk END) AS perk1,
				   MAX(CASE WHEN sp.perk_rank = 3 THEN sp.perk END) AS perk2,
				   MAX(CASE WHEN sp.perk_rank = 4 THEN sp.perk END) AS perk3
			FROM RecentPerkStyles rps
			LEFT JOIN SortedPerks sp ON rps.style_id = sp.style_id
			GROUP BY rps.match_participant_id, rps.style_id;`,
		`CREATE INDEX perk_groups_participant_style_index
			ON PerkGroups (match_participant_id, style_id);`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS PerkStyleGroups AS
			SELECT rps.match_participant_id,
				   MAX(CASE WHEN rps.description = 'primaryStyle' THEN rps.style END) AS primary_style,
				   MAX(CASE WHEN rps.description = 'primaryStyle' THEN pg.perk0 END) AS primary_perk0,
				   MAX(CASE WHEN rps.description = 'primaryStyle' THEN pg.perk1 END) AS primary_perk1,
				   MAX(CASE WHEN rps.description = 'primaryStyle' THEN pg.perk2 END) AS primary_perk2,
				   MAX(CASE WHEN rps.description = 'primaryStyle' THEN pg.perk3 END) AS primary_perk3,
				   MAX(CASE WHEN rps.description = 'subStyle' THEN rps.style END) AS sub_style,
				   MAX(CASE WHEN rps.description = 'subStyle' THEN pg.perk0 END) AS sub_perk0,
				   MAX(CASE WHEN rps.description = 'subStyle' THEN pg.perk1 END) AS sub_perk1,
				   mpp.stat_perk_defense,
				   mpp.stat_perk_flex,
				   mpp.stat_perk_offense
			FROM RecentPerkStyles rps
			LEFT JOIN match_participant_perks mpp ON rps.match_participant_id = mpp.match_participant_id
			LEFT JOIN PerkGroups pg ON rps.match_participant_id = pg.match_participant_id
								   AND rps.style_id = pg.style_id
			GROUP BY rps.match_participant_id;`,
		`CREATE INDEX perk_style_groups_participant_index
			ON PerkStyleGroups (match_participant_id);`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS ItemDetails AS
			SELECT mp.match_id, mp.match_participant_id, mp.champion_id, mp.champion_name, mp.team_position,
				   mp.kills, mp.deaths, mp.assists, mp.team_id,
				   mp.summoner1_id, mp.summoner2_id, si.id AS item_id, si.name AS item_name,
				   si.gold_total AS gold_value,
				   pc.primary_style, pc.primary_perk0, pc.primary_perk1, pc.primary_perk2, pc.primary_perk3,
				   pc.sub_style, pc.sub_perk0, pc.sub_perk1,
				   pc.stat_perk_defense, pc.stat_perk_flex, pc.stat_perk_offense,
				   mp.win,
				   ROW_NUMBER() OVER (PARTITION BY mp.match_id, mp.match_participant_id ORDER BY si.depth DESC, si.gold_total DESC) AS item_rank
			FROM RecentParticipants mp
			JOIN static_items si ON si.id IN (mp.item0, mp.item1, mp.item2, mp.item3, mp.item4, mp.item5, mp.item6)
			LEFT JOIN PerkStyleGroups pc ON mp.match_participant_id = pc.match_participant_id
			WHERE si.id IS NOT NULL
			  AND si.id != 0
			  AND si.required_ally IS NULL
			  AND si.gold_purchasable IS TRUE
			  AND si.gold_total > 0
			  AND si.depth >= 3
			  AND mp.team_position != ''
			  AND pc.primary_style IS NOT NULL
			  AND pc.sub_style IS NOT NULL
			  AND pc.primary_style != 0
		  	  AND pc.primary_perk1 IS NOT NULL
			  AND pc.primary_perk2 IS NOT NULL
			  AND pc.primary_perk3 IS NOT NULL
			  AND pc.sub_style != 0
			  AND pc.sub_perk0 IS NOT NULL
			  AND pc.sub_perk1 IS NOT NULL;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS MainGroup AS
			SELECT match_id, match_participant_id, champion_id, champion_name, team_position,
				   kills, deaths, assists, win, team_id,
				   summoner1_id, summoner2_id, primary_style, primary_perk0, primary_perk1, primary_perk2, primary_perk3,
				   sub_style, sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense,
				   MAX(CASE WHEN item_rank = 1 THEN item_id END) AS item0_id,
				   MAX(CASE WHEN item_rank = 1 THEN item_name END) AS item0_name,
				   MAX(CASE WHEN item_rank = 2 THEN item_id END) AS item1_id,
				   MAX(CASE WHEN item_rank = 2 THEN item_name END) AS item1_name,
				   MAX(CASE WHEN item_rank = 3 THEN item_id END) AS item2_id,
				   MAX(CASE WHEN item_rank = 3 THEN item_name END) AS item2_name,
				   MAX(CASE WHEN item_rank = 4 THEN item_id END) AS item3_id,
				   MAX(CASE WHEN item_rank = 4 THEN item_name END) AS item3_name,
				   MAX(CASE WHEN item_rank = 5 THEN item_id END) AS item4_id,
				   MAX(CASE WHEN item_rank = 5 THEN item_name END) AS item4_name,
				   MAX(CASE WHEN item_rank = 6 THEN item_id END) AS item5_id,
				   MAX(CASE WHEN item_rank = 6 THEN item_name END) AS item5_name
			FROM ItemDetails
			GROUP BY match_id, match_participant_id, champion_id, champion_name, team_position,
					 kills, deaths, assists, win, team_id,
					 primary_style, sub_style, summoner1_id, summoner2_id,
					 primary_perk0, primary_perk1, primary_perk2, primary_perk3,
					 sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense;`,
	}
	metaSqls := []string{
		`CREATE TEMPORARY TABLE IF NOT EXISTS SummonerSpellCounts AS 
			SELECT champion_id, team_position, primary_style, sub_style, summoner1_id, summoner2_id, COUNT(*) AS count
			FROM MainGroup
			GROUP BY champion_id, team_position, primary_style, sub_style, summoner1_id, summoner2_id;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS SummonerSpellRanks AS 
			SELECT *, ROW_NUMBER() OVER (PARTITION BY champion_id, team_position, primary_style, sub_style ORDER BY count DESC) AS spell_rank
			FROM SummonerSpellCounts;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS PerkCounts AS 
			SELECT champion_id, team_position, primary_style, sub_style, primary_perk0, primary_perk1, primary_perk2, primary_perk3, 
				   sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense, COUNT(*) AS count
			FROM MainGroup
			GROUP BY champion_id, team_position, primary_style, sub_style, primary_perk0, primary_perk1, primary_perk2, primary_perk3, 
					 sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS PerkRanks AS 
			SELECT *, ROW_NUMBER() OVER (PARTITION BY champion_id, team_position, primary_style, sub_style ORDER BY count DESC) AS perk_rank
			FROM PerkCounts;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS FullItemTreeGroups AS 
			SELECT champion_id, champion_name, team_position, primary_style, sub_style, 
				   item0_id, item1_id, item2_id, item3_id, item4_id, item5_id,
				   item0_name, item1_name, item2_name, item3_name, item4_name, item5_name,
				   IF(item0_id IS NOT NULL, 1, 0) 
					   + IF(item1_id IS NOT NULL, 1, 0) 
					   + IF(item2_id IS NOT NULL, 1, 0) 
					   + IF(item3_id IS NOT NULL, 1, 0) 
					   + IF(item4_id IS NOT NULL, 1, 0) 
					   + IF(item5_id IS NOT NULL, 1, 0) 
				   AS item_count, 
				   COUNT(*) AS full_item_tree_count
			FROM MainGroup
			WHERE item0_id IS NOT NULL AND item1_id IS NOT NULL AND item2_id IS NOT NULL
			GROUP BY champion_id, champion_name, team_position, primary_style, sub_style, 
					 item0_id, item1_id, item2_id, item3_id, item4_id, item5_id, 
					 item0_name, item1_name, item2_name, item3_name, item4_name, item5_name;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS FullItemTreeRanks AS 
			SELECT *, ROW_NUMBER() OVER (PARTITION BY champion_id, champion_name, team_position, primary_style, sub_style, item0_id, item1_id, item2_id ORDER BY item_count DESC, full_item_tree_count DESC) AS item_combo_rank
			FROM FullItemTreeGroups;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS RefinedMetaGroups AS 
			SELECT champion_id, champion_name, team_position, primary_style, sub_style, item0_id, item1_id, item2_id, SUM(win) AS wins, COUNT(*) AS total, AVG(win) AS win_rate
			FROM MainGroup
			WHERE item0_id IS NOT NULL AND item1_id IS NOT NULL AND item2_id IS NOT NULL
			GROUP BY champion_id, champion_name, team_position, primary_style, sub_style, item0_id, item1_id, item2_id;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS RankedMetas AS 
			SELECT *, ROW_NUMBER() OVER (PARTITION BY champion_id, champion_name, team_position ORDER BY total DESC, win_rate DESC) AS meta_rank
			FROM RefinedMetaGroups;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS FinalRankedMetas AS 
			SELECT rm.*, fitr.item3_id, fitr.item4_id, fitr.item5_id, fitr.item3_name, fitr.item4_name, fitr.item5_name, 
				mss.summoner1_id, mss.summoner2_id, 
				pr.primary_perk0, pr.primary_perk1, pr.primary_perk2, pr.primary_perk3, 
				pr.sub_perk0, pr.sub_perk1, pr.stat_perk_defense, pr.stat_perk_flex, pr.stat_perk_offense
			FROM RankedMetas rm
			LEFT JOIN FullItemTreeRanks fitr ON rm.champion_id = fitr.champion_id 
				AND rm.champion_name = fitr.champion_name 
				AND rm.team_position = fitr.team_position 
				AND rm.primary_style = fitr.primary_style 
				AND rm.sub_style = fitr.sub_style 
				AND rm.item0_id = fitr.item0_id 
				AND rm.item1_id = fitr.item1_id 
				AND rm.item2_id = fitr.item2_id
				AND fitr.item_combo_rank = 1
			LEFT JOIN SummonerSpellRanks mss ON rm.champion_id = mss.champion_id 
				AND rm.team_position = mss.team_position 
				AND rm.primary_style = mss.primary_style 
				AND rm.sub_style = mss.sub_style 
				AND mss.spell_rank = 1
			LEFT JOIN PerkRanks pr ON rm.champion_id = pr.champion_id 
				AND rm.team_position = pr.team_position 
				AND rm.primary_style = pr.primary_style 
				AND rm.sub_style = pr.sub_style AND pr.perk_rank = 1;`,
	}
	counterSqls := []string{
		`CREATE TEMPORARY TABLE IF NOT EXISTS CounterGroup AS
			SELECT mg.match_id,
				   mg.team_position,
				   mg.champion_id,
				   mg.champion_name,
				   mg.summoner1_id,
				   mg.summoner2_id,
				   mg.primary_style,
				   mg.primary_perk0,
				   mg.primary_perk1,
				   mg.primary_perk2,
				   mg.primary_perk3,
				   mg.sub_style,
				   mg.sub_perk0,
				   mg.sub_perk1,
				   mg.stat_perk_defense,
				   mg.stat_perk_flex,
				   mg.stat_perk_offense,
				   mg.item0_id,
				   mg.item1_id,
				   mg.item2_id,
				   mg.item3_id,
				   mg.item4_id,
				   mg.item5_id,
				   mg.item0_name,
				   mg.item1_name,
				   mg.item2_name,
				   mg.item3_name,
				   mg.item4_name,
				   mg.item5_name,
				   mg.kills AS my_kills,
				   mg.deaths AS my_deaths,
				   mg.assists AS my_assists,
				   mg.win AS my_win,
				   emp.champion_id AS enemy_champion_id,
				   emp.champion_name AS enemy_champion_name,
				   emp.kills AS enemy_kills,
				   emp.deaths AS enemy_deaths,
				   emp.assists AS enemy_assists,
				   emp.win AS enemy_win
			FROM MainGroup mg
			LEFT JOIN RecentParticipants emp ON mg.match_id = emp.match_id
											 AND mg.team_id != emp.team_id
											 AND mg.team_position = emp.team_position
			WHERE emp.team_position != ''
			  AND emp.team_position IS NOT NULL;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS CounterSummonerSpellCounts AS
			SELECT champion_id, enemy_champion_id, team_position, primary_style, sub_style, summoner1_id, summoner2_id, COUNT(*) AS count, AVG(my_win) AS win_rate
			FROM CounterGroup cg
			GROUP BY champion_id, enemy_champion_id, team_position, primary_style, sub_style, summoner1_id, summoner2_id;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS CounterSummonerSpellRanks AS
			SELECT *, ROW_NUMBER() OVER (PARTITION BY champion_id, enemy_champion_id, team_position ORDER BY count DESC, win_rate DESC) AS spell_rank
			FROM CounterSummonerSpellCounts;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS CounterPerkCounts AS
			SELECT champion_id, enemy_champion_id, team_position, primary_style, sub_style, primary_perk0, primary_perk1, primary_perk2, primary_perk3,
				   sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense, COUNT(*) AS count, AVG(my_win) AS win_rate
			FROM CounterGroup
			GROUP BY champion_id, enemy_champion_id, team_position, primary_style, sub_style, primary_perk0, primary_perk1, primary_perk2, primary_perk3,
					 sub_perk0, sub_perk1, stat_perk_defense, stat_perk_flex, stat_perk_offense;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS CounterPerkRanks AS
			SELECT *, ROW_NUMBER() OVER (PARTITION BY champion_id, enemy_champion_id, team_position ORDER BY count DESC, win_rate DESC) AS perk_rank
			FROM CounterPerkCounts;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS CounterFullItemTreeGroups AS
			SELECT champion_id, enemy_champion_id, champion_name, team_position, primary_style, sub_style,
				   item0_id, item1_id, item2_id, item3_id, item4_id, item5_id,
				   item0_name, item1_name, item2_name, item3_name, item4_name, item5_name,
				   IF(item0_id IS NOT NULL, 1, 0)
					   + IF(item1_id IS NOT NULL, 1, 0)
					   + IF(item2_id IS NOT NULL, 1, 0)
					   + IF(item3_id IS NOT NULL, 1, 0)
					   + IF(item4_id IS NOT NULL, 1, 0)
					   + IF(item5_id IS NOT NULL, 1, 0)
				   AS item_count,
			        AVG(my_win) AS win_rate,
				   COUNT(*) AS full_item_tree_count
			FROM CounterGroup
			WHERE item0_id IS NOT NULL AND item1_id IS NOT NULL AND item2_id IS NOT NULL
			GROUP BY champion_id, enemy_champion_id, champion_name, team_position, primary_style, sub_style,
					 item0_id, item1_id, item2_id, item3_id, item4_id, item5_id,
					 item0_name, item1_name, item2_name, item3_name, item4_name, item5_name;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS CounterFullItemTreeRanks AS
			SELECT *, ROW_NUMBER() OVER (PARTITION BY champion_id, enemy_champion_id, team_position ORDER BY item_count DESC, full_item_tree_count DESC) AS item_combo_rank
			FROM CounterFullItemTreeGroups;`,
		`CREATE TEMPORARY TABLE IF NOT EXISTS CounterMetaGroup AS
			SELECT cg.champion_id,
				   cg.champion_name,
				   cg.team_position,
				   cg.enemy_champion_id,
				   cg.enemy_champion_name,
				   AVG(cg.my_kills) AS avg_kills,
				   AVG(cg.my_deaths) AS avg_deaths,
				   AVG(cg.my_assists) AS avg_assists,
				   SUM(cg.my_win) AS wins,
				   AVG(cg.my_win) AS win_rate,
				   AVG(cg.enemy_kills) AS avg_enemy_kills,
				   AVG(cg.enemy_deaths) AS avg_enemy_deaths,
				   AVG(cg.enemy_assists) AS avg_enemy_assists,
				   SUM(cg.enemy_win) AS enemy_wins,
				   AVG(cg.enemy_win) AS enemy_win_rate,
				   COUNT(DISTINCT cg.match_id) AS total
			FROM CounterGroup cg
			GROUP BY cg.champion_id, cg.champion_name, cg.team_position, cg.enemy_champion_id, cg.enemy_champion_name;`,
	}

	totalSqls := append(commonSqls, append(metaSqls, counterSqls...)...)
	index := 0
	for _, totalSql := range totalSqls {
		splited := strings.Split(totalSql, " ")
		tableName := "unknown"
		if len(splited) > 6 {
			tableName = splited[6]
		}

		if _, err := db.Exec(totalSql); err != nil {
			log.Errorf("error occurred while creating %s", tableName)
			log.Error(err)
			return err
		}

		index += 1
		log.Debugf("champion detail statistics meta CTT process: %d/%d (%s) complete", index, len(totalSqls), tableName)
	}

	return nil
}

func DropTemporaryTables(db db.Context) error {
	droppingTables := []string{
		"RecentParticipants",
		"RecentPerkStyles",
		"SortedPerks",
		"PerkGroups",
		"PerkStyleGroups",
		"ItemDetails",
		"MainGroup",
		"SummonerSpellCounts",
		"SummonerSpellRanks",
		"PerkCounts",
		"PerkRanks",
		"FullItemTreeGroups",
		"FullItemTreeRanks",
		"RefinedMetaGroups",
		"RankedMetas",
		"FinalRankedMetas",
		"CounterGroup",
		"CounterSummonerSpellCounts",
		"CounterSummonerSpellRanks",
		"CounterPerkCounts",
		"CounterPerkRanks",
		"CounterFullItemTreeGroups",
		"CounterFullItemTreeRanks",
		"CounterMetaGroup",
	}

	dropped := 0
	for _, tableName := range droppingTables {
		result, err := db.Exec("DROP TEMPORARY TABLE IF EXISTS " + tableName + ";")
		if err != nil {
			log.Errorf("error occurred while dropping %s", tableName)
			log.Error(err)
			return err
		}
		affected, _ := result.RowsAffected()
		if affected > 0 {
			dropped += 1
		}
	}

	log.Debugf("dropped %d temporary tables", len(droppingTables))
	return nil
}

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

func GetChampionDetailStatisticsMetaMXDAOs(db db.Context) ([]ChampionDetailStatisticsMetaMXDAO, error) {
	var result []ChampionDetailStatisticsMetaMXDAO
	if err := db.Select(&result, `
		SELECT *
		FROM FinalRankedMetas
		WHERE meta_rank <= 15 OR (win_rate > 0.5 AND total >= 50)
		ORDER BY champion_name ASC, team_position ASC, meta_rank ASC;
	`); err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			result = make([]ChampionDetailStatisticsMetaMXDAO, 0)
		} else {
			return nil, err
		}
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

func GetChampionCounterStatisticsMXDAOs(db db.Context) ([]ChampionCounterStatisticsMXDAO, error) {
	var result []ChampionCounterStatisticsMXDAO
	if err := db.Select(&result, `
		SELECT cmg.champion_id,
			   cmg.champion_name,
			   cmg.team_position,
			   cmg.enemy_champion_id,
			   cmg.enemy_champion_name,
			   cmg.total,
			   cmg.avg_kills,
			   cmg.avg_deaths,
			   cmg.avg_assists,
			   cmg.wins,
			   cmg.win_rate,
			   cmg.avg_enemy_kills,
			   cmg.avg_enemy_deaths,
			   cmg.avg_enemy_assists,
			   cmg.enemy_wins,
			   cmg.enemy_win_rate,
			   (cssr.win_rate + cpr.win_rate + cfitr.win_rate) / 3 AS total_win_rate,
			   cssr.summoner1_id,
			   cssr.summoner2_id,
			   cpr.primary_style,
			   cpr.primary_perk0,
			   cpr.primary_perk1,
			   cpr.primary_perk2,
			   cpr.primary_perk3,
			   cpr.sub_style,
			   cpr.sub_perk0,
			   cpr.sub_perk1,
			   cpr.stat_perk_defense,
			   cpr.stat_perk_flex,
			   cpr.stat_perk_offense,
			   cfitr.item0_id,
			   cfitr.item1_id,
			   cfitr.item2_id,
			   cfitr.item3_id,
			   cfitr.item4_id,
			   cfitr.item5_id
		FROM CounterMetaGroup cmg
		LEFT JOIN CounterFullItemTreeRanks cfitr ON cmg.champion_id = cfitr.champion_id
												AND cmg.enemy_champion_id = cfitr.enemy_champion_id
												AND cmg.team_position = cfitr.team_position
												AND cfitr.item_combo_rank = 1
		LEFT JOIN CounterSummonerSpellRanks cssr ON cmg.champion_id = cssr.champion_id
						AND cmg.enemy_champion_id = cssr.enemy_champion_id
						AND cmg.team_position = cssr.team_position
						AND cssr.spell_rank = 1
		LEFT JOIN CounterPerkRanks cpr ON cmg.champion_id = cpr.champion_id
									  AND cmg.enemy_champion_id = cpr.enemy_champion_id
									  AND cmg.team_position = cpr.team_position
									  AND cpr.perk_rank = 1
	`); err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			result = make([]ChampionCounterStatisticsMXDAO, 0)
		} else {
			return nil, err
		}
	}

	return result, nil
}
