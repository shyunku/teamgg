package migrations

import (
	"context"

	"github.com/jmoiron/sqlx"
	"team.gg-server/models"
)

func migrationDefinitions() []Definition {
	return []Definition{
		{
			Version:  "20260719_001",
			FileName: "20260719_add_user_identities.sql",
			Apply: func(_ context.Context, database *sqlx.DB) error {
				return models.EnsureUserIdentityTable(database)
			},
			Validate: func(ctx context.Context, database *sqlx.DB) (bool, error) {
				return tablesExist(ctx, database, "user_identities")
			},
		},
		{
			Version:  "20260719_002",
			FileName: "20260719_allow_multiple_riot_identities.sql",
			Apply: func(_ context.Context, database *sqlx.DB) error {
				return models.EnsureUserIdentityTable(database)
			},
			Validate: validateMultipleRiotIdentities,
		},
		{
			Version:  "20260719_003",
			FileName: "20260719_add_riot_custom_game_preferences.sql",
			Apply: func(_ context.Context, database *sqlx.DB) error {
				return models.EnsureRiotCustomGamePreferenceTable(database)
			},
			Validate: func(ctx context.Context, database *sqlx.DB) (bool, error) {
				return tablesExist(ctx, database, "riot_custom_game_preferences")
			},
		},
		{
			Version:  "20260719_004",
			FileName: "20260719_add_custom_game_line_mastery.sql",
			Apply: func(_ context.Context, database *sqlx.DB) error {
				if err := models.EnsureRiotCustomGamePreferenceTable(database); err != nil {
					return err
				}
				return models.EnsureCustomGameLineMasterySchema(database)
			},
			Validate: validateCustomGameLineMastery,
		},
		{
			Version:  "20260720_001",
			FileName: "20260720_add_data_explorer_queue.sql",
			Apply:    applyDataExplorerQueue,
			Validate: validateDataExplorerQueue,
		},
		{
			Version:  "20260724_001",
			FileName: "20260724_add_statistics_snapshots.sql",
			Apply: func(_ context.Context, database *sqlx.DB) error {
				return models.EnsureStatisticsSnapshotSchema(database)
			},
			Validate: func(ctx context.Context, database *sqlx.DB) (bool, error) {
				return tablesExist(ctx, database, "statistics_snapshots")
			},
		},
		{
			Version:  "20260724_002",
			FileName: "20260724_add_statistics_indexes.sql",
			Apply:    applyStatisticsIndexes,
			Validate: validateStatisticsIndexes,
		},
		{
			Version:  "20260806_001",
			FileName: "20260806_add_custom_game_replay_analyses.sql",
			Apply: func(_ context.Context, database *sqlx.DB) error {
				return models.EnsureCustomGameReplayAnalysisSchema(database)
			},
			Validate: func(ctx context.Context, database *sqlx.DB) (bool, error) {
				return tablesExist(ctx, database, "custom_game_replay_analyses")
			},
		},
		{
			Version:  "20260820_001",
			FileName: "20260820_add_data_explorer_processing_state.sql",
			Apply: func(_ context.Context, database *sqlx.DB) error {
				return models.EnsureDataExplorerSchema(database)
			},
			Validate: validateDataExplorerProcessingState,
		},
		{
			Version:  "20260821_001",
			FileName: "20260821_add_data_explorer_metrics.sql",
			Apply: func(_ context.Context, database *sqlx.DB) error {
				return models.EnsureDataExplorerMetricsSchema(database)
			},
			Validate: func(ctx context.Context, database *sqlx.DB) (bool, error) {
				return tablesExist(ctx, database, "data_explorer_metrics_daily")
			},
		},
		{
			Version:  "20260823_001",
			FileName: "20260823_add_admin_operations.sql",
			Apply: func(_ context.Context, database *sqlx.DB) error {
				return models.EnsureAdminOperationsSchema(database)
			},
			Validate: func(ctx context.Context, database *sqlx.DB) (bool, error) {
				return tablesExist(ctx, database, "user_roles", "admin_audit_logs", "admin_operational_events")
			},
		},
		{
			Version:  "20260830_001",
			FileName: "20260830_add_incremental_mastery_statistics.sql",
			Apply:    applyMasteryStatisticsAggregates,
			Validate: validateMasteryStatisticsAggregates,
		},
		{
			Version:  "20260830_002",
			FileName: "20260830_create_champion_detail_statistics_source.sql",
			Apply:    applyChampionDetailStatisticsSource,
			Validate: validateChampionDetailStatisticsSource,
		},
		{
			Version:  "20260830_003",
			FileName: "20260830_create_incremental_champion_detail_statistics.sql",
			Apply:    applyIncrementalChampionDetailStatistics,
			Validate: validateIncrementalChampionDetailStatistics,
		},
		{
			Version:  "20260830_004",
			FileName: "20260830_remove_unused_indexes.sql",
			Apply:    applyUnusedIndexCleanup,
			Validate: validateUnusedIndexCleanup,
		},
	}
}

