CREATE TABLE elections (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL CHECK (status IN ('nomination', 'voting', 'closed')),
    created_by  BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at   TIMESTAMPTZ,
    winner_id   BIGINT      REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX elections_status_idx ON elections (status);

CREATE TABLE candidates (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    election_id BIGINT      NOT NULL REFERENCES elections (id) ON DELETE CASCADE,
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    statement   TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One candidacy per user per election.
    UNIQUE (election_id, user_id),
    -- Composite target for votes (election_id, candidate_id) integrity.
    UNIQUE (election_id, id)
);

CREATE INDEX candidates_user_id_idx ON candidates (user_id);

CREATE TABLE votes (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    election_id  BIGINT      NOT NULL,
    voter_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    candidate_id BIGINT      NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One vote per voter per election.
    UNIQUE (election_id, voter_id),
    -- A vote's candidate must belong to the same election as the vote.
    FOREIGN KEY (election_id, candidate_id) REFERENCES candidates (election_id, id) ON DELETE CASCADE
);
