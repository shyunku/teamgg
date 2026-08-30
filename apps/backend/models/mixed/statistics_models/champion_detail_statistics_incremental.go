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

const (
	championDetailParticipantSourceTable = "champion_detail_statistics_participants"
	championDetailBanSourceTable         = "champion_detail_statistics_bans"
	championDetailProcessedMatchTable    = "champion_detail_statistics_processed_matches"
	championDetailProgressTable          = "champion_detail_statistics_progress"
)

type ChampionDetailSourceOptions struct {
	BatchSize   int
	WorkLimit   time.Duration
	CleanupSize int
}

type ChampionDetailSourcePreparation struct {
	Ready            bool
	ProcessedBatches int
	ProcessedMatches int64
	ParticipantRows  int64
	BanRows          int64
}

type championDetailProgress struct {
	LastMatchId      string `db:"last_match_id"`
	ProcessedMatches int64  `db:"processed_matches"`
}

const populateChampionDetailParticipantBatchQuery = `
	INSERT INTO champion_detail_statistics_participants (
		game_version, match_id, match_participant_id, game_duration,
		champion_id, champion_name, team_position, kills, deaths, assists,
		team_id, summoner1_id, summoner2_id, win,
		total_minions_killed, gold_earned, total_damage_dealt_to_champions,
		total_damage_taken, total_heal, vision_score, total_time_cc_dealt,
		damage_self_mitigated, damage_dealt_to_buildings,
		damage_dealt_to_objectives, damage_dealt_to_turrets,
		enemy_champion_id, enemy_champion_name, enemy_kills, enemy_deaths,
		enemy_assists, enemy_win, build_valid,
		primary_style, primary_perk0, primary_perk1, primary_perk2, primary_perk3,
		sub_style, sub_perk0, sub_perk1,
		stat_perk_defense, stat_perk_flex, stat_perk_offense,
		item0_id, item0_name, item1_id, item1_name, item2_id, item2_name,
		item3_id, item3_name, item4_id, item4_name, item5_id, item5_name
	)
	WITH BoundMatches AS (
		SELECT match_id, game_version, game_duration
		FROM matches FORCE INDEX (PRIMARY)
		WHERE game_version = ? AND match_id IN (?)
	), BoundParticipants AS (
		SELECT
			m.game_version, m.game_duration,
			mp.match_id, mp.match_participant_id, mp.champion_id, mp.champion_name,
			mp.team_position, mp.kills, mp.deaths, mp.assists, mp.team_id,
			mp.summoner1_id, mp.summoner2_id, mp.win,
			mp.total_minions_killed, mp.gold_earned,
			mp.total_damage_dealt_to_champions, mp.total_damage_taken,
			mp.total_heal, mp.vision_score, mp.total_time_cc_dealt,
			mp.item0, mp.item1, mp.item2, mp.item3, mp.item4, mp.item5, mp.item6,
			COALESCE(detail.damage_self_mitigated, 0) AS damage_self_mitigated,
			COALESCE(detail.damage_dealt_to_buildings, 0) AS damage_dealt_to_buildings,
			COALESCE(detail.damage_dealt_to_objectives, 0) AS damage_dealt_to_objectives,
			COALESCE(detail.damage_dealt_to_turrets, 0) AS damage_dealt_to_turrets
		FROM BoundMatches m
		INNER JOIN match_participants mp ON mp.match_id = m.match_id
		LEFT JOIN match_participant_details detail
			ON detail.match_id = mp.match_id
			AND detail.match_participant_id = mp.match_participant_id
		WHERE mp.team_position != ''
	), ParticipantOpponents AS (
		SELECT
			participant.*,
			enemy.champion_id AS enemy_champion_id,
			enemy.champion_name AS enemy_champion_name,
			enemy.kills AS enemy_kills,
			enemy.deaths AS enemy_deaths,
			enemy.assists AS enemy_assists,
			enemy.win AS enemy_win
		FROM BoundParticipants participant
		LEFT JOIN BoundParticipants enemy
			ON enemy.match_id = participant.match_id
			AND enemy.team_id != participant.team_id
			AND enemy.team_position = participant.team_position
	), RankedPerkSelections AS (
		SELECT
			style.match_participant_id, style.style_id, style.description, style.style,
			selection.perk,
			ROW_NUMBER() OVER (
				PARTITION BY style.style_id
				ORDER BY selection.perk DESC
			) AS perk_rank
		FROM BoundParticipants participant
		INNER JOIN match_participant_perk_styles style
			ON style.match_participant_id = participant.match_participant_id
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
			participant.match_participant_id,
			item.id AS item_id,
			item.name AS item_name,
			ROW_NUMBER() OVER (
				PARTITION BY participant.match_participant_id
				ORDER BY item.depth DESC, item.gold_total DESC, item.id DESC
			) AS item_rank
		FROM BoundParticipants participant
		INNER JOIN static_items item
			ON item.id IN (
				participant.item0, participant.item1, participant.item2, participant.item3,
				participant.item4, participant.item5, participant.item6
			)
		WHERE item.id != 0
			AND item.required_ally IS NULL
			AND item.gold_purchasable IS TRUE
			AND item.gold_total > 0
			AND item.depth >= 3
	)
	SELECT
		participant.game_version, participant.match_id, participant.match_participant_id,
		participant.game_duration, participant.champion_id, participant.champion_name,
		participant.team_position, participant.kills, participant.deaths, participant.assists,
		participant.team_id, participant.summoner1_id, participant.summoner2_id, participant.win,
		participant.total_minions_killed, participant.gold_earned,
		participant.total_damage_dealt_to_champions, participant.total_damage_taken,
		participant.total_heal, participant.vision_score, participant.total_time_cc_dealt,
		participant.damage_self_mitigated, participant.damage_dealt_to_buildings,
		participant.damage_dealt_to_objectives, participant.damage_dealt_to_turrets,
		participant.enemy_champion_id, participant.enemy_champion_name,
		participant.enemy_kills, participant.enemy_deaths, participant.enemy_assists, participant.enemy_win,
		CASE WHEN perks.primary_style IS NOT NULL AND perks.primary_style != 0
			AND perks.primary_perk0 IS NOT NULL AND perks.primary_perk1 IS NOT NULL
			AND perks.primary_perk2 IS NOT NULL AND perks.primary_perk3 IS NOT NULL
			AND perks.sub_style IS NOT NULL AND perks.sub_style != 0
			AND perks.sub_perk0 IS NOT NULL AND perks.sub_perk1 IS NOT NULL
			AND perks.stat_perk_defense IS NOT NULL AND perks.stat_perk_flex IS NOT NULL
			AND perks.stat_perk_offense IS NOT NULL AND COUNT(item.item_id) > 0
			THEN 1 ELSE 0 END AS build_valid,
		perks.primary_style, perks.primary_perk0, perks.primary_perk1,
		perks.primary_perk2, perks.primary_perk3,
		perks.sub_style, perks.sub_perk0, perks.sub_perk1,
		perks.stat_perk_defense, perks.stat_perk_flex, perks.stat_perk_offense,
		MAX(CASE WHEN item.item_rank = 1 THEN item.item_id END),
		MAX(CASE WHEN item.item_rank = 1 THEN item.item_name END),
		MAX(CASE WHEN item.item_rank = 2 THEN item.item_id END),
		MAX(CASE WHEN item.item_rank = 2 THEN item.item_name END),
		MAX(CASE WHEN item.item_rank = 3 THEN item.item_id END),
		MAX(CASE WHEN item.item_rank = 3 THEN item.item_name END),
		MAX(CASE WHEN item.item_rank = 4 THEN item.item_id END),
		MAX(CASE WHEN item.item_rank = 4 THEN item.item_name END),
		MAX(CASE WHEN item.item_rank = 5 THEN item.item_id END),
		MAX(CASE WHEN item.item_rank = 5 THEN item.item_name END),
		MAX(CASE WHEN item.item_rank = 6 THEN item.item_id END),
		MAX(CASE WHEN item.item_rank = 6 THEN item.item_name END)
	FROM ParticipantOpponents participant
	LEFT JOIN ParticipantPerks perks
		ON perks.match_participant_id = participant.match_participant_id
	LEFT JOIN RankedItems item
		ON item.match_participant_id = participant.match_participant_id
	GROUP BY
		participant.game_version, participant.match_id, participant.match_participant_id,
		participant.game_duration, participant.champion_id, participant.champion_name,
		participant.team_position, participant.kills, participant.deaths, participant.assists,
		participant.team_id, participant.summoner1_id, participant.summoner2_id, participant.win,
		participant.total_minions_killed, participant.gold_earned,
		participant.total_damage_dealt_to_champions, participant.total_damage_taken,
		participant.total_heal, participant.vision_score, participant.total_time_cc_dealt,
		participant.damage_self_mitigated, participant.damage_dealt_to_buildings,
		participant.damage_dealt_to_objectives, participant.damage_dealt_to_turrets,
		participant.enemy_champion_id, participant.enemy_champion_name,
		participant.enemy_kills, participant.enemy_deaths, participant.enemy_assists, participant.enemy_win,
		perks.primary_style, perks.primary_perk0, perks.primary_perk1,
		perks.primary_perk2, perks.primary_perk3,
		perks.sub_style, perks.sub_perk0, perks.sub_perk1,
		perks.stat_perk_defense, perks.stat_perk_flex, perks.stat_perk_offense
	ON DUPLICATE KEY UPDATE
		game_version = VALUES(game_version), match_id = VALUES(match_id),
		game_duration = VALUES(game_duration), champion_id = VALUES(champion_id),
		champion_name = VALUES(champion_name), team_position = VALUES(team_position),
		kills = VALUES(kills), deaths = VALUES(deaths), assists = VALUES(assists),
		team_id = VALUES(team_id), summoner1_id = VALUES(summoner1_id),
		summoner2_id = VALUES(summoner2_id), win = VALUES(win),
		total_minions_killed = VALUES(total_minions_killed), gold_earned = VALUES(gold_earned),
		total_damage_dealt_to_champions = VALUES(total_damage_dealt_to_champions),
		total_damage_taken = VALUES(total_damage_taken), total_heal = VALUES(total_heal),
		vision_score = VALUES(vision_score), total_time_cc_dealt = VALUES(total_time_cc_dealt),
		damage_self_mitigated = VALUES(damage_self_mitigated),
		damage_dealt_to_buildings = VALUES(damage_dealt_to_buildings),
		damage_dealt_to_objectives = VALUES(damage_dealt_to_objectives),
		damage_dealt_to_turrets = VALUES(damage_dealt_to_turrets),
		enemy_champion_id = VALUES(enemy_champion_id), enemy_champion_name = VALUES(enemy_champion_name),
		enemy_kills = VALUES(enemy_kills), enemy_deaths = VALUES(enemy_deaths),
		enemy_assists = VALUES(enemy_assists), enemy_win = VALUES(enemy_win),
		build_valid = VALUES(build_valid), primary_style = VALUES(primary_style),
		primary_perk0 = VALUES(primary_perk0), primary_perk1 = VALUES(primary_perk1),
		primary_perk2 = VALUES(primary_perk2), primary_perk3 = VALUES(primary_perk3),
		sub_style = VALUES(sub_style), sub_perk0 = VALUES(sub_perk0), sub_perk1 = VALUES(sub_perk1),
		stat_perk_defense = VALUES(stat_perk_defense), stat_perk_flex = VALUES(stat_perk_flex),
		stat_perk_offense = VALUES(stat_perk_offense),
		item0_id = VALUES(item0_id), item0_name = VALUES(item0_name),
		item1_id = VALUES(item1_id), item1_name = VALUES(item1_name),
		item2_id = VALUES(item2_id), item2_name = VALUES(item2_name),
		item3_id = VALUES(item3_id), item3_name = VALUES(item3_name),
		item4_id = VALUES(item4_id), item4_name = VALUES(item4_name),
		item5_id = VALUES(item5_id), item5_name = VALUES(item5_name)
`

