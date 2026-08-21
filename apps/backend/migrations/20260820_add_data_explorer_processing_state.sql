-- Persistent completion state allows completed queue rows to be removed
-- without immediately re-enqueueing the same summoner or match.

CREATE TABLE IF NOT EXISTS data_explorer_summoner_processing_state
(
    puuid varchar(255) not null,
    last_processed_at datetime(6) not null,
    next_eligible_at datetime(6) not null,
    updated_at datetime(6) not null default current_timestamp(6) on update current_timestamp(6),
    primary key (puuid),
    key data_explorer_summoner_state_eligible_index (next_eligible_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS data_explorer_match_processing_state
(
    match_id varchar(255) not null,
    last_processed_at datetime(6) not null,
    next_eligible_at datetime(6) not null,
    updated_at datetime(6) not null default current_timestamp(6) on update current_timestamp(6),
    primary key (match_id),
    key data_explorer_match_state_eligible_index (next_eligible_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS data_explorer_source_cleanup_state
(
    state_key varchar(64) not null,
    cursor_match_id varchar(255) not null default '',
    cursor_puuid varchar(255) not null default '',
    updated_at datetime(6) not null default current_timestamp(6) on update current_timestamp(6),
    primary key (state_key)
) ENGINE=InnoDB;

INSERT IGNORE INTO data_explorer_state
    (state_key, cursor_match_id, cursor_participant_id, completed)
VALUES
    ('summoner_processing_state_backfill', '', 0, 0),
    ('match_processing_state_backfill', '', 0, 0);

INSERT IGNORE INTO data_explorer_source_cleanup_state
    (state_key, cursor_match_id, cursor_puuid)
VALUES ('completed_match_sources', '', '');
