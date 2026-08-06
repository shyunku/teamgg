CREATE TABLE IF NOT EXISTS statistics_snapshots
(
    snapshot_key varchar(64) not null,
    payload longblob not null,
    updated_at datetime(6) not null default current_timestamp(6),
    primary key (snapshot_key),
    key statistics_snapshots_updated_at_index (updated_at)
) ENGINE=InnoDB;
