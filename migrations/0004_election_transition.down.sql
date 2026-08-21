DROP INDEX IF EXISTS elections_transition_ends_at_idx;

ALTER TABLE elections
    DROP CONSTRAINT IF EXISTS elections_transition_consistency,
    DROP CONSTRAINT IF EXISTS elections_status_check,
    ADD CONSTRAINT elections_status_check CHECK (status IN ('nomination', 'voting', 'closed')),
    DROP COLUMN IF EXISTS transition_ends_at,
    DROP COLUMN IF EXISTS transition_days;