const populateChampionDetailBanBatchQuery = `
	INSERT INTO champion_detail_statistics_bans
		(game_version, match_id, team_id, champion_id, pick_turn)
	SELECT match_row.game_version, ban.match_id, ban.team_id, ban.champion_id, ban.pick_turn
	FROM matches match_row FORCE INDEX (PRIMARY)
	INNER JOIN match_team_bans ban ON ban.match_id = match_row.match_id
	WHERE match_row.game_version = ? AND match_row.match_id IN (?)
	ON DUPLICATE KEY UPDATE
		game_version = VALUES(game_version), champion_id = VALUES(champion_id)
`

const markChampionDetailProcessedMatchesQuery = `
	INSERT IGNORE INTO champion_detail_statistics_processed_matches (game_version, match_id)
	SELECT game_version, match_id
	FROM matches FORCE INDEX (PRIMARY)
	WHERE game_version = ? AND match_id IN (?)
`

func PrepareIncrementalChampionDetailStatisticsSource(
	database db.Context,
	matchGameVersions []string,
	options ChampionDetailSourceOptions,
) (ChampionDetailSourcePreparation, error) {
	result := ChampionDetailSourcePreparation{}
	if len(matchGameVersions) == 0 {
		return result, errors.New("match game versions are required")
	}
	if options.BatchSize <= 0 || options.CleanupSize <= 0 || options.WorkLimit <= 0 {
		return result, errors.New("champion detail source options must be greater than zero")
	}
	started := time.Now()
	deadline := started.Add(options.WorkLimit)

	cleanupReady, err := cleanupChampionDetailStatisticsSources(database, matchGameVersions, options.CleanupSize, deadline)
	if err != nil {
		return result, err
	}
	if !cleanupReady {
		log.Infof("champion detail incremental source cleanup paused after %s", time.Since(started))
		return result, nil
	}

	for _, version := range matchGameVersions {
		progress := championDetailProgress{}
		err := database.Get(&progress, `
			SELECT last_match_id, processed_matches
			FROM champion_detail_statistics_progress
			WHERE game_version = ?
		`, version)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return result, fmt.Errorf("load champion detail source progress for %s: %w", version, err)
		}

		for {
			if time.Now().After(deadline) {
				log.Infof(
					"champion detail incremental source paused: version=%s cursor=%s batches=%d matches=%d duration=%s",
					version, progress.LastMatchId, result.ProcessedBatches, result.ProcessedMatches, time.Since(started),
				)
				return result, nil
			}
			matchIds := make([]string, 0, options.BatchSize)
			if err := database.Select(&matchIds, `
				SELECT match_row.match_id
				FROM matches match_row FORCE INDEX (matches_game_version_index)
				WHERE match_row.game_version = ? AND match_row.match_id > ?
					AND NOT EXISTS (
						SELECT 1 FROM champion_detail_statistics_processed_matches processed
						WHERE processed.game_version = ? AND processed.match_id = match_row.match_id
					)
				ORDER BY match_row.match_id
				LIMIT ?
			`, version, progress.LastMatchId, version, options.BatchSize); err != nil {
				return result, fmt.Errorf("select champion detail source batch for %s: %w", version, err)
			}
			if len(matchIds) == 0 {
				if err := database.Select(&matchIds, `
					SELECT match_row.match_id
					FROM matches match_row FORCE INDEX (matches_game_version_index)
					WHERE match_row.game_version = ?
						AND NOT EXISTS (
							SELECT 1 FROM champion_detail_statistics_processed_matches processed
							WHERE processed.game_version = ? AND processed.match_id = match_row.match_id
						)
					ORDER BY match_row.match_id
					LIMIT ?
				`, version, version, options.BatchSize); err != nil {
					return result, fmt.Errorf("select missing champion detail source batch for %s: %w", version, err)
				}
			}
			if len(matchIds) == 0 {
				if _, err := database.Exec(`
					INSERT INTO champion_detail_statistics_progress
						(game_version, last_match_id, processed_matches, completed)
					VALUES (?, ?, ?, 1)
					ON DUPLICATE KEY UPDATE completed = 1, updated_at = CURRENT_TIMESTAMP(6)
				`, version, progress.LastMatchId, progress.ProcessedMatches); err != nil {
					return result, fmt.Errorf("complete champion detail source progress for %s: %w", version, err)
				}
				break
			}

			if err := populateChampionDetailSourceBatch(database, version, matchIds); err != nil {
				return result, err
			}
			lastMatchId := matchIds[len(matchIds)-1]
			if lastMatchId > progress.LastMatchId {
				progress.LastMatchId = lastMatchId
			}
			progress.ProcessedMatches += int64(len(matchIds))
			if _, err := database.Exec(`
				INSERT INTO champion_detail_statistics_progress
					(game_version, last_match_id, processed_matches, completed)
				VALUES (?, ?, ?, 0)
				ON DUPLICATE KEY UPDATE
					last_match_id = VALUES(last_match_id),
					processed_matches = VALUES(processed_matches),
					completed = 0,
					updated_at = CURRENT_TIMESTAMP(6)
			`, version, progress.LastMatchId, progress.ProcessedMatches); err != nil {
				return result, fmt.Errorf("save champion detail source progress for %s: %w", version, err)
			}
			result.ProcessedBatches++
			result.ProcessedMatches += int64(len(matchIds))
		}
	}

	if err := database.Get(&result.ParticipantRows, `
		SELECT COUNT(*) FROM champion_detail_statistics_participants
	`); err != nil {
		return result, fmt.Errorf("count champion detail participant source: %w", err)
	}
	if err := database.Get(&result.BanRows, `
		SELECT COUNT(*) FROM champion_detail_statistics_bans
	`); err != nil {
		return result, fmt.Errorf("count champion detail ban source: %w", err)
	}
	result.Ready = true
	log.Infof(
		"champion detail incremental source ready: patches=%d rows=%d bans=%d batches=%d matches=%d duration=%s",
		len(matchGameVersions), result.ParticipantRows, result.BanRows,
		result.ProcessedBatches, result.ProcessedMatches, time.Since(started),
	)
	return result, nil
}

