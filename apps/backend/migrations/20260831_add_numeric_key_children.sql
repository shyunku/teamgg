-- Additive child-table phase for the numeric-key schema v2 migration.
-- Applied by migrations/numeric_key_children.go so every ALTER and trigger
-- installation is resumable and validated independently.

CREATE TABLE IF NOT EXISTS numeric_key_child_backfill_progress (
    entity_name VARCHAR(64) NOT NULL,
    cursor_text VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    cursor_text_2 VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    processed_rows BIGINT UNSIGNED NOT NULL DEFAULT 0,
    completed TINYINT(1) NOT NULL DEFAULT 0,
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (entity_name)
) ENGINE=InnoDB;

-- The Go migration adds nullable numeric FK columns with ALGORITHM=INSTANT
-- and installs BEFORE INSERT/UPDATE dual-write triggers for the eight core
-- relationship tables. Index and constraint cutover is intentionally deferred
-- until the bounded backfill and equality checks are complete.
