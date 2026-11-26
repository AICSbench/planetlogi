# 05 — Gamification Economy, Social Mechanics & Child Safety

> **Owner Tracks**: `BE`, `GAMIFICATION`, `SECURITY`, `QA`  
> **Scope**: Dual-currency economy, XP/Level curve, Streak/Cryo Shield logic, Daily Badge rarity system, Healthy Screen-Time & Operating-Hours engine, Child Privacy (UU PDP / COPPA), and Anti-Cheat controls.

---

## 1. Dual-Currency & Progression Economy

AlgiPlanet separates **competitive recognition** from **spendable rewards** so spending coins on stories or games NEVER lowers a student's leaderboard ranking.

### 1.1 Currencies & Tokens
| Asset | Code | Spendable? | How Earned | How Used |
|---|---|---|---|---|
| **Star Points** | `SP` | **No** (Cumulative + Daily/Weekly/Quarterly snapshots) | Daily Orbit (up to 17 SP/day), Star Duels (1–5 SP/duel), Sunday Meteor Challenge (up to 20 SP) | Unlocks Daily Orbit difficulty tiers (15/50/100 SP), drives Galaxy Leaderboards & Galaxy Showcase eligibility (`>=1 SP`) |
| **Astro Coins** | `AC` | **Yes** (Double-entry ledger) | Daily Orbit completion (+5 AC), AstroQuiz Weekly Supply Pod (up to +5 AC/wk), Assignments (+3–5 AC), Daily Bonus Gift (+1–3 AC), Streak Milestones | Unlock Planet Stories chapters (**20 AC**), Brain Galaxy game entry (**1 AC/game/day**), Recover missed Daily Star Badges (**20 AC**), Buy Cryo Shield top-up (optional, capped at max 3) |
| **Pilot XP** | `XP` | **No** (Monotonic) | Every answered question (+1 XP attempt, +2 XP correct), completing lessons, quizzes, and duels | Advances **Pilot Level 1 → 100** and unlocks cosmic rank titles & celebratory badges |
| **Star Scrolls** | `SCROLL` | **No** (Per topic) | Completing Space Academy *Do & Learn* guided drills | Tracks Space Academy topic mastery & unlocks topic mastery crests |
| **Super Diamond** | `S_PTS` | **No** (Weekly reset) | Super Sight (10 pts/diff) + Super Warp (1 pt/correct) | Ranks students on weekly *Super Nova* leaderboard |

### 1.2 Pilot XP Level Curve (Levels 1–100)
- Early levels trigger rapidly so Grade 1–6 newcomers feel immediate momentum (`Lvl 1 → 2` requires **4 XP**; `Lvl 2 → 3` requires **10 XP**).
- Formula for Level `L` (`1 <= L < 100`):
  - `xpRequired(1) = 4`
  - `xpRequired(L) = Math.round(4 + 6 * (L - 1) + 1.8 * Math.pow(L - 1, 1.55))`
- Every 10 levels unlocks a Cosmic Rank Title (bilingual `en`/`id`), e.g.:
  - Lvl 1–9: *Star Cadet / Kadet Bintang*
  - Lvl 10–19: *Orbit Scout / Pengintai Orbit*
  - Lvl 20–29: *Comet Pilot / Pilot Komet*
  - ...
  - Lvl 90–99: *Nebula Admiral / Laksamana Nebula*
  - Lvl 100: *AlgiPlanet Grand Commander / Panglima Besar AlgiPlanet*

### 1.3 Ledger Integrity Rules
- Every SP, AC, XP, or Scroll mutation writes an immutable `CurrencyTransaction` row with a unique `idempotencyKey` (e.g., `daily_orbit:2026-11-26:student_123:q9` or `story_unlock:planet_1:ch_2:student_123`).
- Spending AC executes inside a PostgreSQL row-locked transaction (`SELECT ... FOR UPDATE`) verifying `balance >= cost` before debiting.

---

## 2. Orbit Streak, Nova Pet & Daily Star Badges

### 2.1 Orbit Streak State Machine
Evaluated per calendar day in the student's configured IANA timezone (e.g., `Asia/Jakarta`, `Asia/Singapore`, `UTC`):
- **Trigger**: When `dailyOrbitCorrectCount >= 5` on day `D`, and day `D` has not yet been credited:
  - `currentStreak += 1`
  - `longestStreak = Math.max(longestStreak, currentStreak)`
  - `novaPetGrowthStage` updates based on `currentStreak` (Stage 1: 1–2d, Stage 2: 3–6d, Stage 3: 7–13d, Stage 4: 14–29d, Stage 5: 30+d).
- **Midnight Reconciliation Job**:
  - For any active streak where yesterday (`D - 1`) did not reach 5 correct answers:
    - If `cryoShieldCount > 0`: decrement `cryoShieldCount -= 1`, mark day `D - 1` as `FROZEN` on the monthly calendar, keep `currentStreak` intact.
    - Else: reset `currentStreak = 0`, mark day `D - 1` as `MISSED`.
- **Cryo Shield Cap**: Hard cap of **3** stored Cryo Shields per student.

