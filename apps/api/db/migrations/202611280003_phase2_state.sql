-- +goose Up
CREATE TABLE phase2_state (
    student_id UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    module     TEXT NOT NULL,
    payload    JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (student_id, module)
);

-- +goose Down
DROP TABLE phase2_state;
