create table teamgg.matches
(
    data_version         varchar(255) not null,
    match_id             varchar(255) not null
        primary key,
    game_creation        bigint       not null,
    game_duration        bigint       not null,
    game_end_timestamp   bigint       not null,
    game_id              bigint       not null,
    game_mode            varchar(255) not null,
    game_name            varchar(255) not null,
    game_start_timestamp bigint       not null,
    game_type            varchar(255) not null,
    game_version         varchar(255) not null,
    map_id               int          not null,
    platform_id          varchar(255) not null,
    queue_id             int          not null,
    tournament_code      varchar(255) not null
);

create table teamgg.match_participants
(
    match_id                           varchar(255) not null,
    participant_id                     int          not null,
    match_participant_id               varchar(255) not null,
    puuid                              varchar(255) not null,
    kills                              int          not null,
    deaths                             int          not null,
    assists                            int          not null,
    champion_id                        int          not null,
    champion_level                     int          not null,
    champion_name                      varchar(255) not null,
    champ_experience                   int          not null,
    summoner_level                     int          not null,
    summoner_name                      varchar(255) not null,
    riot_id_name                       varchar(255) not null,
    riot_id_tag_line                   varchar(255) not null,
    profile_icon                       int          not null,
    magic_damage_dealt_to_champions    int          not null,
    physical_damage_dealt_to_champions int          not null,
    true_damage_dealt_to_champions     int          not null,
    total_damage_dealt_to_champions    int          not null,
    magic_damage_taken                 int          not null,
    physical_damage_taken              int          not null,
    true_damage_taken                  int          not null,
    total_damage_taken                 int          not null,
    total_heal                         int          not null,
    total_heals_on_teammates           int          not null,
    item0                              int          not null,
    item1                              int          not null,
    item2                              int          not null,
    item3                              int          not null,
    item4                              int          not null,
    item5                              int          not null,
    item6                              int          not null,
    spell1_casts                       int          not null,
    spell2_casts                       int          not null,
    spell3_casts                       int          not null,
    spell4_casts                       int          not null,
    summoner1_casts                    int          not null,
    summoner1_id                       int          not null,
    summoner2_casts                    int          not null,
    summoner2_id                       int          not null,
    first_blood_assist                 tinyint(1)   not null,
    first_blood_kill                   tinyint(1)   not null,
    double_kills                       int          not null,
    triple_kills                       int          not null,
    quadra_kills                       int          not null,
    penta_kills                        int          not null,
    total_minions_killed               int          not null,
    total_time_cc_dealt                int          not null,
    neutral_minions_killed             int          not null,
    gold_spent                         int          not null,
    gold_earned                        int          not null,
    individual_position                varchar(255) not null,
    team_position                      varchar(255) not null,
    lane                               varchar(255) not null,
    role                               varchar(255) not null,
    team_id                            int          not null,
    vision_score                       int          not null,
    win                                tinyint(1)   not null,
    game_ended_in_early_surrender      tinyint(1)   not null,
    game_ended_in_surrender            tinyint(1)   not null,
    team_early_surrendered             tinyint(1)   not null,
    primary key (match_id, participant_id),
    constraint match_participants_matches_match_id_fk
        foreign key (match_id) references teamgg.matches (match_id)
            on update cascade on delete cascade
);

