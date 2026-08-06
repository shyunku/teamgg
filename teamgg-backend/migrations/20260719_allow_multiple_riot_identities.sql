ALTER TABLE user_identities
    ADD COLUMN is_primary TINYINT(1) NOT NULL DEFAULT 0 AFTER display_name;

UPDATE user_identities
SET is_primary = 1;

ALTER TABLE user_identities
    DROP INDEX user_identities_provider_uid_uindex,
    ADD INDEX user_identities_provider_uid_index (provider, uid);
