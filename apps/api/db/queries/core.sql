-- name: ListActiveSubjects :many
SELECT id, code, translations, is_active, sort_order
FROM subjects
WHERE is_active = TRUE
ORDER BY sort_order ASC;

-- name: GetStudentByLoginID :one
SELECT id, parent_account_id, school_id, classroom_id, login_id, password_hash,
       first_name, masked_display_name, grade, preferred_locale, timezone,
       avatar_pack_id, avatar_item_id, astro_buddy_name, crew_code, daily_orbit_tier
FROM student_profiles
WHERE login_id = $1
LIMIT 1;

-- name: GetUserAccountByEmail :one
SELECT id, role, email, phone, display_name, password_hash, otp_enabled, preferred_locale, timezone
FROM user_accounts
WHERE lower(email) = lower($1)
LIMIT 1;

-- name: GetStudentWalletForUpdate :one
SELECT student_id, total_sp, astro_coins, pilot_xp, pilot_level, total_star_scrolls
FROM student_wallets
WHERE student_id = $1
FOR UPDATE;

-- name: ListCurriculaBySubject :many
SELECT id, subject_id, code, translations, sort_order, is_active
FROM curricula
WHERE subject_id = $1 AND is_active = TRUE
ORDER BY sort_order ASC;
