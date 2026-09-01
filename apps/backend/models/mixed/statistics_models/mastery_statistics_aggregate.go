package statistics_models

import (
	"fmt"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"time"
)

const maxMasteryStatisticsBatch = 1000

type MasteryStatisticsRefreshResult struct {
	DirtyChampions int
	Refreshed      int
	Removed        int
	Duration       time.Duration
}

func RefreshDirtyMasteryStatisticsAggregates(database db.Context, limit int) (*MasteryStatisticsRefreshResult, error) {
	started := time.Now()
	if limit < 1 || limit > maxMasteryStatisticsBatch {
		limit = maxMasteryStatisticsBatch
	}

	var cutoff time.Time
	if err := database.Get(&cutoff, "SELECT NOW(6)"); err != nil {
		return nil, fmt.Errorf("load mastery statistics cutoff: %w", err)
	}

	championIds := make([]int, 0)
	if err := database.Select(&championIds, `
		SELECT champion_id
		FROM mastery_statistics_dirty_champions
		WHERE dirty_at < ?
		ORDER BY dirty_at, champion_id
		LIMIT ?
	`, cutoff, limit); err != nil {
		return nil, fmt.Errorf("load dirty mastery champions: %w", err)
	}

	result := &MasteryStatisticsRefreshResult{DirtyChampions: len(championIds)}
	for _, championId := range championIds {
		var aggregate MasteryStatisticsMXDAO
		aggregateQuery := `
			SELECT
				? AS champion_id,
				COALESCE(MAX(champion_points), 0) AS max_mastery,
				0 AS avg_mastery,
				COALESCE(SUM(champion_points), 0) AS total_mastery,
				COALESCE(SUM(IF(champion_level >= 7, 1, 0)), 0) AS mastered_count,
				COUNT(*) AS count
			FROM masteries FORCE INDEX (masteries_champion_points_level_covering_index)
			WHERE champion_id = ?
		`
		if models.MasteryNumericV2ReadsEnabled() {
			aggregateQuery = `
				SELECT
					? AS champion_id,
					COALESCE(MAX(champion_points), 0) AS max_mastery,
					0 AS avg_mastery,
					COALESCE(SUM(champion_points), 0) AS total_mastery,
					COALESCE(SUM(IF(champion_level >= 7, 1, 0)), 0) AS mastered_count,
					COUNT(*) AS count
				FROM masteries_numeric_v2
					FORCE INDEX (masteries_numeric_champion_points_level_covering_index)
				WHERE champion_id = ?
			`
		}
		if err := database.Get(&aggregate, aggregateQuery, championId, championId); err != nil {
			return nil, fmt.Errorf("aggregate mastery champion %d: %w", championId, err)
		}

		if aggregate.Count == 0 {
			if _, err := database.Exec("DELETE FROM mastery_statistics_aggregates WHERE champion_id = ?", championId); err != nil {
				return nil, fmt.Errorf("remove empty mastery aggregate %d: %w", championId, err)
			}
			result.Removed++
		} else {
			if _, err := database.Exec(`
				INSERT INTO mastery_statistics_aggregates
					(champion_id, max_mastery, total_mastery, mastered_count, summoner_count, updated_at)
				VALUES (?, ?, ?, ?, ?, NOW(6))
				ON DUPLICATE KEY UPDATE
					max_mastery = VALUES(max_mastery),
					total_mastery = VALUES(total_mastery),
					mastered_count = VALUES(mastered_count),
					summoner_count = VALUES(summoner_count),
					updated_at = VALUES(updated_at)
			`, aggregate.ChampionId, aggregate.MaxMastery, aggregate.TotalMastery, aggregate.MasteredCount, aggregate.Count); err != nil {
				return nil, fmt.Errorf("save mastery aggregate %d: %w", championId, err)
			}
			result.Refreshed++
		}

		if _, err := database.Exec(`
			DELETE FROM mastery_statistics_dirty_champions
			WHERE champion_id = ? AND dirty_at < ?
		`, championId, cutoff); err != nil {
			return nil, fmt.Errorf("acknowledge mastery aggregate %d: %w", championId, err)
		}
	}
	result.Duration = time.Since(started)
	return result, nil
}
