CREATE TABLE IF NOT EXISTS user_identities
(
    provider         VARCHAR(32)  NOT NULL,
    provider_subject VARCHAR(255) NOT NULL,
    uid              VARCHAR(255) NOT NULL,
    display_name     VARCHAR(255) NOT NULL,
    created_at       DATETIME     NOT NULL,
    updated_at       DATETIME     NOT NULL,
    PRIMARY KEY (provider, provider_subject),
    CONSTRAINT user_identities_provider_uid_uindex
        UNIQUE (provider, uid),
    CONSTRAINT user_identities_users_uid_fk
        FOREIGN KEY (uid) REFERENCES users (uid)
            ON UPDATE CASCADE ON DELETE CASCADE
);
