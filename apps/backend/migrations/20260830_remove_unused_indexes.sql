-- Applied by applyUnusedIndexCleanup so missing indexes remain idempotent and
-- same-named indexes with unexpected columns are never removed.
ALTER TABLE match_participant_perk_styles
    DROP INDEX match_participant_perk_styles_description_index,
    ALGORITHM=INPLACE,
    LOCK=NONE;

ALTER TABLE masteries
    DROP INDEX masteries_champion_id_champion_points_index,
    ALGORITHM=INPLACE,
    LOCK=NONE;

ALTER TABLE match_participants
    DROP INDEX match_participants_participant_id_index,
    DROP INDEX match_participants_team_position_index,
    ALGORITHM=INPLACE,
    LOCK=NONE;
