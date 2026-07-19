CREATE TABLE IF NOT EXISTS riot_custom_game_preferences
(
    puuid          VARCHAR(255) NOT NULL PRIMARY KEY,
    flavor_top     INT          NOT NULL DEFAULT 0,
    flavor_jungle  INT          NOT NULL DEFAULT 0,
    flavor_mid     INT          NOT NULL DEFAULT 0,
    flavor_adc     INT          NOT NULL DEFAULT 0,
    flavor_support INT          NOT NULL DEFAULT 0,
    mastery_top    INT          NOT NULL DEFAULT 0,
    mastery_jungle INT          NOT NULL DEFAULT 0,
    mastery_mid    INT          NOT NULL DEFAULT 0,
    mastery_adc    INT          NOT NULL DEFAULT 0,
    mastery_support INT         NOT NULL DEFAULT 0,
    updated_at     DATETIME     NOT NULL,
    CONSTRAINT riot_custom_game_preferences_summoners_puuid_fk
        FOREIGN KEY (puuid) REFERENCES summoners (puuid)
            ON UPDATE CASCADE ON DELETE CASCADE
);
