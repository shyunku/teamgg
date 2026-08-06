-- Run once during a low-traffic maintenance window on large installations.
-- These indexes support the statistics scans without changing their results.
--
-- MySQL does not support CREATE INDEX IF NOT EXISTS. Check for an existing
-- index before rerunning this file:
-- SHOW INDEX FROM matches WHERE Key_name = 'matches_game_version_index';
-- SHOW INDEX FROM match_team_bans WHERE Key_name = 'match_team_bans_champion_id_index';

CREATE INDEX matches_game_version_index
    ON matches (game_version);

CREATE INDEX match_team_bans_champion_id_index
    ON match_team_bans (champion_id);
