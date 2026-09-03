-- Manual rollback for migration 20260830_004.
-- Run only after confirming that each index is currently absent.
ALTER TABLE match_participant_perk_styles
    ADD INDEX match_participant_perk_styles_description_index (description),
    ALGORITHM=INPLACE,
    LOCK=NONE;

-- The legacy masteries table was retired by 20260903_002. Its removed index
-- has no rollback action; numeric mastery storage has its own covering index.

ALTER TABLE match_participants
    ADD INDEX match_participants_participant_id_index (participant_id),
    ADD INDEX match_participants_team_position_index (team_position),
    ALGORITHM=INPLACE,
    LOCK=NONE;
