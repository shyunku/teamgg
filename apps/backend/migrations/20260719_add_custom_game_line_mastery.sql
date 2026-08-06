ALTER TABLE custom_game_configurations
    ADD COLUMN mastery_influence_weight DOUBLE NOT NULL DEFAULT 0.5 AFTER line_satisfaction_weight;

ALTER TABLE custom_game_candidates
    ADD COLUMN mastery_top INT NOT NULL DEFAULT 0 AFTER flavor_support,
    ADD COLUMN mastery_jungle INT NOT NULL DEFAULT 0 AFTER mastery_top,
    ADD COLUMN mastery_mid INT NOT NULL DEFAULT 0 AFTER mastery_jungle,
    ADD COLUMN mastery_adc INT NOT NULL DEFAULT 0 AFTER mastery_mid,
    ADD COLUMN mastery_support INT NOT NULL DEFAULT 0 AFTER mastery_adc;

ALTER TABLE riot_custom_game_preferences
    ADD COLUMN mastery_top INT NOT NULL DEFAULT 0 AFTER flavor_support,
    ADD COLUMN mastery_jungle INT NOT NULL DEFAULT 0 AFTER mastery_top,
    ADD COLUMN mastery_mid INT NOT NULL DEFAULT 0 AFTER mastery_jungle,
    ADD COLUMN mastery_adc INT NOT NULL DEFAULT 0 AFTER mastery_mid,
    ADD COLUMN mastery_support INT NOT NULL DEFAULT 0 AFTER mastery_adc;