### 2.2 Daily Star Badges (Collectible Trivia Cards)
- Each calendar date maps to a unique bilingual space/math trivia card (`365/366` curated entries).
- **4 Rarity Tiers**:
  - **1★ Bronze**: Complete Daily Orbit (10/10 attempted, or 5+ correct)
  - **2★ Silver**: Daily Orbit with `>= 8/10` accuracy (or 2 subjects once expanded)
  - **3★ Gold**: Daily Orbit `10/10` OR Daily Orbit + Super Nova / AstroQuiz milestone on same day
  - **4★ Holographic**: Complete Daily Orbit + Star Duel + Super Nova (or all active daily subjects) on the same day
- **Retroactive Unlock**: Any missed day in the current or past calendar month can be unlocked at **1★ Bronze** by spending **20 AC**.

---

## 3. Healthy Social & Fair Competition Rules

- **Zero Free-Text Child-to-Child Chat**: Prevents cyberbullying, grooming, and PII sharing. All social interactions use structured game actions, preset cosmic reactions (e.g., *"Great Duel!"*, *"Star Job!"*), 9-digit **Star Crew Codes**, and preset 2-part **Astro Buddy** adjective names.
- **Star Duels Anti-Farming**:
  - Hard cap of **3 energy cells/day** per student.
  - Cannot duel the exact same classmate more than once per day for SP.
  - Random matchmaking pairs students of the **same grade** and similar skill band, returning masked display info only.
- **Leaderboard Fairness & Privacy**:
  - Snapshots refresh every **5 minutes** via scheduled Redis aggregation so rapid polling cannot overload PostgreSQL.
  - All leaderboard entries mask surnames (e.g., `Amr P.` or `A** P****` depending on school privacy setting) and never expose school address, birth date, or login ID.
  - Dedicated **Nearby Pilots** view (`±5` ranks) keeps motivation high even for students outside the Top 50.

---

## 4. Healthy Screen-Time & Operating-Hours Engine

A central `TimeWindowService` enforces healthy study-play rhythms server-side:

| Feature | Default Hours (Student Local TZ) | Daily Cap | Parent Override |
|---|---|---|---|
| **Daily Orbit** | `06:00 – 22:00` | 10 questions/day | Can tighten window |
| **Super Nova Challenge** | `06:00 – 22:00` (Mon–Sat) | Unlimited practice; ranked attempts tracked | Can disable/tighten |
| **Sunday Meteor Challenge** | `06:00 – 22:00` (Sunday only) | 10 questions | — |
| **Brain Galaxy Mini-Games** | `08:00 – 18:00` (School) / `08:00 – 20:00` (Individual) | **25 minutes/day hard cap** + 1 AC/game/day | Parent can reduce cap (e.g., `0–25 min`) or narrow hours |
| **Galaxy Training, Space Academy, Stories, AstroQuiz** | `24/7` (review/learning never blocked, unless parent sets bedtime lock) | None | Optional bedtime lock |

### Brain Galaxy 25-Minute Heartbeat Enforcement
1. Opening a Brain Galaxy game calls `POST /api/v1/brain-galaxy/sessions/start` (verifies operating hours, remaining seconds `> 0`, and 1 AC daily game entry).
2. Client sends heartbeat `POST /api/v1/brain-galaxy/sessions/:id/heartbeat` every **30 seconds**; server increments `secondsUsedToday` in Redis/PostgreSQL and returns authoritative `remainingSeconds`.
3. When `remainingSeconds <= 0`, server closes the session and rejects further score submissions for that day; UI displays a warm *"Great brain workout today! Your 25-minute Brain Galaxy timer resets tomorrow."* screen.

---

## 5. Security, Child Privacy & Anti-Cheat

### 5.1 Child Data Protection (UU PDP Indonesia & COPPA Principles)
- **Minimal Data Collection**: Child accounts store only First Name, Grade, School (optional), Birth Month/Year (or Date gated to parent), Gender (optional), and Avatar/Buddy config. No third-party ad trackers or behavioral ad SDKs anywhere in the app.
- **Parental Consent & Control**: Parent account holds the email/phone, billing relationship, OTP security toggle, and full right to **Export Child Data** or **Permanently Delete Account** (hard-deletes PII within 30 days).
- **No Direct Child Monetization Pressure**: Subscription upgrade buttons inside the student view ALWAYS trigger a **Parent Gate** (parent login / OTP / math+PIN gate) before showing checkout.

### 5.2 Anti-Cheat & API Integrity
- **Server-Side Grading Only**: Correct answer keys and worked solutions are NEVER transmitted to the client prior to submission (except after explicit Skip or Space Academy retry feedback).
- **Single-Attempt Idempotency**: Daily Orbit questions enforce a database unique constraint on `(dailyOrbitSessionId, questionIndex)` once submitted; retries return `409 Conflict`.
- **Rate Limiting & Bot Protection**: Strict per-user/IP rate limits on Login, OTP verify (max 5 attempts / 15 min), Star Duel matchmaking, and submission endpoints (flags impossibly fast `<1.5s` multi-step answers for anomaly review without falsely banning fast arithmetic kids).
- **Parent OTP Gate**: Enabling OTP Security in Settings requires verifying a 6-digit time-limited code (Email/SMS/WhatsApp) before modifying password, linked parent contact, curriculum locks, or billing.