create table teamgg.match_participant_details
(
    match_participant_id               varchar(255) not null
        primary key,
    match_id                           varchar(255) not null,
    baron_kills                        int          not null,
    bounty_level                       int          not null,
    champion_transform                 int          not null,
    consumables_purchased              int          not null,
    damage_dealt_to_buildings          int          not null,
    damage_dealt_to_objectives         int          not null,
    damage_dealt_to_turrets            int          not null,
    damage_self_mitigated              int          not null,
    detector_wards_placed              int          not null,
    dragon_kills                       int          not null,
    physical_damage_dealt              int          not null,
    magic_damage_dealt                 int          not null,
    total_damage_dealt                 int          not null,
    largest_critical_strike            int          not null,
    largest_killing_spree              int          not null,
    largest_multi_kill                 int          not null,
    first_tower_assist                 tinyint(1)   not null,
    first_tower_kill                   tinyint(1)   not null,
    inhibitor_kills                    int          not null,
    inhibitor_takedowns                int          not null,
    inhibitors_lost                    int          not null,
    items_purchased                    int          not null,
    killing_sprees                     int          not null,
    nexus_kills                        int          not null,
    nexus_takedowns                    int          not null,
    nexus_lost                         int          not null,
    longest_time_spent_living          int          not null,
    objective_stolen                   int          not null,
    objective_stolen_assists           int          not null,
    sight_wards_bought_in_game         int          not null,
    vision_wards_bought_in_game        int          not null,
    summoner_id                        varchar(255) not null,
    time_ccing_others                  int          not null,
    time_played                        int          not null,
    total_damage_shielded_on_teammates int          not null,
    total_time_spent_dead              int          not null,
    total_units_healed                 int          not null,
    true_damage_dealt                  int          not null,
    turret_kills                       int          not null,
    turret_takedowns                   int          not null,
    turrets_lost                       int          not null,
    unreal_kills                       int          not null,
    wards_killed                       int          not null,
    wards_placed                       int          not null,
    constraint match_participant_details_id_fk
        foreign key (match_participant_id) references teamgg.match_participants (match_participant_id)
            on update cascade on delete cascade,
    constraint match_participant_details_matches_match_id_fk
        foreign key (match_id) references teamgg.matches (match_id)
            on update cascade on delete cascade
);

create table teamgg.match_participant_perk_styles
(
    match_participant_id varchar(255) not null,
    style_id             varchar(255) not null,
    description          varchar(255) not null,
    style                int          not null,
    constraint match_participant_perk_styles_pk
        unique (style_id),
    constraint match_participant_perk_styles_id_fk
        foreign key (match_participant_id) references teamgg.match_participants (match_participant_id)
            on update cascade on delete cascade
);

create table teamgg.match_participant_perk_style_selections
(
    style_id varchar(255) not null,
    perk     int          not null,
    var1     int          not null,
    var2     int          not null,
    var3     int          not null,
    constraint match_participant_perk_style_selection_id_fk
        foreign key (style_id) references teamgg.match_participant_perk_styles (style_id)
            on update cascade on delete cascade
);

create index match_participant_perk_styles_description_index
    on teamgg.match_participant_perk_styles (description);

create table teamgg.match_participant_perks
(
    match_participant_id varchar(255) not null
        primary key,
    stat_perk_defense    int          not null,
    stat_perk_flex       int          not null,
    stat_perk_offense    int          not null,
    constraint match_participant_perks_id_fk
        foreign key (match_participant_id) references teamgg.match_participants (match_participant_id)
            on update cascade on delete cascade
);

create index match_participants_champion_id_index
    on teamgg.match_participants (champion_id);

create index match_participants_match_participant_id_index
    on teamgg.match_participants (match_participant_id);

create index match_participants_participant_id_index
    on teamgg.match_participants (participant_id);

create index match_participants_summoner_puuid_fk
    on teamgg.match_participants (puuid);

create index match_participants_team_position_index
    on teamgg.match_participants (team_position);

create table teamgg.match_teams
(
    match_id          varchar(255) not null,
    team_id           int          not null,
    win               tinyint(1)   not null,
    baron_first       tinyint(1)   not null,
    baron_kills       int          not null,
    champion_first    tinyint(1)   not null,
    champion_kills    int          not null,
    dragon_first      tinyint(1)   not null,
    dragon_kills      int          not null,
    inhibitor_first   tinyint(1)   not null,
    inhibitor_kills   int          not null,
    rift_herald_first tinyint(1)   not null,
    rift_herald_kills int          not null,
    tower_first       tinyint(1)   not null,
    tower_kills       int          not null,
    constraint match_teams_matches_match_id_fk
        foreign key (match_id) references teamgg.matches (match_id)
            on update cascade on delete cascade
);

create table teamgg.match_team_bans
(
    match_id    varchar(255) not null,
    team_id     int          not null,
    champion_id int          not null,
    pick_turn   int          not null,
    constraint match_team_bans_match_teams_team_id_fk
        foreign key (team_id) references teamgg.match_teams (team_id)
            on update cascade on delete cascade,
    constraint match_team_bans_matches_match_id_fk
        foreign key (match_id) references teamgg.matches (match_id)
            on update cascade on delete cascade
);

create index match_teams_team_id_index
    on teamgg.match_teams (team_id);

