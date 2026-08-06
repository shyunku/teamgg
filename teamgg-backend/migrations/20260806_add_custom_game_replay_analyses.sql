CREATE TABLE IF NOT EXISTS custom_game_replay_analyses (
    id VARCHAR(36) NOT NULL,
    custom_game_config_id VARCHAR(255) NOT NULL,
    creator_uid VARCHAR(255) NOT NULL,
    request_id VARCHAR(64) NULL,
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    status VARCHAR(24) NOT NULL,
    stage VARCHAR(255) NOT NULL,
    progress INT NOT NULL DEFAULT 0,
    analysis LONGTEXT NULL,
    model VARCHAR(100) NULL,
    error_message TEXT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    completed_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    KEY custom_game_replay_analyses_config_created_index (custom_game_config_id, created_at),
    KEY custom_game_replay_analyses_status_index (status),
    CONSTRAINT custom_game_replay_analyses_config_fk
        FOREIGN KEY (custom_game_config_id) REFERENCES custom_game_configurations (id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT custom_game_replay_analyses_creator_fk
        FOREIGN KEY (creator_uid) REFERENCES users (uid)
        ON UPDATE CASCADE ON DELETE CASCADE
) ENGINE=InnoDB;
