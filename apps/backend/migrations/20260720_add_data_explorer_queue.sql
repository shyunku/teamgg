-- DataExplorer queue migration for MySQL 8.0+.
-- Build a deduplicated replacement because the legacy table has no row key.
-- Stop application writers before running this migration. The backfill commits
-- per seven-day match window, so it never creates one transaction containing
-- the complete history. Keep the legacy table for verification.

CREATE TABLE summoner_matches_v2
(
    puuid varchar(255) not null,
    match_id varchar(255) not null,
    primary key (puuid, match_id),
    key summoner_matches_v2_match_id_index (match_id),
    constraint summoner_matches_v2_matches_match_id_fk foreign key (match_id)
        references matches (match_id) on update cascade on delete cascade,
    constraint summoner_matches_v2_summoners_puuid_fk foreign key (puuid)
        references summoners (puuid) on update cascade on delete cascade
) ENGINE=InnoDB;

DELIMITER //
CREATE PROCEDURE backfill_summoner_matches_v2()
BEGIN
    DECLARE cursor_timestamp BIGINT DEFAULT 0;
    DECLARE maximum_timestamp BIGINT DEFAULT 0;
    DECLARE window_milliseconds BIGINT DEFAULT 604800000;

    SELECT COALESCE(MIN(game_end_timestamp), 0), COALESCE(MAX(game_end_timestamp), 0)
    INTO cursor_timestamp, maximum_timestamp
    FROM matches;

    WHILE cursor_timestamp <= maximum_timestamp DO
        INSERT IGNORE INTO summoner_matches_v2 (puuid, match_id)
        SELECT sm.puuid, sm.match_id
        FROM matches m
        INNER JOIN summoner_matches sm ON sm.match_id = m.match_id
        WHERE m.game_end_timestamp >= cursor_timestamp
          AND m.game_end_timestamp < cursor_timestamp + window_milliseconds;

        COMMIT;
        SET cursor_timestamp = cursor_timestamp + window_milliseconds;
    END WHILE;
END//
DELIMITER ;

CALL backfill_summoner_matches_v2();
DROP PROCEDURE backfill_summoner_matches_v2;

RENAME TABLE
    summoner_matches TO summoner_matches_legacy_20260720,
    summoner_matches_v2 TO summoner_matches;

CREATE TABLE IF NOT EXISTS data_explorer_summoner_jobs
(
    puuid varchar(255) not null,
    status varchar(16) not null default 'pending',
    priority int not null default 0,
    depth int not null default 0,
    attempts int not null default 0,
    next_attempt_at datetime(6) not null default current_timestamp(6),
    lease_until datetime(6) null,
    discovered_from_match_id varchar(255) null,
    last_error text null,
    created_at datetime(6) not null default current_timestamp(6),
    updated_at datetime(6) not null default current_timestamp(6) on update current_timestamp(6),
    primary key (puuid),
    key data_explorer_summoner_jobs_claim_index (status, next_attempt_at, priority, created_at),
    key data_explorer_summoner_jobs_lease_index (status, lease_until)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS data_explorer_match_jobs
(
    match_id varchar(255) not null,
    status varchar(16) not null default 'pending',
    priority int not null default 0,
    depth int not null default 0,
    attempts int not null default 0,
    next_attempt_at datetime(6) not null default current_timestamp(6),
    lease_until datetime(6) null,
    last_error text null,
    rescan_requested tinyint(1) not null default 1,
    created_at datetime(6) not null default current_timestamp(6),
    updated_at datetime(6) not null default current_timestamp(6) on update current_timestamp(6),
    primary key (match_id),
    key data_explorer_match_jobs_claim_index (status, next_attempt_at, priority, created_at),
    key data_explorer_match_jobs_lease_index (status, lease_until)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS data_explorer_match_sources
(
    match_id varchar(255) not null,
    puuid varchar(255) not null,
    created_at datetime(6) not null default current_timestamp(6),
    primary key (match_id, puuid),
    key data_explorer_match_sources_puuid_index (puuid)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS data_explorer_state
(
    state_key varchar(64) not null,
    cursor_match_id varchar(255) not null default '',
    cursor_participant_id int not null default 0,
    completed tinyint(1) not null default 0,
    updated_at datetime(6) not null default current_timestamp(6) on update current_timestamp(6),
    primary key (state_key)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS data_explorer_daily_usage
(
    usage_date date not null,
    usage_kind varchar(16) not null,
    usage_count int not null default 0,
    primary key (usage_date, usage_kind)
) ENGINE=InnoDB;

INSERT IGNORE INTO data_explorer_state
    (state_key, cursor_match_id, cursor_participant_id, completed)
VALUES ('match_participant_bootstrap', '', 0, 0);
