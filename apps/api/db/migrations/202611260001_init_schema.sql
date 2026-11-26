-- +goose Up
-- PlanetLogi Core PostgreSQL 16 Schema (v1, Multi-Subject Ready, Grades 1-6)

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 1. Identity, Schools & Subscriptions
CREATE TABLE user_accounts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role             TEXT NOT NULL CHECK (role IN ('PARENT', 'TEACHER', 'SCHOOL_ADMIN', 'ADMIN', 'CONTENT_EDITOR')),
    email            TEXT UNIQUE,
    phone            TEXT UNIQUE,
    display_name     TEXT NOT NULL,
    password_hash    TEXT NOT NULL,
    otp_enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    preferred_locale TEXT NOT NULL DEFAULT 'en' CHECK (preferred_locale IN ('en', 'id')),
    timezone         TEXT NOT NULL DEFAULT 'Asia/Jakarta',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE schools (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    TEXT NOT NULL,
    country_code            TEXT NOT NULL DEFAULT 'ID',
    academic_rollover_month SMALLINT NOT NULL DEFAULT 7 CHECK (academic_rollover_month BETWEEN 1 AND 12),
    academic_rollover_day   SMALLINT NOT NULL DEFAULT 1 CHECK (academic_rollover_day BETWEEN 1 AND 28),
    license_seats           INTEGER NOT NULL DEFAULT 0,
    active_until            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE classrooms (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id           UUID NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    teacher_id          UUID REFERENCES user_accounts(id) ON DELETE SET NULL,
    name                TEXT NOT NULL,
    grade               SMALLINT NOT NULL CHECK (grade BETWEEN 1 AND 6),
    academic_year       SMALLINT NOT NULL,
    leaderboard_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE student_profiles (
    id                               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_account_id                UUID NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
    school_id                        UUID REFERENCES schools(id) ON DELETE SET NULL,
    classroom_id                     UUID REFERENCES classrooms(id) ON DELETE SET NULL,
    login_id                         TEXT NOT NULL UNIQUE,
    password_hash                    TEXT NOT NULL,
    first_name                       TEXT NOT NULL,
    masked_display_name              TEXT NOT NULL,
    grade                            SMALLINT NOT NULL CHECK (grade BETWEEN 1 AND 6),
    birthday                         DATE,
    gender                           TEXT CHECK (gender IN ('MALE', 'FEMALE', 'UNSPECIFIED')),
    preferred_locale                 TEXT NOT NULL DEFAULT 'en' CHECK (preferred_locale IN ('en', 'id')),
    timezone                         TEXT NOT NULL DEFAULT 'Asia/Jakarta',
    avatar_pack_id                   TEXT NOT NULL DEFAULT 'space_pets',
    avatar_item_id                   TEXT NOT NULL DEFAULT 'astro_fox',
    astro_buddy_config               JSONB NOT NULL DEFAULT '{}'::jsonb,
    astro_buddy_name                 TEXT NOT NULL DEFAULT 'Starry Pilot',
    crew_code                        CHAR(9) NOT NULL UNIQUE,
    daily_orbit_tier                 TEXT NOT NULL DEFAULT 'ELEMENTARY' CHECK (daily_orbit_tier IN ('ELEMENTARY', 'INTERMEDIATE', 'ADVANCED', 'MASTER')),
    active_daily_curricula_by_subject JSONB NOT NULL DEFAULT '{"math":["SINGAPORE_MOE","HOTS_WHEEL"]}'::jsonb,
    active_duel_curricula_by_subject  JSONB NOT NULL DEFAULT '{"math":["SINGAPORE_MOE"]}'::jsonb,
    brain_galaxy_daily_cap_sec       INTEGER NOT NULL DEFAULT 1500 CHECK (brain_galaxy_daily_cap_sec BETWEEN 0 AND 1500),
    brain_galaxy_window_start        TIME NOT NULL DEFAULT '08:00:00',
    brain_galaxy_window_end          TIME NOT NULL DEFAULT '20:00:00',
    next_promotion_date              DATE,
    created_at                       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_student_profiles_parent ON student_profiles(parent_account_id);
CREATE INDEX idx_student_profiles_school_grade ON student_profiles(school_id, grade);
CREATE INDEX idx_student_profiles_classroom ON student_profiles(classroom_id);

CREATE TABLE subscriptions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type           TEXT NOT NULL CHECK (owner_type IN ('FAMILY', 'SCHOOL')),
    owner_id             UUID NOT NULL,
    plan_code            TEXT NOT NULL,
    status               TEXT NOT NULL CHECK (status IN ('ACTIVE', 'TRIALING', 'PAST_DUE', 'CANCELED', 'EXPIRED')),
    provider             TEXT,
    external_id          TEXT,
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end   TIMESTAMPTZ NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Subjects, Curricula, Questions & Attempts (Multi-Subject Ready)
CREATE TABLE subjects (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code         TEXT NOT NULL UNIQUE,
    translations JSONB NOT NULL,
    is_active    BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order   SMALLINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE curricula (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id   UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    code         TEXT NOT NULL UNIQUE,
    translations JSONB NOT NULL,
    sort_order   SMALLINT NOT NULL DEFAULT 0,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE topics (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    curriculum_id UUID NOT NULL REFERENCES curricula(id) ON DELETE CASCADE,
    grade         SMALLINT NOT NULL CHECK (grade BETWEEN 1 AND 6),
    category      TEXT NOT NULL CHECK (category IN ('CORE', 'WORD_PROBLEM', 'CHALLENGING', 'HOTS', 'EXPLORER')),
    translations  JSONB NOT NULL,
    sort_order    SMALLINT NOT NULL DEFAULT 0
);

CREATE TABLE skills (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_code      TEXT NOT NULL,
    topic_id            UUID NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    difficulty          SMALLINT NOT NULL CHECK (difficulty BETWEEN 1 AND 5),
    heuristic_code      TEXT,
    age_band            TEXT CHECK (age_band IN ('6-9', '8-11', '10-13')),
    video_tutorial_urls JSONB NOT NULL DEFAULT '{}'::jsonb,
    translations        JSONB NOT NULL
);

CREATE INDEX idx_skills_canonical ON skills(canonical_code);
CREATE INDEX idx_skills_topic ON skills(topic_id);

CREATE TABLE questions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            TEXT NOT NULL UNIQUE,
    subject_id      UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    skill_id        UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('STATIC', 'PROCEDURAL')),
    grade           SMALLINT NOT NULL CHECK (grade BETWEEN 1 AND 6),
    difficulty      SMALLINT NOT NULL CHECK (difficulty BETWEEN 1 AND 5),
    input_type      TEXT NOT NULL,
    heuristic_code  TEXT,
    template_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    translations    JSONB NOT NULL,
    media           JSONB NOT NULL DEFAULT '{}'::jsonb,
    grading_rule    JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'IN_REVIEW', 'PUBLISHED', 'ARCHIVED')),
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_questions_subject_grade_status ON questions(subject_id, grade, status);
CREATE INDEX idx_questions_skill ON questions(skill_id);

CREATE TABLE question_attempts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id       UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    subject_id       UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    question_id      UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    question_seed    BIGINT,
    skill_id         UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    mode             TEXT NOT NULL CHECK (mode IN ('DAILY_ORBIT', 'GALAXY_TRAINING', 'SPACE_ACADEMY', 'ASTRO_QUIZ', 'ASSIGNMENT', 'STAR_DUEL', 'SUNDAY_METEOR')),
    session_id       UUID,
    submitted_answer JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_correct       BOOLEAN NOT NULL,
    is_skipped       BOOLEAN NOT NULL DEFAULT FALSE,
    is_first_check   BOOLEAN NOT NULL DEFAULT TRUE,
    time_spent_ms    INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_attempts_student_skill ON question_attempts(student_id, skill_id, created_at DESC);

CREATE TABLE student_skill_stats (
    student_id          UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    skill_id            UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    recent_window_count SMALLINT NOT NULL DEFAULT 0,
    recent_correct_count SMALLINT NOT NULL DEFAULT 0,
    proficiency_pct     SMALLINT NOT NULL DEFAULT 0 CHECK (proficiency_pct BETWEEN 0 AND 100),
    proficiency_tier    TEXT NOT NULL DEFAULT 'INCOMPLETE' CHECK (proficiency_tier IN ('INCOMPLETE', 'BEGINNING', 'COMPETENT', 'MASTERY')),
    best_session_score  SMALLINT NOT NULL DEFAULT 0 CHECK (best_session_score BETWEEN 0 AND 10),
    best_stars          SMALLINT NOT NULL DEFAULT 0 CHECK (best_stars BETWEEN 0 AND 3),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (student_id, skill_id)
);

-- 3. Daily Orbit, Streaks, Space Academy, AstroQuiz & Assignments
CREATE TABLE daily_orbit_sessions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id           UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    subject_id           UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    local_date           DATE NOT NULL,
    tier                 TEXT NOT NULL CHECK (tier IN ('ELEMENTARY', 'INTERMEDIATE', 'ADVANCED', 'MASTER')),
    questions_json       JSONB NOT NULL,
    change_question_used SMALLINT NOT NULL DEFAULT 0 CHECK (change_question_used BETWEEN 0 AND 3),
    correct_count        SMALLINT NOT NULL DEFAULT 0,
    total_sp_earned      INTEGER NOT NULL DEFAULT 0,
    completed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (student_id, subject_id, local_date)
);

CREATE TABLE student_streaks (
    student_id                UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    subject_id                UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    current_streak            INTEGER NOT NULL DEFAULT 0,
    longest_streak            INTEGER NOT NULL DEFAULT 0,
    cryo_shields              SMALLINT NOT NULL DEFAULT 1 CHECK (cryo_shields BETWEEN 0 AND 3),
    nova_pet_stage            SMALLINT NOT NULL DEFAULT 1 CHECK (nova_pet_stage BETWEEN 1 AND 5),
    last_completed_local_date DATE,
    reminder_enabled          BOOLEAN NOT NULL DEFAULT FALSE,
    push_subscription         JSONB,
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (student_id, subject_id)
);

CREATE TABLE streak_calendar_days (
    student_id UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    subject_id UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    local_date DATE NOT NULL,
    status     TEXT NOT NULL CHECK (status IN ('COMPLETED', 'FROZEN', 'MISSED')),
    PRIMARY KEY (student_id, subject_id, local_date)
);

CREATE TABLE academy_topics (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id   UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    grade        SMALLINT NOT NULL CHECK (grade BETWEEN 1 AND 6),
    translations JSONB NOT NULL,
    sort_order   SMALLINT NOT NULL DEFAULT 0
);

CREATE TABLE academy_lessons (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id                 UUID NOT NULL REFERENCES academy_topics(id) ON DELETE CASCADE,
    age_band                 TEXT NOT NULL,
    translations             JSONB NOT NULL,
    story_video_urls         JSONB NOT NULL DEFAULT '{}'::jsonb,
    concept_video_urls       JSONB NOT NULL DEFAULT '{}'::jsonb,
    do_and_learn_question_ids UUID[] NOT NULL DEFAULT '{}',
    self_check_items         JSONB NOT NULL DEFAULT '[]'::jsonb,
    sort_order               SMALLINT NOT NULL DEFAULT 0
);

CREATE TABLE academy_lesson_progress (
    student_id          UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    lesson_id           UUID NOT NULL REFERENCES academy_lessons(id) ON DELETE CASCADE,
    stage_states        JSONB NOT NULL DEFAULT '{}'::jsonb,
    saved_draft_answers JSONB NOT NULL DEFAULT '{}'::jsonb,
    star_scrolls_earned SMALLINT NOT NULL DEFAULT 0,
    completion_pct      SMALLINT NOT NULL DEFAULT 0 CHECK (completion_pct BETWEEN 0 AND 100),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (student_id, lesson_id)
);

CREATE TABLE astro_quiz_papers (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id   UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    category     TEXT NOT NULL CHECK (category IN ('ESSENTIAL', 'TOP_SCHOOL', 'HOTS')),
    grade        SMALLINT NOT NULL CHECK (grade BETWEEN 1 AND 6),
    code         TEXT NOT NULL UNIQUE,
    question_ids UUID[] NOT NULL DEFAULT '{}',
    translations JSONB NOT NULL
);

CREATE TABLE astro_quiz_attempts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id          UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    paper_id            UUID NOT NULL REFERENCES astro_quiz_papers(id) ON DELETE CASCADE,
    status              TEXT NOT NULL DEFAULT 'IN_PROGRESS' CHECK (status IN ('IN_PROGRESS', 'SUBMITTED')),
    elapsed_sec         INTEGER NOT NULL DEFAULT 0,
    draft_answers       JSONB NOT NULL DEFAULT '{}'::jsonb,
    score               SMALLINT NOT NULL DEFAULT 0,
    submitted_at        TIMESTAMPTZ,
    supply_pod_week_key TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE assignments (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id       UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    assigned_by_role TEXT NOT NULL CHECK (assigned_by_role IN ('SYSTEM', 'PARENT', 'TEACHER')),
    assigned_by_id   UUID,
    title            TEXT NOT NULL,
    question_ids     UUID[] NOT NULL DEFAULT '{}',
    due_date         TIMESTAMPTZ,
    academic_year    SMALLINT NOT NULL,
    reward_ac        INTEGER NOT NULL DEFAULT 5,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE student_assignments (
    assignment_id  UUID NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
    student_id     UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT 'INCOMPLETE' CHECK (status IN ('INCOMPLETE', 'COMPLETE')),
    progress_count SMALLINT NOT NULL DEFAULT 0,
    score          SMALLINT NOT NULL DEFAULT 0,
    completed_at   TIMESTAMPTZ,
    PRIMARY KEY (assignment_id, student_id)
);

-- 4. Economy, Stories, Badges, Social, Super Nova, Brain Galaxy & Audit
CREATE TABLE student_wallets (
    student_id         UUID PRIMARY KEY REFERENCES student_profiles(id) ON DELETE CASCADE,
    total_sp           BIGINT NOT NULL DEFAULT 0 CHECK (total_sp >= 0),
    astro_coins        BIGINT NOT NULL DEFAULT 0 CHECK (astro_coins >= 0),
    pilot_xp           BIGINT NOT NULL DEFAULT 0 CHECK (pilot_xp >= 0),
    pilot_level        SMALLINT NOT NULL DEFAULT 1 CHECK (pilot_level BETWEEN 1 AND 100),
    total_star_scrolls INTEGER NOT NULL DEFAULT 0 CHECK (total_star_scrolls >= 0),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE currency_transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id      UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    subject_id      UUID REFERENCES subjects(id) ON DELETE SET NULL,
    currency        TEXT NOT NULL CHECK (currency IN ('SP', 'AC', 'XP', 'SCROLL', 'S_PTS')),
    delta           BIGINT NOT NULL,
    balance_after   BIGINT NOT NULL CHECK (balance_after >= 0),
    source_type     TEXT NOT NULL,
    source_id       TEXT NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_currency_tx_student ON currency_transactions(student_id, created_at DESC);

CREATE TABLE story_planets (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id   UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    grade        SMALLINT NOT NULL CHECK (grade BETWEEN 1 AND 6),
    code         TEXT NOT NULL UNIQUE,
    translations JSONB NOT NULL
);

CREATE TABLE story_chapters (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    planet_id      UUID NOT NULL REFERENCES story_planets(id) ON DELETE CASCADE,
    chapter_index  SMALLINT NOT NULL,
    unlock_cost_ac INTEGER NOT NULL DEFAULT 20,
    translations   JSONB NOT NULL,
    pages_json     JSONB NOT NULL DEFAULT '[]'::jsonb,
    UNIQUE (planet_id, chapter_index)
);

CREATE TABLE student_story_unlocks (
    student_id     UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    chapter_id     UUID NOT NULL REFERENCES story_chapters(id) ON DELETE CASCADE,
    last_page_read SMALLINT NOT NULL DEFAULT 1,
    unlocked_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (student_id, chapter_id)
);

CREATE TABLE daily_badge_templates (
    day_of_year  SMALLINT PRIMARY KEY CHECK (day_of_year BETWEEN 1 AND 366),
    translations JSONB NOT NULL,
    artwork_url  TEXT NOT NULL
);

CREATE TABLE student_daily_badges (
    student_id      UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    local_date      DATE NOT NULL,
    tier            SMALLINT NOT NULL CHECK (tier BETWEEN 1 AND 4),
    unlocked_via_ac BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (student_id, local_date)
);

CREATE TABLE star_crews (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  TEXT NOT NULL,
    created_by_student_id UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE star_crew_members (
    crew_id    UUID NOT NULL REFERENCES star_crews(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (crew_id, student_id)
);

CREATE TABLE star_duels (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id       UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    challenger_id    UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    opponent_id      UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    match_type       TEXT NOT NULL CHECK (match_type IN ('CLASSMATE', 'CREW', 'RANDOM')),
    grade            SMALLINT NOT NULL CHECK (grade BETWEEN 1 AND 6),
    question_ids     JSONB NOT NULL DEFAULT '[]'::jsonb,
    challenger_score SMALLINT NOT NULL DEFAULT 0,
    opponent_score   SMALLINT NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'COMPLETED', 'EXPIRED')),
    local_date       DATE NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE super_nova_attempts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id  UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    subject_id  UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    mode        TEXT NOT NULL CHECK (mode IN ('SUPER_SIGHT', 'SUPER_WARP')),
    is_practice BOOLEAN NOT NULL DEFAULT FALSE,
    score       INTEGER NOT NULL DEFAULT 0,
    week_key    TEXT NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE brain_galaxy_sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id     UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    subject_id     UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    zone           TEXT NOT NULL CHECK (zone IN ('memory', 'flexibility', 'speed', 'problem_solving', 'adventure')),
    game_code      TEXT NOT NULL,
    local_date     DATE NOT NULL,
    seconds_played INTEGER NOT NULL DEFAULT 0,
    high_score     INTEGER NOT NULL DEFAULT 0,
    entry_paid_ac  BOOLEAN NOT NULL DEFAULT FALSE,
    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at       TIMESTAMPTZ
);

CREATE INDEX idx_brain_galaxy_student_date ON brain_galaxy_sessions(student_id, local_date);

CREATE TABLE question_feedback (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id  UUID NOT NULL REFERENCES student_profiles(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    seed        BIGINT,
    locale      TEXT NOT NULL CHECK (locale IN ('en', 'id')),
    category    TEXT NOT NULL,
    message     TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'FIXED', 'DISMISSED')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    UUID NOT NULL,
    actor_role  TEXT NOT NULL,
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    diff_json   JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS audit_logs, question_feedback, brain_galaxy_sessions, super_nova_attempts,
    star_duels, star_crew_members, star_crews, student_daily_badges, daily_badge_templates,
    student_story_unlocks, story_chapters, story_planets, currency_transactions, student_wallets,
    student_assignments, assignments, astro_quiz_attempts, astro_quiz_papers,
    academy_lesson_progress, academy_lessons, academy_topics, streak_calendar_days,
    student_streaks, daily_orbit_sessions, student_skill_stats, question_attempts, questions,
    skills, topics, curricula, subjects, subscriptions, student_profiles, classrooms, schools,
    user_accounts CASCADE;
