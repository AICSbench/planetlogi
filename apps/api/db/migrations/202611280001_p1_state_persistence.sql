-- +goose Up
ALTER TABLE student_streaks
    ADD COLUMN IF NOT EXISTS today_correct_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS today_local_date DATE,
    ADD COLUMN IF NOT EXISTS reminder_time_local TEXT NOT NULL DEFAULT '19:00';

CREATE TABLE IF NOT EXISTS daily_orbit_prefs (
    student_id       UUID PRIMARY KEY REFERENCES student_profiles(id) ON DELETE CASCADE,
    grade            SMALLINT NOT NULL DEFAULT 1 CHECK (grade BETWEEN 1 AND 6),
    tier             TEXT NOT NULL DEFAULT 'ELEMENTARY' CHECK (tier IN ('ELEMENTARY', 'INTERMEDIATE', 'ADVANCED', 'MASTER')),
    active_curricula JSONB NOT NULL DEFAULT '["SINGAPORE_MOE","HOTS_WHEEL"]'::jsonb,
    total_sp         BIGINT NOT NULL DEFAULT 12,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS training_skill_stats (
    student_id           UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    skill_code           TEXT NOT NULL,
    recent_results       JSONB NOT NULL DEFAULT '[]'::jsonb,
    recent_window_count  SMALLINT NOT NULL DEFAULT 0,
    recent_correct_count SMALLINT NOT NULL DEFAULT 0,
    proficiency_pct      SMALLINT NOT NULL DEFAULT 0 CHECK (proficiency_pct BETWEEN 0 AND 100),
    proficiency_tier     TEXT NOT NULL DEFAULT 'INCOMPLETE' CHECK (proficiency_tier IN ('INCOMPLETE', 'BEGINNING', 'COMPETENT', 'MASTERY')),
    best_session_score   SMALLINT NOT NULL DEFAULT 0 CHECK (best_session_score BETWEEN 0 AND 10),
    best_stars           SMALLINT NOT NULL DEFAULT 0 CHECK (best_stars BETWEEN 0 AND 3),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (student_id, skill_code)
);

CREATE TABLE IF NOT EXISTS training_practice_sessions (
    id            TEXT PRIMARY KEY,
    student_id    UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    skill_code    TEXT NOT NULL,
    grade         SMALLINT NOT NULL CHECK (grade BETWEEN 1 AND 6),
    mode          TEXT NOT NULL,
    seed          BIGINT NOT NULL,
    answered      JSONB NOT NULL DEFAULT '{}'::jsonb,
    correct_count SMALLINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS training_practice_sessions;
DROP TABLE IF EXISTS training_skill_stats;
DROP TABLE IF EXISTS daily_orbit_prefs;
ALTER TABLE student_streaks
    DROP COLUMN IF EXISTS reminder_time_local,
    DROP COLUMN IF EXISTS today_local_date,
    DROP COLUMN IF EXISTS today_correct_count;
