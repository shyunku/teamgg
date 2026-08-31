-- Task #64 phase 1: additive numeric-key foundation.
-- Applied by migrations/numeric_keys.go so partially initialized databases can resume safely.
-- This phase keeps all legacy keys and does not replace primary keys.

CREATE TABLE IF NOT EXISTS summoner_numeric_keys (
    summoner_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    puuid VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (summoner_id),
    UNIQUE KEY summoner_numeric_keys_puuid_uindex (puuid)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS match_numeric_keys (
    match_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    riot_match_id VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (match_id),
    UNIQUE KEY match_numeric_keys_riot_match_id_uindex (riot_match_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS match_participant_numeric_keys (
    match_participant_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    legacy_match_participant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    match_id BIGINT UNSIGNED NULL,
    summoner_id BIGINT UNSIGNED NULL,
    participant_id TINYINT UNSIGNED NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (match_participant_id),
    UNIQUE KEY match_participant_numeric_keys_legacy_uindex (legacy_match_participant_id),
    UNIQUE KEY match_participant_numeric_keys_match_slot_uindex (match_id, participant_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS numeric_key_backfill_progress (
    entity_name VARCHAR(32) NOT NULL,
    cursor_text VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    cursor_number INT NOT NULL DEFAULT 0,
    processed_rows BIGINT UNSIGNED NOT NULL DEFAULT 0,
    completed TINYINT(1) NOT NULL DEFAULT 0,
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (entity_name)
) ENGINE=InnoDB;

-- The Go migration adds nullable summoner_pk, match_pk, match_participant_pk,
-- match_fk and summoner_fk columns with ALGORITHM=INSTANT, then installs the
-- new-write synchronization triggers.
