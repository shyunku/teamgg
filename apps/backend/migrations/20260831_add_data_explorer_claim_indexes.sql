-- Prevent FOR UPDATE SKIP LOCKED job claims from filesorting and locking a
-- large pending range. Applied by migrations/data_explorer_claim_indexes.go.

ALTER TABLE data_explorer_summoner_jobs
    ADD INDEX data_explorer_summoner_jobs_claim_v2_index
        (status, next_attempt_at, priority DESC, created_at, puuid),
    ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE data_explorer_match_jobs
    ADD INDEX data_explorer_match_jobs_claim_v2_index
        (status, next_attempt_at, priority DESC, created_at, match_id),
    ALGORITHM=INPLACE, LOCK=NONE;

-- Existing claim indexes remain available for rollback until the new plans are
-- verified in production.
