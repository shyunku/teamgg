CREATE TABLE IF NOT EXISTS data_explorer_metrics_daily
(
    metric_date date not null,
    baseline_summoner_rows bigint not null default 0,
    current_summoner_rows bigint not null default 0,
    baseline_match_rows bigint not null default 0,
    current_match_rows bigint not null default 0,
    baseline_mastery_rows bigint not null default 0,
    current_mastery_rows bigint not null default 0,
    baseline_queue_rows bigint not null default 0,
    current_queue_rows bigint not null default 0,
    created_at datetime(6) not null default current_timestamp(6),
    updated_at datetime(6) not null default current_timestamp(6) on update current_timestamp(6),
    primary key (metric_date)
) ENGINE=InnoDB;