create index matches_game_end_timestamp_index
    on teamgg.matches (game_end_timestamp);

create index matches_game_start_timestamp_index
    on teamgg.matches (game_start_timestamp);

create table teamgg.static_items
(
    id               int          not null
        primary key,
    name             varchar(255) not null,
    description      text         not null,
    plaintext        text         not null,
    required_ally    varchar(255) null,
    depth            int          null,
    gold_base        int          not null,
    gold_purchasable tinyint      not null,
    gold_total       int          not null,
    gold_sell        int          not null
);

create table teamgg.static_item_tags
(
    item_id int          not null,
    tag     varchar(100) not null,
    primary key (item_id, tag),
    constraint static_item_tags_ibfk_1
        foreign key (item_id) references teamgg.static_items (id)
);

create index tag
    on teamgg.static_item_tags (tag);

create index depth
    on teamgg.static_items (depth desc);

create index gold_total
    on teamgg.static_items (gold_total desc);

create index name
    on teamgg.static_items (name);

create table teamgg.static_tier_ranks
(
    id         int auto_increment
        primary key,
    tier_label varchar(20) not null,
    rank_label varchar(20) not null,
    score      int         not null,
    constraint score
        unique (score),
    constraint tier_label
        unique (tier_label, rank_label)
);

create index rank_label
    on teamgg.static_tier_ranks (rank_label);

create index tier_label_2
    on teamgg.static_tier_ranks (tier_label);

create table teamgg.summoners
(
    account_id        varchar(255) not null,
    profile_icon_id   int          not null,
    revision_date     mediumtext   not null,
    game_name         varchar(255) not null,
    tag_line          varchar(255) not null,
    name              varchar(255) not null,
    id                varchar(255) not null,
    puuid             varchar(255) not null
        primary key,
    summoner_level    bigint       not null,
    shorten_game_name varchar(255) not null,
    shorten_name      varchar(255) not null,
    last_updated_at   datetime     not null
);

create table teamgg.leagues
(
    puuid         varchar(255) not null,
    league_id     varchar(255) not null,
    queue_type    varchar(255) not null,
    tier          varchar(255) not null,
    league_rank   varchar(255) not null,
    league_points int          not null,
    wins          int          not null,
    losses        int          not null,
    hot_streak    tinyint(1)   not null,
    veteran       tinyint(1)   not null,
    fresh_blood   tinyint(1)   not null,
    inactive      tinyint(1)   not null,
    ms_target     int          null,
    ms_wins       int          null,
    ms_losses     int          null,
    ms_progress   varchar(255) null,
    primary key (puuid, league_id, queue_type),
    constraint ranks_summoner_puuid_fk
        foreign key (puuid) references teamgg.summoners (puuid)
            on update cascade on delete cascade
);

create index leagues_queue_type_index
    on teamgg.leagues (queue_type);

create index leagues_queue_type_tier_league_rank_league_points_wins_index
    on teamgg.leagues (queue_type asc, tier asc, league_rank asc, league_points desc, wins desc);

create table teamgg.masteries
(
    puuid                            varchar(255) not null,
    champion_points_until_next_level bigint       not null,
    chest_granted                    tinyint(1)   not null,
    champion_id                      bigint       not null,
    last_play_time                   datetime     not null,
    champion_level                   int          not null,
    champion_points                  int          not null,
    champion_points_since_last_level bigint       not null,
    tokens_earned                    int          not null,
    primary key (puuid, champion_id),
    constraint masteries_summoner_puuid_fk
        foreign key (puuid) references teamgg.summoners (puuid)
            on update cascade on delete cascade
);

create index masteries_champion_id_champion_points_index
    on teamgg.masteries (champion_id asc, champion_points desc);

create table teamgg.summoner_matches
(
    puuid    varchar(255) not null,
    match_id varchar(255) not null,
    constraint summoner_matches_matches_match_id_fk
        foreign key (match_id) references teamgg.matches (match_id)
            on update cascade on delete cascade,
    constraint summoner_matches_summoners_puuid_fk
        foreign key (puuid) references teamgg.summoners (puuid)
            on update cascade on delete cascade
);