func validateMultipleRiotIdentities(ctx context.Context, database *sqlx.DB) (bool, error) {
	columns, err := columnsExist(ctx, database, map[string][]string{
		"user_identities": {"is_primary"},
	})
	if err != nil || !columns {
		return false, err
	}
	newIndex, err := indexMatches(ctx, database, "user_identities", "user_identities_provider_uid_index", "provider", "uid")
	if err != nil || !newIndex {
		return false, err
	}
	oldIndex, err := indexColumns(ctx, database, "user_identities", "user_identities_provider_uid_uindex")
	return len(oldIndex) == 0, err
}

func validateCustomGameLineMastery(ctx context.Context, database *sqlx.DB) (bool, error) {
	masteries := []string{"mastery_top", "mastery_jungle", "mastery_mid", "mastery_adc", "mastery_support"}
	return columnsExist(ctx, database, map[string][]string{
		"custom_game_configurations":   {"mastery_influence_weight"},
		"custom_game_candidates":       masteries,
		"riot_custom_game_preferences": masteries,
	})
}

func applyDataExplorerQueue(ctx context.Context, database *sqlx.DB) error {
	if err := models.EnsureDataExplorerSchema(database); err != nil {
		return err
	}
	return ensureSummonerMatchesPrimaryKey(ctx, database)
}

func validateDataExplorerQueue(ctx context.Context, database *sqlx.DB) (bool, error) {
	tables, err := tablesExist(
		ctx,
		database,
		"data_explorer_summoner_jobs",
		"data_explorer_match_jobs",
		"data_explorer_match_sources",
		"data_explorer_state",
		"data_explorer_daily_usage",
	)
	if err != nil || !tables {
		return false, err
	}
	rescan, err := columnExists(ctx, database, "data_explorer_match_jobs", "rescan_requested")
	if err != nil || !rescan {
		return false, err
	}
	return indexMatches(ctx, database, "summoner_matches", "PRIMARY", "puuid", "match_id")
}

func applyStatisticsIndexes(ctx context.Context, database *sqlx.DB) error {
	if err := ensureIndex(ctx, database, "matches", "matches_game_version_index", "game_version", "game_version"); err != nil {
		return err
	}
	return ensureIndex(
		ctx,
		database,
		"match_team_bans",
		"match_team_bans_champion_id_index",
		"champion_id",
		"champion_id",
	)
}

func validateStatisticsIndexes(ctx context.Context, database *sqlx.DB) (bool, error) {
	matches, err := indexMatches(ctx, database, "matches", "matches_game_version_index", "game_version")
	if err != nil || !matches {
		return false, err
	}
	return indexMatches(ctx, database, "match_team_bans", "match_team_bans_champion_id_index", "champion_id")
}

func validateDataExplorerProcessingState(ctx context.Context, database *sqlx.DB) (bool, error) {
	return tablesExist(
		ctx,
		database,
		"data_explorer_summoner_processing_state",
		"data_explorer_match_processing_state",
		"data_explorer_source_cleanup_state",
	)
}
