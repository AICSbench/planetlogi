-- +goose Up
-- Shared (non-student-scoped) durable state for Phase 3 social features:
-- Star Crew rosters / membership and Galaxy Showcase entries. Unlike
-- phase2_state, rows here are keyed by an arbitrary text key so records that
-- span several students (a crew with multiple members) can be persisted.
CREATE TABLE shared_state (
    scope      TEXT NOT NULL,
    key        TEXT NOT NULL,
    payload    JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scope, key)
);

-- +goose Down
DROP TABLE shared_state;
