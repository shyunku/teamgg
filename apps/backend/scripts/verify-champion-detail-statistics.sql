-- Task #62 production verification. Capture this output immediately before and
-- after one Champion Detail statistics collection.

SELECT
    table_rows AS estimated_rows,
    data_length,
    index_length,
    data_length + index_length AS total_bytes
FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND table_name = 'champion_detail_statistics_source';

SELECT COUNT(*) AS exact_source_rows
FROM champion_detail_statistics_source;

SELECT team_position, COUNT(*) AS participants
FROM champion_detail_statistics_source
GROUP BY team_position
ORDER BY team_position;

SHOW GLOBAL STATUS
WHERE Variable_name IN ('Created_tmp_tables', 'Created_tmp_disk_tables');

SELECT
    COUNT(*) AS file_count,
    COALESCE(SUM(total_extents * extent_size), 0) AS allocated_bytes,
    COALESCE(SUM(free_extents * extent_size), 0) AS free_bytes
FROM information_schema.files
WHERE tablespace_name = 'innodb_temporary'
   OR tablespace_name LIKE 'innodb_temporary_%'
   OR file_name LIKE '%#innodb_temp%';

-- Replace these versions with the versions printed by the collection log.
EXPLAIN FORMAT=JSON
WITH RecentMatches AS (
    SELECT match_id
    FROM matches FORCE INDEX (matches_game_version_index)
    WHERE game_version IN ('16.16.1', '16.15.1', '16.14.1')
)
SELECT COUNT(*)
FROM RecentMatches recent
INNER JOIN match_participants participant
    ON participant.match_id = recent.match_id
WHERE participant.team_position != '';

EXPLAIN FORMAT=JSON
SELECT champion_id, team_position, primary_style, sub_style, COUNT(*)
FROM champion_detail_statistics_source
GROUP BY champion_id, team_position, primary_style, sub_style;
