ALTER TABLE elections
    ADD COLUMN transition_days    INT         NOT NULL DEFAULT 7,
    ADD COLUMN transition_ends_at TIMESTAMPTZ,
    DROP CONSTRAINT elections_status_check,
    ADD CONSTRAINT elections_status_check CHECK (status IN ('nomination', 'voting', 'transition', 'closed')),
    -- A transition only exists when there is a winner to hand over to and a deadline.
    ADD CONSTRAINT elections_transition_consistency CHECK (
        status <> 'transition' OR (winner_id IS NOT NULL AND transition_ends_at IS NOT NULL)
    );

CREATE INDEX elections_transition_ends_at_idx ON elections (transition_ends_at) WHERE status = 'transition';
