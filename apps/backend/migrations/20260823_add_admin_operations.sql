CREATE TABLE IF NOT EXISTS user_roles
(
    uid varchar(255) not null,
    role varchar(24) not null,
    created_at datetime(6) not null default current_timestamp(6),
    updated_at datetime(6) not null default current_timestamp(6) on update current_timestamp(6),
    primary key (uid),
    key user_roles_role_index (role),
    constraint user_roles_user_fk foreign key (uid) references users (uid)
        on update cascade on delete cascade
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS admin_audit_logs
(
    id bigint unsigned not null auto_increment,
    actor_uid varchar(255) not null,
    action varchar(64) not null,
    resource varchar(128) not null,
    result varchar(24) not null,
    client_ip varchar(64) not null default '',
    metadata_json text null,
    created_at datetime(6) not null default current_timestamp(6),
    primary key (id),
    key admin_audit_logs_actor_created_index (actor_uid, created_at),
    key admin_audit_logs_created_index (created_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS admin_operational_events
(
    id bigint unsigned not null auto_increment,
    source varchar(48) not null,
    level varchar(16) not null,
    event_type varchar(64) not null,
    message varchar(500) not null,
    details_json text null,
    created_at datetime(6) not null default current_timestamp(6),
    primary key (id),
    key admin_operational_events_created_index (created_at),
    key admin_operational_events_type_created_index (event_type, created_at)
) ENGINE=InnoDB;
