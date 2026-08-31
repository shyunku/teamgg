-- Task #64 phase-1 rollback.
-- Run only before any child table or read path depends on numeric keys.
-- This script intentionally removes only the additive numeric-key foundation.

DROP TRIGGER IF EXISTS match_participants_numeric_key_bi;
DROP TRIGGER IF EXISTS matches_numeric_key_bi;
DROP TRIGGER IF EXISTS summoners_numeric_key_bu;
DROP TRIGGER IF EXISTS summoners_numeric_key_bi;

ALTER TABLE match_participants
    DROP COLUMN summoner_fk,
    DROP COLUMN match_fk,
    DROP COLUMN match_participant_pk,
    ALGORITHM=INSTANT;

ALTER TABLE matches DROP COLUMN match_pk, ALGORITHM=INSTANT;
ALTER TABLE summoners DROP COLUMN summoner_pk, ALGORITHM=INSTANT;

DROP TABLE IF EXISTS numeric_key_backfill_progress;
DROP TABLE IF EXISTS match_participant_numeric_keys;
DROP TABLE IF EXISTS match_numeric_keys;
DROP TABLE IF EXISTS summoner_numeric_keys;