func populateChampionDetailSourceBatch(database db.Context, version string, matchIds []string) error {
	participantQuery, participantArgs, err := sqlx.In(
		populateChampionDetailParticipantBatchQuery, version, matchIds,
	)
	if err != nil {
		return fmt.Errorf("bind champion detail participant batch for %s: %w", version, err)
	}
	if _, err := database.Exec(database.Rebind(participantQuery), participantArgs...); err != nil {
		return fmt.Errorf("populate champion detail participant batch for %s: %w", version, err)
	}
	banQuery, banArgs, err := sqlx.In(populateChampionDetailBanBatchQuery, version, matchIds)
	if err != nil {
		return fmt.Errorf("bind champion detail ban batch for %s: %w", version, err)
	}
	if _, err := database.Exec(database.Rebind(banQuery), banArgs...); err != nil {
		return fmt.Errorf("populate champion detail ban batch for %s: %w", version, err)
	}
	processedQuery, processedArgs, err := sqlx.In(markChampionDetailProcessedMatchesQuery, version, matchIds)
	if err != nil {
		return fmt.Errorf("bind champion detail processed matches for %s: %w", version, err)
	}
	if _, err := database.Exec(database.Rebind(processedQuery), processedArgs...); err != nil {
		return fmt.Errorf("mark champion detail matches processed for %s: %w", version, err)
	}
	return nil
}

