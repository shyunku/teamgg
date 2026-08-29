CREATE TABLE IF NOT EXISTS mastery_statistics_aggregates (
    champion_id BIGINT NOT NULL,
    max_mastery BIGINT NOT NULL,
    total_mastery BIGINT NOT NULL,
    mastered_count BIGINT NOT NULL,
    summoner_count BIGINT NOT NULL,
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (champion_id),
    KEY mastery_statistics_aggregates_updated_index (updated_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS mastery_statistics_dirty_champions (
    champion_id BIGINT NOT NULL,
    dirty_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (champion_id),
    KEY mastery_statistics_dirty_at_index (dirty_at, champion_id)
) ENGINE=InnoDB;

ALTER TABLE masteries
    ADD INDEX masteries_champion_points_level_covering_index
        (champion_id ASC, champion_points DESC, champion_level),
    ALGORITHM=INPLACE,
    LOCK=NONE;

-- The versioned Go migration runner installs AFTER INSERT, UPDATE, and DELETE
-- triggers that only enqueue changed champion IDs, then seeds the queue once
-- from the existing champion IDs. The collector refreshes each queued champion
-- independently and acknowledges only dirty_at values older than its DB cutoff.
