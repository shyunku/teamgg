-- The match ingestion flow stores participants before every participant's
-- summoner profile has necessarily been discovered. Reserve stable numeric
-- identities directly from the Riot identifiers so this additive foundation
-- never blocks the existing write order.
DROP TRIGGER IF EXISTS match_participants_numeric_key_bi;

CREATE TRIGGER match_participants_numeric_key_bi BEFORE INSERT ON match_participants
FOR EACH ROW
BEGIN
    INSERT INTO match_numeric_keys (riot_match_id) VALUES (NEW.match_id)
    ON DUPLICATE KEY UPDATE match_id = LAST_INSERT_ID(match_id);
    SET NEW.match_fk = LAST_INSERT_ID();

    INSERT INTO summoner_numeric_keys (puuid) VALUES (NEW.puuid)
    ON DUPLICATE KEY UPDATE summoner_id = LAST_INSERT_ID(summoner_id);
    SET NEW.summoner_fk = LAST_INSERT_ID();

    INSERT INTO match_participant_numeric_keys
        (legacy_match_participant_id, match_id, summoner_id, participant_id)
    VALUES
        (NEW.match_participant_id, NEW.match_fk, NEW.summoner_fk, NEW.participant_id)
    ON DUPLICATE KEY UPDATE
        match_participant_id = LAST_INSERT_ID(match_participant_id),
        match_id = VALUES(match_id),
        summoner_id = VALUES(summoner_id),
        participant_id = VALUES(participant_id);
    SET NEW.match_participant_pk = LAST_INSERT_ID();
END;
