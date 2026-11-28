-- +goose Up
CREATE TABLE training_attempt_receipts (
    student_id UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    attempt_id TEXT NOT NULL,
    PRIMARY KEY (student_id, attempt_id)
);

-- +goose Down
DROP TABLE training_attempt_receipts;