func cleanupChampionDetailStatisticsSources(
	database db.Context,
	matchGameVersions []string,
	batchSize int,
	deadline time.Time,
) (bool, error) {
	tables := []string{
		championDetailParticipantSourceTable,
		championDetailBanSourceTable,
		championDetailProcessedMatchTable,
	}
	for _, table := range tables {
		for {
			if time.Now().After(deadline) {
				return false, nil
			}
			query, args, err := sqlx.In(
				"DELETE FROM "+table+" WHERE game_version NOT IN (?) LIMIT ?",
				matchGameVersions, batchSize,
			)
			if err != nil {
				return false, fmt.Errorf("bind champion detail cleanup for %s: %w", table, err)
			}
			result, err := database.Exec(database.Rebind(query), args...)
			if err != nil {
				return false, fmt.Errorf("clean champion detail source %s: %w", table, err)
			}
			rows, err := result.RowsAffected()
			if err != nil {
				return false, fmt.Errorf("read champion detail cleanup result for %s: %w", table, err)
			}
			if rows < int64(batchSize) {
				break
			}
		}
	}
	query, args, err := sqlx.In(
		"DELETE FROM "+championDetailProgressTable+" WHERE game_version NOT IN (?)",
		matchGameVersions,
	)
	if err != nil {
		return false, fmt.Errorf("bind champion detail progress cleanup: %w", err)
	}
	if _, err := database.Exec(database.Rebind(query), args...); err != nil {
		return false, fmt.Errorf("clean champion detail progress: %w", err)
	}
	return true, nil
}
