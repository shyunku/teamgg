-- Task #61 production verification. Run after migration and after the first
-- mastery statistics collection. Set a representative champion ID if needed.
SET @champion_id = (
    SELECT champion_id
    FROM mastery_statistics_aggregates
    ORDER BY summoner_count DESC
    LIMIT 1
);

SHOW INDEX FROM masteries_numeric_v2
WHERE Key_name = 'masteries_numeric_champion_points_level_covering_index';

SELECT trigger_name, event_object_table, event_manipulation, action_timing
FROM information_schema.triggers
WHERE trigger_schema = DATABASE()
  AND trigger_name IN (
      'teamgg_masteries_statistics_insert',
      'teamgg_masteries_statistics_update',
      'teamgg_masteries_statistics_delete'
  )
ORDER BY trigger_name;

SELECT
    (SELECT COUNT(*) FROM mastery_statistics_aggregates) AS aggregate_champions,
    (SELECT COUNT(*) FROM mastery_statistics_dirty_champions) AS dirty_champions;

EXPLAIN FORMAT=JSON
SELECT
    COALESCE(MAX(champion_points), 0),
    COALESCE(SUM(champion_points), 0),
    COALESCE(SUM(IF(champion_level >= 7, 1, 0)), 0),
    COUNT(*)
FROM masteries_numeric_v2 FORCE INDEX (masteries_numeric_champion_points_level_covering_index)
WHERE champion_id = @champion_id;

EXPLAIN FORMAT=JSON
SELECT numeric_key.puuid, mastery.champion_points
FROM masteries_numeric_v2 mastery FORCE INDEX (masteries_numeric_champion_points_level_covering_index)
INNER JOIN summoner_numeric_keys numeric_key ON numeric_key.summoner_id = mastery.summoner_fk
WHERE mastery.champion_id = @champion_id
ORDER BY mastery.champion_points DESC
LIMIT 30;

SELECT
    aggregate.champion_id,
    aggregate.max_mastery AS materialized_max,
    live.max_mastery AS live_max,
    aggregate.total_mastery AS materialized_total,
    live.total_mastery AS live_total,
    aggregate.mastered_count AS materialized_mastered,
    live.mastered_count AS live_mastered,
    aggregate.summoner_count AS materialized_summoners,
    live.summoner_count AS live_summoners,
    aggregate.max_mastery = live.max_mastery
        AND aggregate.total_mastery = live.total_mastery
        AND aggregate.mastered_count = live.mastered_count
        AND aggregate.summoner_count = live.summoner_count AS exact_match
FROM mastery_statistics_aggregates aggregate
JOIN (
    SELECT
        champion_id,
        COALESCE(MAX(champion_points), 0) AS max_mastery,
        COALESCE(SUM(champion_points), 0) AS total_mastery,
        COALESCE(SUM(IF(champion_level >= 7, 1, 0)), 0) AS mastered_count,
        COUNT(*) AS summoner_count
    FROM masteries_numeric_v2 FORCE INDEX (masteries_numeric_champion_points_level_covering_index)
    WHERE champion_id = @champion_id
    GROUP BY champion_id
) live ON live.champion_id = aggregate.champion_id
WHERE aggregate.champion_id = @champion_id;
