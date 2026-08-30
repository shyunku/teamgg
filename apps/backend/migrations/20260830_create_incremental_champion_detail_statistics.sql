CREATE TABLE IF NOT EXISTS champion_detail_statistics_participants (
    source_id BIGINT NOT NULL AUTO_INCREMENT,
    game_version VARCHAR(64) NOT NULL,
    match_id VARCHAR(255) NOT NULL,
    match_participant_id VARCHAR(255) NOT NULL,
    game_duration BIGINT NOT NULL,
    champion_id INT NOT NULL,
    champion_name VARCHAR(255) NOT NULL,
    team_position VARCHAR(32) NOT NULL,
    kills INT NOT NULL,
    deaths INT NOT NULL,
    assists INT NOT NULL,
    team_id INT NOT NULL,
    summoner1_id INT NOT NULL,
    summoner2_id INT NOT NULL,
    win TINYINT(1) NOT NULL,
    total_minions_killed INT NOT NULL,
    gold_earned INT NOT NULL,
    total_damage_dealt_to_champions INT NOT NULL,
    total_damage_taken INT NOT NULL,
    total_heal INT NOT NULL,
    vision_score INT NOT NULL,
    total_time_cc_dealt INT NOT NULL,
    damage_self_mitigated INT NOT NULL,
    damage_dealt_to_buildings INT NOT NULL,
    damage_dealt_to_objectives INT NOT NULL,
    damage_dealt_to_turrets INT NOT NULL,
    enemy_champion_id INT NULL,
    enemy_champion_name VARCHAR(255) NULL,
    enemy_kills INT NULL,
    enemy_deaths INT NULL,
    enemy_assists INT NULL,
    enemy_win TINYINT(1) NULL,
    build_valid TINYINT(1) NOT NULL DEFAULT 0,
    primary_style INT NULL,
    primary_perk0 INT NULL,
    primary_perk1 INT NULL,
    primary_perk2 INT NULL,
    primary_perk3 INT NULL,
    sub_style INT NULL,
    sub_perk0 INT NULL,
    sub_perk1 INT NULL,
    stat_perk_defense INT NULL,
    stat_perk_flex INT NULL,
    stat_perk_offense INT NULL,
    item0_id INT NULL,
    item0_name VARCHAR(255) NULL,
    item1_id INT NULL,
    item1_name VARCHAR(255) NULL,
    item2_id INT NULL,
    item2_name VARCHAR(255) NULL,
    item3_id INT NULL,
    item3_name VARCHAR(255) NULL,
    item4_id INT NULL,
    item4_name VARCHAR(255) NULL,
    item5_id INT NULL,
    item5_name VARCHAR(255) NULL,
    PRIMARY KEY (source_id),
    UNIQUE KEY champion_detail_participant_uindex (match_participant_id),
    KEY champion_detail_participant_version_match_index (game_version, match_id),
    KEY champion_detail_participant_match_index (match_id),
    KEY champion_detail_participant_meta_index
        (build_valid, champion_id, team_position, primary_style, sub_style),
    KEY champion_detail_participant_counter_index
        (build_valid, champion_id, enemy_champion_id, team_position)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS champion_detail_statistics_bans (
    game_version VARCHAR(64) NOT NULL,
    match_id VARCHAR(255) NOT NULL,
    team_id INT NOT NULL,
    champion_id INT NOT NULL,
    pick_turn INT NOT NULL,
    PRIMARY KEY (match_id, team_id, pick_turn),
    KEY champion_detail_bans_version_champion_index (game_version, champion_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS champion_detail_statistics_processed_matches (
    game_version VARCHAR(64) NOT NULL,
    match_id VARCHAR(255) NOT NULL,
    processed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (game_version, match_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS champion_detail_statistics_progress (
    game_version VARCHAR(64) NOT NULL,
    last_match_id VARCHAR(255) NOT NULL DEFAULT '',
    processed_matches BIGINT NOT NULL DEFAULT 0,
    completed TINYINT(1) NOT NULL DEFAULT 0,
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (game_version)
) ENGINE=InnoDB;

CREATE OR REPLACE VIEW champion_detail_statistics_valid_builds AS
SELECT *
FROM champion_detail_statistics_participants
WHERE build_valid = 1;