create table teamgg.summoner_rankings
(
    puuid        varchar(255) not null
        primary key,
    ranking      int          not null,
    rating_point int          not null,
    total        int          not null,
    updated_at   datetime     not null,
    constraint summoner_rankings_summoners_puuid_fk
        foreign key (puuid) references teamgg.summoners (puuid)
            on update cascade on delete cascade
);

create index summoners_game_name_index
    on teamgg.summoners (game_name desc);

create index summoners_name_index
    on teamgg.summoners (name desc);

create index summoners_shorten_game_name_index
    on teamgg.summoners (shorten_game_name desc);

create index summoners_shorten_name_index
    on teamgg.summoners (shorten_name desc);

create index summoners_tag_line_index
    on teamgg.summoners (tag_line desc);

create table teamgg.users
(
    uid          varchar(255) not null
        primary key,
    user_id      varchar(255) not null,
    encrypted_pw varchar(255) not null,
    constraint user_id
        unique (user_id)
);

create table teamgg.user_identities
(
    provider         varchar(32)  not null,
    provider_subject varchar(255) not null,
    uid              varchar(255) not null,
    display_name     varchar(255) not null,
    created_at       datetime     not null,
    updated_at       datetime     not null,
    primary key (provider, provider_subject),
    constraint user_identities_provider_uid_uindex
        unique (provider, uid),
    constraint user_identities_users_uid_fk
        foreign key (uid) references teamgg.users (uid)
            on update cascade on delete cascade
);

create table teamgg.custom_game_configurations
(
    id                       varchar(255)            not null
        primary key,
    name                     varchar(255)            not null,
    creator_uid              varchar(255)            not null,
    created_at               datetime                not null,
    last_updated_at          datetime                not null,
    is_public                tinyint(1) default 0    not null,
    fairness                 double                  not null,
    line_fairness            double                  not null,
    tier_fairness            double                  not null,
    line_satisfaction        double                  not null,
    line_fairness_weight     double     default 0.36 not null,
    tier_fairness_weight     double     default 0.24 not null,
    line_satisfaction_weight double     default 0.4  not null,
    top_influence_weight     double     default 0.14 not null,
    jungle_influence_weight  double     default 0.23 not null,
    mid_influence_weight     double     default 0.25 not null,
    adc_influence_weight     double     default 0.21 not null,
    support_influence_weight double     default 0.17 not null,
    constraint custom_game_configurations_users_uid_fk
        foreign key (creator_uid) references teamgg.users (uid)
            on update cascade on delete cascade
);

create table teamgg.custom_game_candidates
(
    custom_game_config_id varchar(255)  not null,
    puuid                 varchar(255)  not null,
    custom_tier           varchar(255)  null,
    custom_rank           varchar(255)  null,
    flavor_top            int default 0 not null,
    flavor_jungle         int default 0 not null,
    flavor_mid            int default 0 not null,
    flavor_adc            int default 0 not null,
    flavor_support        int default 0 not null,
    primary key (custom_game_config_id, puuid),
    constraint custom_game_candidates_custom_game_configurations_id_fk
        foreign key (custom_game_config_id) references teamgg.custom_game_configurations (id)
            on update cascade on delete cascade,
    constraint custom_game_candidates_summoners_puuid_fk
        foreign key (puuid) references teamgg.summoners (puuid)
            on update cascade on delete cascade
);

create table teamgg.custom_game_participant_color_labels
(
    custom_game_config_id varchar(255) not null,
    puuid                 varchar(255) not null,
    color_code            int          not null,
    primary key (custom_game_config_id, puuid),
    constraint custom_game_color_labels_configurations_id_fk
        foreign key (custom_game_config_id) references teamgg.custom_game_configurations (id)
);

create table teamgg.custom_game_participants
(
    custom_game_config_id varchar(255)  not null,
    puuid                 varchar(255)  not null,
    team                  int default 0 not null,
    position              varchar(255)  not null,
    primary key (custom_game_config_id, puuid),
    constraint custom_game_participants_pk
        unique (custom_game_config_id, team, position),
    constraint custom_game_participants_custom_game_configurations_id_fk
        foreign key (custom_game_config_id) references teamgg.custom_game_configurations (id)
            on update cascade on delete cascade,
    constraint custom_game_participants_summoners_puuid_fk
        foreign key (puuid) references teamgg.summoners (puuid)
            on update cascade on delete cascade
);

