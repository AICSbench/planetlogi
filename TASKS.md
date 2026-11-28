# PlanetLogi — Master Parallel Task Board

> **INSTRUCTIONS FOR AGENTS (MANDATORY)**
> 1. Read [`AGENTS.md`](./AGENTS.md) before claiming any task.
> 2. Pick an unclaimed task (`[ ]`) whose `Depends On` tasks are ALL `[x]`.
> 3. **Mark as In Progress (`[~]`) BEFORE starting work**: replace `[ ]` with `[~]` and append `*(Owner: <your-agent-id> | Started: YYYY-MM-DDTHH:MMZ)*`.
> 4. **Mark as Completed (`[x]`) ONLY AFTER verification passes**: replace `[~]` with `[x]`, append `*(Owner: <your-agent-id> | Done: YYYY-MM-DDTHH:MMZ)*`, and log a summary in **Section 7 (Handoff & Activity Log)**.
> 5. **If Blocked**, mark `[!]` with the exact reason and log it in Section 7.

---

## Status Legend & Phase Progress

- `[ ]` Unclaimed / Pending
- `[~]` In Progress (Locked by an agent)
- `[x]` Completed & Verified
- `[!]` Blocked

| Phase | Name | Total Tasks | Done | Status |
|---|---|---:|---:|---|
| Phase 0 | Foundation & Architecture | 8 | 8 | Completed |
| Phase 1 | Core Learning Engine & Daily Orbit (MVP) | 12 | 12 | Completed |
| Phase 2 | Space Academy, AstroQuiz & Parent Portal | 10 | 10 | Completed |
| Phase 3 | Gamification, Planet Stories, Social & Events | 13 | 13 | Completed |
| Phase 4 | Brain Galaxy, School/Teacher Portal & CMS v2 | 9 | 0 | Ready to Start |
| Phase 5 | Monetization, Security Audit, Performance & Launch | 7 | 0 | Not Started |
| **Total** | | **59** | **43** | **72.9%** |

---

## Phase 0 — Foundation & Architecture

> **Parallelism Note**: Once `P0-01` (Monorepo Bootstrap) is `[x]`, tasks `P0-02` through `P0-08` can run largely in parallel across `INFRA`, `DB`, `SHARED`, `UI`, and `BE` agents.

- [x] **P0-01** `[INFRA]` Bootstrap Monorepo, Tooling, Dockerfiles & CI Pipeline *(Owner: agent-infra-1 | Done: 2026-11-26T15:45Z)*
  - **Depends On**: None
  - **Spec**: `docs/08-technical-architecture-and-database.md` §1–2, §5, §7
  - **Scope**: Root `package.json`, `pnpm-workspace.yaml`, `turbo.json`, `Makefile`, `apps/web` (Vite + React 19 + TypeScript + TanStack Router + Tailwind CSS + Cloudflare Pages `_redirects`), `apps/api` (Go 1.23+ module with Chi + Huma OpenAPI 3.1 + `GET /healthz` & `GET /readyz` + graceful `SIGTERM` shutdown + `sqlc.yaml` + `asynq` worker entrypoint + multi-stage distroless Dockerfile for Coolify), `packages/{api-client,ui,i18n,brain-games,config}`, `.github/workflows/ci.yml`, `docker/docker-compose.yml` (PostgreSQL 16 + Redis 7 + MinIO S3).
  - **Acceptance Criteria**: `pnpm install`, `pnpm typecheck`, `pnpm lint`, `pnpm build`, `go vet ./...`, `go test ./...`, and `docker compose up -d` succeed cleanly; `/healthz` & `/readyz` respond properly; OpenAPI client codegen (`pnpm api:gen`) runs cleanly.

- [x] **P0-02** `[DB]` Design & Migrate Core Database Schema (v1, `goose` + `sqlc`) *(Owner: agent-db-1 | Done: 2026-11-26T15:45Z)*
  - **Depends On**: `P0-01`
  - **Spec**: `docs/08-technical-architecture-and-database.md` §3
  - **Scope**: `apps/api/db/migrations/` (`goose` SQL migrations), `apps/api/db/queries/` (`sqlc`), `apps/api/internal/db/` (generated), `apps/api/cmd/seed/` — Multi-subject-ready PostgreSQL 16 schema (`subjects` table with `math` active first + `subject_id` scoping) covering Accounts (Parent, Student, Teacher, Admin), Schools/Classes, Curricula, Topics/Skills, Questions & Templates, Attempts, Daily Orbit Sessions, Streaks, Currencies/Ledgers, Assignments, Stories, Quizzes, Badges, Avatars/Astro Buddies, Duels, Crews, Events, Brain Galaxy Sessions, Subscriptions.
  - **Acceptance Criteria**: `goose up` runs cleanly against PostgreSQL 16; `sqlc generate` compiles with zero errors; Go seed CLI populates demo school, parent, G1–G6 test student, and 6 curriculum shells.

- [x] **P0-03** `[SHARED]` Build Multi-Language i18n Framework (`en` & `id`, `i18next`) *(Owner: agent-shared-1 | Done: 2026-11-26T15:45Z)*
  - **Depends On**: `P0-01`
  - **Spec**: `docs/07-i18n-multilanguage-architecture.md`
  - **Scope**: `packages/i18n/`, `apps/web/src/i18n/` — Locale prefix routing (`/en/...`, `/id/...` via TanStack Router), `i18next` + `react-i18next` namespaced JSON loaders, typed translation hooks, math/date/currency formatters, fallback resolver, CI missing-key checker script.
  - **Acceptance Criteria**: Switching between `en` and `id` persists in cookie + user profile; missing-key check script passes in CI; localized number formatting verified by unit tests.

- [x] **P0-04** `[UI]` Create PlanetLogi Galaxy Design System & Component Library *(Owner: agent-ui-1 | Done: 2026-11-26T15:45Z)*
  - **Depends On**: `P0-01`
  - **Spec**: `docs/01-product-vision-and-theme.md` §4–6
  - **Scope**: `packages/ui/` — Galaxy color palette (deep space navy, nebula indigo, starlight gold, aurora cyan, solar coral, light notebook canvas for question focus), typography scale, kid-friendly `>=44px` buttons, cards, modals, progress bars, 5-chili/flame difficulty meter, star ratings, confetti celebration primitive, responsive breakpoints.
  - **Acceptance Criteria**: Component showcase/Storybook builds cleanly; WCAG AA contrast verified for text & math canvas; dark-space chrome + high-contrast crisp reading surface for questions.

- [x] **P0-05** `[BE]` Implement Authentication, RBAC & Child-Safe Session System *(Owner: agent-be-1 | Done: 2026-11-26T15:45Z)*
  - **Depends On**: `P0-02`
  - **Spec**: `docs/02-features-student-portal.md` §1, `docs/05-gamification-social-and-safety.md` §6, `docs/08-technical-architecture-and-database.md` §4
  - **Scope**: `apps/api/internal/modules/auth/` — Student Login ID + password auth, Parent email/phone auth, JWT + HttpOnly refresh cookies, RBAC guards (`STUDENT`, `PARENT`, `TEACHER`, `ADMIN`), Parent OTP security toggle & verification service, rate limiting.
  - **Acceptance Criteria**: Login/logout/refresh/OTP flows covered by integration tests; student cannot access parent/teacher/admin endpoints.

- [x] **P0-06** `[FE]` Build Login Page (with Live Star Points Feed) & Persistent App Shell *(Owner: agent-fe-1 | Done: 2026-11-26T15:45Z)*
  - **Depends On**: `P0-03`, `P0-04`, `P0-05`
  - **Spec**: `docs/02-features-student-portal.md` §1–2
  - **Scope**: `apps/web/src/routes/$locale/(auth)/login/`, `apps/web/src/components/shell/` — Split login screen (live recent Star Points ticker on left, Login ID/Password with Show toggle on right), persistent responsive top nav bar (PlanetLogi logo, Subject/Plan switcher, Home, Report, Leaderboard, Achievement, Events, Streak counter, Bonus Gift, Notification Bell, 9-Dot App Launcher, Account dropdown, overflow menu on `<1440px`).
  - **Acceptance Criteria**: Fully responsive at 375px, 768px, 1024px, 1440px+; both `en` & `id` work; keyboard-accessible menus.

- [x] **P0-07** `[INFRA]` Configure Static PWA (`vite-plugin-pwa`), Cloudflare Pages & Offline Shell *(Owner: agent-infra-1 | Done: 2026-11-26T15:45Z)*
  - **Depends On**: `P0-04`, `P0-06`
  - **Spec**: `docs/01-product-vision-and-theme.md` §5, `docs/08-technical-architecture-and-database.md` §5–6
  - **Scope**: `apps/web/vite.config.ts` (`vite-plugin-pwa` / Workbox), `apps/web/public/_redirects` (Cloudflare Pages SPA fallback), galaxy app icons, service worker precaching strategy (app shell, static assets, offline fallback screen, IndexedDB draft backup).
  - **Acceptance Criteria**: Static `dist/` deploys cleanly to Cloudflare Pages; Lighthouse PWA installability check passes on iPadOS Safari, Chrome Desktop, and Android Chrome.

- [x] **P0-08** `[BE]` Build Timezone, Operating-Hours & Currency Ledger Core Services (Go) *(Owner: agent-be-2 | Done: 2026-11-26T15:45Z)*
  - **Depends On**: `P0-02`
  - **Spec**: `docs/05-gamification-social-and-safety.md` §1, §4
  - **Scope**: `apps/api/internal/platform/economy/`, `apps/api/internal/platform/timewindow/` — Atomic PostgreSQL `SELECT ... FOR UPDATE` double-entry ledger for **Star Points (SP)**, **Astro Coins (AC)**, **Pilot XP (Lvl 1–100 curve)**, and **Star Scrolls**; IANA timezone-aware operating window validator (e.g., 06:00–22:00).
  - **Acceptance Criteria**: Concurrent reward grants are idempotent (`go test -race` passes with zero double-crediting); XP level-up triggers cleanly across Lvl 1–100; unit tests cover timezone boundaries & DST.

---

## Phase 1 — Core Learning Engine & Daily Orbit (MVP)

> **Parallelism Note**: `P1-ENG-01`, `P1-BE-01`, `P1-BE-02`, `P1-BE-03`, `P1-BE-04`, and `P1-CNT-01..03` can run concurrently once Phase 0 core tasks finish.

- [x] **P1-ENG-01** `[ENGINE]` Build Server-Authoritative Question Engine, Evaluator & Procedural Generator (Go) *(Owner: agent-engine-1 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P0-01`, `P0-03`
  - **Spec**: `docs/03-question-engine-and-math-tools.md` §2–3
  - **Scope**: `apps/api/internal/mathengine/` — Bilingual question JSON schema (single numeric, multi-part numeric, letter/text, MCQ, fraction/mixed-number, drag-match, visual SVG diagram specs), deterministic seeded procedural generator (for infinite arithmetic/word-problem variants), equivalence answer grader (handles fractions, `.` and `,` decimals per locale, whitespace), and OpenAPI question response DTOs synced to `packages/api-client/`.
  - **Acceptance Criteria**: ≥20 procedural generator templates covering G1–G6 core operations pass deterministic Go snapshot tests in `en` and `id`; answer keys are never serialized in unsubmitted question payloads.

- [x] **P1-ENG-02** `[ENGINE]` Implement 4 Interactive On-Screen Math Working Tools *(Owner: agent-engine-2 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P0-04`, `P0-03`
  - **Spec**: `docs/03-question-engine-and-math-tools.md` §4
  - **Scope**: `apps/web/src/components/question-player/tools/` — (1) **Star Sketchpad** (canvas pencil, highlighter, eraser, clear), (2) **Equation Working Pad** (step-by-step math expression editor + symbol pad), (3) **Bar Model Builder** (1–20 adjustable labeled units, split/combine/bracket labels), (4) **Column 4-Operations Grid** (`+`, `−`, `×`, `÷` up to 4 digits with carry/borrow).
  - **Acceptance Criteria**: All 4 tools work with mouse, touch, and Apple Pencil/stylus on tablet without scrolling interference; collapsible right-side drawer on desktop & bottom sheet on mobile.

- [x] **P1-ENG-03** `[ENGINE]` Build Unified Question Player UI (4 Mode Behaviors + TTS) *(Owner: agent-engine-2 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P1-ENG-01`, `P1-ENG-02`
  - **Spec**: `docs/03-question-engine-and-math-tools.md` §1, §5
  - **Scope**: `apps/web/src/components/question-player/` — Notebook/cosmic focus canvas, Skill Name + `?` info, Question ID, 5-level difficulty meter, collapsible Side Nav, **TTS Read-Aloud** (`en-US`/`en-GB` & `id-ID` with server TTS fallback), Video Tutorial modal, Confetti + Visual Worked Solution viewer, Feedback/Report modal, mode-specific behaviors (`DAILY_ORBIT`, `GALAXY_TRAINING`, `SPACE_ACADEMY`, `ASTRO_QUIZ`).
  - **Acceptance Criteria**: Each of the 4 mode behaviors passes E2E interaction tests (single-attempt lock vs Save/Try Again vs Timed/Print).

- [x] **P1-BE-01** `[BE]` Implement Daily Orbit API (10-Question Path, Tiers, 3x Change Question) *(Owner: agent-be-1 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P0-02`, `P0-05`, `P0-08`, `P1-ENG-01`
  - **Spec**: `docs/02-features-student-portal.md` §3, `docs/05-gamification-social-and-safety.md` §1–2
  - **Scope**: `apps/api/internal/modules/dailyorbit/` — 06:00–22:00 window check, 10-question session generator (Q1–3: 1 SP, Q4–5: 1 SP, Q6–8: 2 SP, Q9–10: 3 SP, completion bonus: 5 AC), 4 difficulty tiers (Elementary, Intermediate @15 SP, Advanced @50 SP, Master @100 SP), active curriculum filter, max 3/day "Change Question" (easier/harder/topic), single-attempt HTTP 409/400 lock, Skip ("Show Answer" = 0 SP), Today's SP breakdown endpoint.
  - **Acceptance Criteria**: Full integration test suite verifies scoring, tier unlocks, duplicate submission rejection, 3-edit cap, and midnight reset.

- [x] **P1-BE-02** `[BE]` Implement Orbit Streak, Nova Pet Growth & Cryo Shield API *(Owner: agent-be-1 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P0-08`, `P1-BE-01`
  - **Spec**: `docs/02-features-student-portal.md` §4, `docs/05-gamification-social-and-safety.md` §2
  - **Scope**: `apps/api/internal/modules/streak/` — Daily milestone (5 correct Daily Orbit answers advances streak & Nova Pet stage), Cryo Shield inventory (max 3, automatic midnight consumption when a day is missed), monthly calendar endpoint, web push reminder subscription endpoint.
  - **Acceptance Criteria**: Time-travel unit tests verify streak increment, auto Cryo Shield protection, streak reset when 0 shields, and max-3 cap.

- [x] **P1-BE-03** `[BE]` Implement Galaxy Training, Multi-Curriculum & HOTS Wheel API *(Owner: agent-be-2 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P0-02`, `P0-08`, `P1-ENG-01`
  - **Spec**: `docs/02-features-student-portal.md` §6, `docs/04-curriculum-and-pedagogy.md` §1–3
  - **Scope**: `apps/api/internal/modules/training/` — Endpoints for 6 curriculum tracks (Kurikulum Merdeka, Singapore MOE P1–P6, Cambridge 0096, Challenging Maths, HOTS 4-Quadrant/9 Heuristics across 3 age bands 6–9/8–11/10–13, Real-World Explorer Series), 10-question practice sessions, star rating calculation, proficiency tier calculation + cached National Avg %.
  - **Acceptance Criteria**: Fast paginated skill lists (<150ms); accurate Proficiency % (Mastery/Competent/Beginning/Incomplete) and 3-star High Score updates.

- [x] **P1-BE-04** `[BE]` Implement Proficiency & High-Score Analytics Report API *(Owner: agent-be-2 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P1-BE-03`
  - **Spec**: `docs/02-features-student-portal.md` §14, `docs/04-curriculum-and-pedagogy.md` §4
  - **Scope**: `apps/api/internal/modules/reports/` — Filterable Proficiency Report across all 6 curricula & Grades 1–6 (4 mastery buckets + per-topic skill grids) and High Score Report (star tiers 9–10, 6–8, 4–5, 1–3 out of 10).
  - **Acceptance Criteria**: Proficiency % report accurately reflects student attempts across Daily Orbit and Galaxy Training; High Score star-tier report (9–10, 6–8, 4–5, 1–3 out of 10) reflects 10-question Galaxy Training sessions; response cached per student with instant invalidation on attempt.

- [x] **P1-FE-01** `[FE]` Build Student Dashboard (4-Pillar Hub) & Profile/XP Card *(Owner: agent-fe-1 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P0-06`, `P0-08`
  - **Spec**: `docs/02-features-student-portal.md` §2
  - **Scope**: `apps/web/src/routes/$locale/dashboard/` — Left Student Profile Card (Avatar, Name, Grade, Pilot XP Level badge → `/exp-level`), 4-pillar hub switcher (**Star Challenges**, **Galaxy Training**, **Space Academy**, **Cosmic Fun**), right-side Today's/Total SP & Astro Coins panel with tooltips, XP Level 1–100 progression page (`/exp-level`).
  - **Acceptance Criteria**: Smooth tab switching, live currency/XP hydration, fully responsive & bilingual.

- [x] **P1-FE-02** `[FE]` Build Daily Orbit & Orbit Streak Pages *(Owner: agent-fe-1 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P1-ENG-03`, `P1-BE-01`, `P1-BE-02`
  - **Spec**: `docs/02-features-student-portal.md` §3–4
  - **Scope**: `apps/web/src/routes/$locale/daily-orbit/`, `apps/web/src/routes/$locale/streak/` — 10-question cosmic map path (circles, hexagons, stars), 4-tier difficulty modal, Curriculum Filter popover, Today's SP breakdown dropdown, Change Question modal, Skip confirmation dialog, Orbit Streak page (Nova Pet visual growth, Cryo Shield modal, monthly calendar, Push Reminder toggle).
  - **Acceptance Criteria**: End-to-end Daily Orbit completion awards SP, AC, XP, and advances streak on 5th correct answer.

- [x] **P1-FE-03** `[FE]` Build Galaxy Training, HOTS 4-Quadrant Wheel & Reports UI *(Owner: agent-fe-2 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P1-ENG-03`, `P1-BE-03`, `P1-BE-04`
  - **Spec**: `docs/02-features-student-portal.md` §6, §14
  - **Scope**: `apps/web/src/routes/$locale/training/`, `apps/web/src/routes/$locale/report/` — 6-track curriculum launcher, Grade switcher (G1–G6), toggle between **High Score Mode** (3-star + difficulty meter) and **Proficiency % Mode** (4 color-coded tiers + National Avg %), interactive SVG **HOTS 4-Quadrant Heuristic Wheel** with 3 age bands, Real-World Explorer Series list, Proficiency & High-Score Report pages.
  - **Acceptance Criteria**: All 6 tracks render cleanly; HOTS wheel quadrants & 9 heuristics interactive on touch & mouse; reports filter accurately.

- [x] **P1-CNT-01** `[CONTENT]` Author Bilingual Curriculum Taxonomy (G1–G6, All 6 Tracks) *(Owner: agent-content-1 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P0-02`, `P0-03`
  - **Spec**: `docs/04-curriculum-and-pedagogy.md` §1–3
  - **Scope**: `content/curricula/` — Structured JSON/YAML skill trees for Grades 1–6 across Kurikulum Merdeka, Singapore MOE, Cambridge 0096, Challenging Maths, HOTS (9 heuristics × 3 age bands), and Real-World Explorer Series, with complete `en` & `id` titles, difficulty ratings, and cross-curriculum skill mapping.
  - **Acceptance Criteria**: Seed script imports all 6 curricula cleanly with zero broken IDs or missing translations.

- [x] **P1-CNT-02** `[CONTENT]` Seed Initial Bilingual Question Bank & Generator Packs (G1–G6) *(Owner: agent-content-1 | Done: 2026-11-26T23:16Z)*
  - **Depends On**: `P1-ENG-01`, `P1-CNT-01`
  - **Spec**: `docs/03-question-engine-and-math-tools.md` §2–3, `docs/09-business-roadmap-and-operations.md` §1
  - **Scope**: `content/questions/` — Minimum 60 parametric generator templates + 300 curated static word/HOTS problems (with visual diagrams & step-by-step worked solutions) spanning Grades 1–6 in both `en` and `id`.
  - **Acceptance Criteria**: Automated validation script passes 100% of questions (valid schema, verifiable answer key, valid `en` & `id` copy, no empty solutions).

---

## Phase 2 — Space Academy, AstroQuiz, Assignments & Parent Portal

> **Parallelism Note**: Space Academy (`P2-BE-01`/`P2-FE-01`), AstroQuiz (`P2-BE-02`/`P2-FE-02`), Assignments (`P2-BE-03`/`P2-FE-03`), and Profile/Settings/Parent Portal (`P2-BE-04`/`P2-FE-04`) can run in parallel.

- [x] **P2-BE-01** `[BE]` Implement Space Academy API (Topics, Lessons, Do & Learn, Star Scrolls) *(Owner: agent-be-1 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P0-02`, `P0-08`, `P1-ENG-01`
  - **Spec**: `docs/02-features-student-portal.md` §7
  - **Scope**: `apps/api/internal/modules/academy/` — Topic list with Star Scroll progress, Lesson timeline (`STORY_LAUNCH`, `WATCH_LEARN`, `DO_LEARN`, `SELF_CHECK`), Save draft & Try Again attempt handling, Star Scroll award on drill completion, Self-Check objective persistence.
  - **Acceptance Criteria**: Save/resume works across reloads; Try Again allows re-attempting wrong boxes; Star Scrolls increment atomically.

- [x] **P2-FE-01** `[FE]` Build Space Academy UI (Topic Grid, Lesson Timeline & Do & Learn) *(Owner: agent-fe-1 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P1-ENG-03`, `P2-BE-01`
  - **Spec**: `docs/02-features-student-portal.md` §7
  - **Scope**: `apps/web/src/routes/$locale/academy/` — Topic cards with Star Scroll badges, Lesson list with age tags & completion %, Video player (bilingual caption/track support), Do & Learn Question Player with `Save` + `Check Answer` → `Try Again`, Self-Check checklist linking to Galaxy Training.
  - **Acceptance Criteria**: Complete lesson flow updates progress % and awards Star Scrolls; responsive on tablet/mobile.

- [x] **P2-BE-02** `[BE]` Implement AstroQuiz API (Exam Papers, Timer, Weekly Supply Pods) *(Owner: agent-be-2 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P0-02`, `P0-08`, `P1-ENG-01`
  - **Spec**: `docs/02-features-student-portal.md` §9
  - **Scope**: `apps/api/internal/modules/astroquiz/` — 3 paper categories across G1–G6 (*Essential Skills*, *Top School Year-End*, *HOTS Papers*), timed session state + Save draft, full-paper submission grading + solutions reveal, weekly deadline Supply Pod reward (1/week, up to 5 AC), print-ready structured payload.
  - **Acceptance Criteria**: Solutions stay hidden until full submit; weekly Supply Pod enforces 1-per-week cap; print payload renders cleanly.

- [x] **P2-FE-02** `[FE]` Build AstroQuiz UI & Printable Worksheet Renderer *(Owner: agent-fe-2 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P1-ENG-03`, `P2-BE-02`
  - **Spec**: `docs/02-features-student-portal.md` §9, `docs/03-question-engine-and-math-tools.md` §6
  - **Scope**: `apps/web/src/routes/$locale/astro-quiz/` — Category & Grade browser, weekly Supply Pod banner, 20-question Proficiency sidebar, live elapsed timer, Save/Submit modals, dedicated `@media print` / PDF worksheet view (`/astro-quiz/print/[id]`) with clean A4 pagination.
  - **Acceptance Criteria**: Printing to A4 PDF shows crisp questions, diagrams, and space for working without broken page splits or dark galaxy backgrounds.

- [x] **P2-BE-03** `[BE]` Implement Assignments & Parent Custom Assignment Builder API *(Owner: agent-be-1 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P0-02`, `P0-05`, `P1-BE-03`
  - **Spec**: `docs/02-features-student-portal.md` §10, `docs/06-portals-parent-teacher-admin.md` §1
  - **Scope**: `apps/api/internal/modules/assignments/` — Student assignment list (`All`/`Complete`/`Incomplete`, filters: Year, Assigned By `System`/`Parents`/`Teachers`), Parent Custom Assignment Builder (browse skills with child's live proficiency %, pick specific questions, set due date, assign to child), completion grading + AC reward.
  - **Acceptance Criteria**: Parent can only assign to linked children; student sees newly assigned homework immediately and earns reward upon completion.

- [x] **P2-FE-03** `[FE]` Build Student Assignments Page & Parent Custom Builder UI *(Owner: agent-fe-1 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P1-ENG-03`, `P2-BE-03`
  - **Spec**: `docs/02-features-student-portal.md` §10, `docs/06-portals-parent-teacher-admin.md` §1
  - **Scope**: `apps/web/src/routes/$locale/assignments/` — Student tabbed assignment list with filters & refresh; Parent Custom Builder allowing multi-skill question selection with proficiency badges and live preview.
  - **Acceptance Criteria**: Full create → complete → review loop passes E2E tests in both `en` and `id`.

- [x] **P2-BE-04** `[BE]` Implement Account, Profile, Settings & Parent Portal API *(Owner: agent-be-2 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P0-05`, `P1-BE-04`
  - **Spec**: `docs/02-features-student-portal.md` §15, `docs/06-portals-parent-teacher-admin.md` §1
  - **Scope**: `apps/api/internal/modules/profile/` — Parent & Child profile management, auto grade promotion metadata, password change, OTP-gated Parent Settings (language `en`/`id`, active Daily Orbit & Star Duel curricula, screen-time & operating hours preferences, notification prefs), Parent multi-child dashboard overview.
  - **Acceptance Criteria**: OTP gate enforced when enabled; curriculum/language setting changes immediately reflect in Daily Orbit and UI.

- [x] **P2-FE-04** `[FE]` Build Profile, Settings, Help & Parent Portal Dashboard UI *(Owner: agent-fe-2 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P0-06`, `P2-BE-04`
  - **Spec**: `docs/02-features-student-portal.md` §15, `docs/06-portals-parent-teacher-admin.md` §1
  - **Scope**: `apps/web/src/routes/$locale/profile-settings/`, `apps/web/src/routes/$locale/parent/` — Edit Profile (Parent & Child cards), Settings (OTP toggle, Language switcher, Curriculum checkboxes), Subscription summary placeholder, Help Center FAQ page, Parent Portal overview with child mastery alerts & quick custom assignment button.
  - **Acceptance Criteria**: Parent can switch child's language/curricula and inspect weak topics in <3 clicks.

- [x] **P2-CNT-01** `[CONTENT]` Author Space Academy Lesson Packages (G1–G6 Core Topics) *(Owner: agent-content-1 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P1-CNT-01`, `P1-CNT-02`
  - **Spec**: `docs/04-curriculum-and-pedagogy.md` §1, `docs/09-business-roadmap-and-operations.md` §1
  - **Scope**: `content/academy/` — Structured lesson manifests (metadata, bilingual Self-Check checklists, 4–6 Do & Learn linked questions, bilingual video/subtitle references) for core G1–G6 topics.
  - **Acceptance Criteria**: Seed script populates at least 12 complete Space Academy topics across G1–G6 with working drills and bilingual self-check items.

- [x] **P2-CNT-02** `[CONTENT]` Author AstroQuiz Exam & Revision Papers (G1–G6) *(Owner: agent-content-1 | Done: 2026-11-26T23:34Z)*
  - **Depends On**: `P1-CNT-02`
  - **Spec**: `docs/02-features-student-portal.md` §9, `docs/04-curriculum-and-pedagogy.md`
  - **Scope**: `content/quizzes/` — Curated 15–20 question papers across all 3 categories (Essential Skills, Top School Year-End, HOTS Papers) for Grades 1–6 in `en` & `id`.
  - **Acceptance Criteria**: Every Grade 1–6 has at least 1 paper per category; automated schema check passes 100%.

---

## Phase 3 — Gamification, Planet Stories, Social & Cosmic Events

> **Parallelism Note**: Super Nova (`P3-BE-01`/`P3-FE-01`), Planet Stories (`P3-BE-02`/`P3-FE-02`), Customization & Badges (`P3-BE-03`/`P3-FE-03`), Star Duels & Crews (`P3-BE-04`/`P3-FE-04`), and Leaderboards & Events (`P3-BE-05`/`P3-FE-05`) are cleanly decoupled and can run in parallel!

- [x] **P3-BE-01** `[BE]` Implement Super Nova Challenge API (Super Sight & Super Warp) *(Owner: agent-be-1 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P0-08`, `P1-ENG-01`
  - **Spec**: `docs/02-features-student-portal.md` §5
  - **Scope**: `apps/api/internal/modules/supernova/` — Mon–Sat 06:00–22:00 schedule, weekly diamond `S` score aggregation, **Super Sight** (spot-the-difference: 5 diffs, max 5 wrong attempts, 10 pts/hit, 5-min limit), **Super Warp** (2-min rapid-fire sprint, 1 pt/correct, 10s auto-skip, Practise vs Challenge modes), Score History API.
  - **Acceptance Criteria**: Server validates timestamps & max attempts; Practice mode does not alter weekly S score; Score History paginates cleanly.

- [x] **P3-FE-01** `[FE]` Build Super Nova Challenge UI (Super Sight & Super Warp) *(Owner: agent-fe-1 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P0-04`, `P3-BE-01`
  - **Spec**: `docs/02-features-student-portal.md` §5
  - **Scope**: `apps/web/src/routes/$locale/super-nova/` — Hub screen showing weekly diamond `S` score, interactive spot-the-difference canvas (click/tap coordinates with animated hit rings & wrong-tap counter), Super Warp rapid-fire player with 2-min bar + 10s per-question timer + "pause after current question" button, Score History modal.
  - **Acceptance Criteria**: Coordinate hit detection scales accurately across mobile, tablet, and desktop aspect ratios.

- [x] **P3-BE-02** `[BE]` Implement Planet Stories API (6 Grade Worlds & Chapter Unlocks) *(Owner: agent-be-1 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P0-08`
  - **Spec**: `docs/02-features-student-portal.md` §8
  - **Scope**: `apps/api/internal/modules/stories/` — 6 Grade Planets metadata, chapter progression (Opening Chapter unlocked free; subsequent chapters unlock atomically for 20 AC), reading progress bookmarking, bilingual page manifest API.
  - **Acceptance Criteria**: Unlocking a chapter deducts 20 AC atomically and is idempotent; insufficient AC returns structured error.

- [x] **P3-FE-02** `[FE]` Build Planet Stories World Map & 20-Page Storybook Viewer *(Owner: agent-fe-1 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P0-04`, `P3-BE-02`
  - **Spec**: `docs/02-features-student-portal.md` §8
  - **Scope**: `apps/web/src/routes/$locale/stories/` — Planet selector dropdown (6 Grade Worlds), horizontal galaxy chapter timeline with lock/unlock badges, full-screen illustrated digital storybook viewer (~20 pages) with swipe/arrow navigation, thumbnail strip, bilingual text overlay, and TTS read-aloud toggle.
  - **Acceptance Criteria**: Smooth page flip on touch & keyboard; zoom/readability crisp on tablets and phones.

- [x] **P3-BE-03** `[BE]` Implement Avatars, My Astro Buddy, Showcase & Daily Star Badges API *(Owner: agent-be-2 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P0-08`, `P1-BE-01`
  - **Spec**: `docs/02-features-student-portal.md` §12, §15
  - **Scope**: `apps/api/internal/modules/customization/` — 8 avatar theme packs, **My Astro Buddy** builder persistence (body, 15 colors, horns/ears, eyes, mouth, 50+ cosmic adjective + student first name generator), **Galaxy Showcase** ranking by SP across G1–G6 (requires `>=1 SP`), **Daily Star Badges** monthly calendar (1–4 stars Bronze/Silver/Gold/Holographic + 20 AC missed-day recovery).
  - **Acceptance Criteria**: Missed badge recovery deducts 20 AC idempotently; Showcase hides students with 0 SP and masks surnames.

- [x] **P3-FE-03** `[FE]` Build Avatar Picker, Astro Buddy Builder, Showcase & Badge Calendar UI *(Owner: agent-fe-2 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P0-04`, `P3-BE-03`
  - **Spec**: `docs/02-features-student-portal.md` §12, §15
  - **Scope**: `apps/web/src/routes/$locale/astro-buddy/`, `apps/web/src/routes/$locale/showcase/`, `apps/web/src/routes/$locale/achievement/` — Interactive layered SVG Astro Buddy customizer with live preview & 2-part name picker, Galaxy Showcase podium/grid, Monthly Daily Star Badges calendar with flip-able trivia cards (Bronze/Silver/Gold/Holographic shader effects) and 20 AC unlock modal, 8-pack Avatar selector.
  - **Acceptance Criteria**: Custom SVG Buddy renders consistently across dashboard, showcase, and duels; badge animations smooth at 60fps.

- [x] **P3-BE-04** `[BE]` Implement Star Duels (1v1 Peer Challenge) & Star Crew (BFF) API *(Owner: agent-be-2 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P0-08`, `P1-ENG-01`
  - **Spec**: `docs/02-features-student-portal.md` §11, `docs/05-gamification-social-and-safety.md` §3
  - **Scope**: `apps/api/internal/modules/social/` — 3 energy cells/day (resets midnight user TZ), matchmaking (My Classmate, My Star Crew, Random Pilot same grade), async/live turn-based duel scoring & SP award, Incoming/History/Rules endpoints, **Star Crew** 9-digit invite code generation, join/leave, and weekly Crew SP aggregation.
  - **Acceptance Criteria**: Energy limit strictly enforced server-side; random matchmaking respects grade level and privacy masking; crew codes unique & collision-safe.

- [x] **P3-FE-04** `[FE]` Build Star Duels & Star Crew UI *(Owner: agent-fe-2 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P1-ENG-03`, `P3-BE-04`
  - **Spec**: `docs/02-features-student-portal.md` §11
  - **Scope**: `apps/web/src/routes/$locale/star-duels/`, `apps/web/src/routes/$locale/star-crew/` — Duel lobby with 3 glowing energy cells, opponent selector (Classmate / Star Crew / Random Match), Incoming challenges, History & Rules tabs, Star Crew 9-digit code copy/join UI and weekly Crew CP board.
  - **Acceptance Criteria**: Full challenge → accept → score comparison flow verified in E2E tests.

- [x] **P3-BE-05** `[BE]` Implement Galaxy Leaderboards (Redis) & Cosmic Events API *(Owner: agent-be-1 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P0-08`, `P3-BE-01`, `P3-BE-04`
  - **Spec**: `docs/02-features-student-portal.md` §11, §13, `docs/08-technical-architecture-and-database.md` §4
  - **Scope**: `apps/api/internal/modules/leaderboards/` — Redis Sorted Sets refreshed every 5 minutes, 6 Open Challenges (*Hero of the Day*, *Top Brain*, *Champion Class*, *Star Crew*, *Super Nova*, *Orbit Streak*) + 4 Seasonal Event configs (*Zodiac Warp*, *Comet vs Meteor*, *Star Academy Cup*, *Galactic City Race*), scopes (`CLASS`, `SCHOOL`, `NATIONAL`), Top 50, Nearby Pilots (`±5` ranks), Hall of Fame archives, masked peer names, Sunday Meteor Challenge (Sun 06:00–22:00, 10 Qs), Quarterly Creator Challenge, Notifications & Daily Bonus Gift endpoints.
  - **Acceptance Criteria**: Leaderboard queries respond `<50ms` under 10k simulated students; zero unmasked PII in payloads.

- [x] **P3-FE-05** `[FE]` Build Galaxy Leaderboards, Events, Bonus Gift & Notifications UI *(Owner: agent-fe-1 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P0-06`, `P3-BE-05`
  - **Spec**: `docs/02-features-student-portal.md` §11, §13
  - **Scope**: `apps/web/src/routes/$locale/leaderboard/`, `apps/web/src/routes/$locale/events/` — Challenge hub (6 Open + 4 Seasonal cards), drill-down leaderboard table with Class/School/National tabs, Grade filter, Top 50 vs Nearby toggle, Hall of Fame drawer, Events page (Sunday Meteor & Creator Challenge), Notification dropdown & Bonus Gift modal in top nav.
  - **Acceptance Criteria**: Masked names & Astro Buddies display cleanly; live 5-min refresh countdown visible.

- [x] **P3-CNT-01** `[CONTENT]` Author Planet Stories (Opening Chapters for G1–G6 + Chapter 1–2 Samples) *(Owner: agent-content-1 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P3-BE-02`
  - **Spec**: `docs/02-features-student-portal.md` §8
  - **Scope**: `content/stories/` — Bilingual (`en` & `id`) 20-page Opening Chapters for all 6 Grade Planets plus unlockable Prologue/Chapter 1 manifests and illustration asset specs.
  - **Acceptance Criteria**: All 6 planets open cleanly in the viewer with complete bilingual narration and page thumbnails.

- [x] **P3-CNT-02** `[CONTENT]` Author Daily Star Badges Trivia Pack (365-Day Bilingual Seed) *(Owner: agent-content-1 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P3-BE-03`
  - **Spec**: `docs/02-features-student-portal.md` §12
  - **Scope**: `content/badges/` — Bilingual space/math fun-fact trivia cards mapped to calendar days with category tags and visual metadata.
  - **Acceptance Criteria**: Every calendar day of the year resolves a valid `en` and `id` trivia card.

- [x] **P3-CNT-03** `[CONTENT]` Author Super Sight Visual Puzzle Pack (Initial 30 Puzzles) *(Owner: agent-content-1 | Done: 2026-11-27T01:33Z)*
  - **Depends On**: `P3-BE-01`
  - **Spec**: `docs/02-features-student-portal.md` §5
  - **Scope**: `content/super-sight/` — Structured SVG/illustrated puzzle pairs with exact normalized bounding boxes for 5 differences per puzzle.
  - **Acceptance Criteria**: Automated coordinate validator confirms 5 non-overlapping target regions per puzzle.

---

## Phase 4 — Brain Galaxy, School/Teacher Portal & Admin CMS v2

> **Parallelism Note**: Brain Galaxy (`P4-GAME-*`), Teacher/School Portal (`P4-BE-02`/`P4-FE-02`), and Admin CMS (`P4-BE-03`/`P4-FE-03`) can execute completely in parallel!

- [ ] **P4-BE-01** `[BE]` Implement Brain Galaxy Gatekeeper & 25-Minute Healthy Timer API
  - **Depends On**: `P0-08`
  - **Spec**: `docs/02-features-student-portal.md` §12, `docs/05-gamification-social-and-safety.md` §5
  - **Scope**: `apps/api/internal/modules/brain-galaxy/` — Operating hours check (`08:00–18:00` school accounts / `08:00–20:00` individual accounts, customizable tighter by parent), server-tracked 25-minute/day heartbeat & hard lock, 1 AC per game per day entry deduction, per-zone high score persistence.
  - **Acceptance Criteria**: Tampering with browser clock cannot extend the 25-min limit; re-entering the same game on the same day does not double-charge 1 AC.

- [ ] **P4-GAME-01** `[GAMES]` Build Brain Galaxy Hub & 5-Zone Cognitive Mini-Games Engine
  - **Depends On**: `P4-BE-01`, `P0-04`
  - **Spec**: `docs/02-features-student-portal.md` §12
  - **Scope**: `packages/brain-games/`, `apps/web/src/routes/$locale/brain-galaxy/` — Interactive Galaxy Island map with live 25-min countdown HUD and 5 initial playable HTML5 Canvas/SVG mini-games (1 per zone: **Memory**, **Flexibility**, **Speed**, **Problem Solving**, **Adventure**), touch & keyboard accessible, bilingual instructions.
  - **Acceptance Criteria**: All 5 mini-games run at 60fps on mid-range tablets, pause cleanly when 25-min timer expires, and submit scores securely.

- [ ] **P4-BE-02** `[BE]` Implement Teacher & School Portal API (Rosters, Bulk Tasks, Auto-Promotion)
  - **Depends On**: `P0-05`, `P1-BE-04`, `P2-BE-03`
  - **Spec**: `docs/06-portals-parent-teacher-admin.md` §2
  - **Scope**: `apps/api/internal/modules/school/` — School & Class CRUD, CSV bulk student import & credential generation, class-wide bulk assignment creation, class proficiency heatmap & weak-topic drilldown, annual automatic grade promotion scheduler (e.g., July 1 academic rollover, P1→P2...P6→Alumni).
  - **Acceptance Criteria**: Teacher RBAC strictly scopes access to assigned classes; CSV import handles 500 students cleanly with validation report; rollover job is idempotent.

- [ ] **P4-FE-02** `[FE]` Build Teacher & School Portal UI
  - **Depends On**: `P0-06`, `P4-BE-02`
  - **Spec**: `docs/06-portals-parent-teacher-admin.md` §2
  - **Scope**: `apps/web/src/routes/$locale/teacher/` — Class switcher, Student Roster & CSV Import wizard, Class Proficiency Heatmap, Bulk Assignment creator, Class Leaderboard & printable student login slips.
  - **Acceptance Criteria**: Teachers can import a class, assign a 10-question revision set, and review class mastery in <5 minutes.

- [ ] **P4-BE-03** `[BE]` Implement Admin & Content Management System (CMS) API
  - **Depends On**: `P0-05`, `P1-ENG-01`, `P2-BE-01`, `P3-BE-02`, `P3-BE-05`
  - **Spec**: `docs/06-portals-parent-teacher-admin.md` §3
  - **Scope**: `apps/api/internal/modules/admin-cms/` — CRUD & Draft/Review/Published workflow for Bilingual Questions & Procedural Templates, Curricula/Skills, Space Academy Lessons, AstroQuiz Papers, Planet Stories, Seasonal Events, User Feedback/Bug Queue, and Audit Logs.
  - **Acceptance Criteria**: Publishing a question validates both `en` & `id` payloads via `internal/mathengine` before going live; all admin mutations write audit logs.

- [ ] **P4-FE-03** `[FE]` Build Admin & Bilingual Content CMS UI
  - **Depends On**: `P0-04`, `P1-ENG-03`, `P4-BE-03`
  - **Spec**: `docs/06-portals-parent-teacher-admin.md` §3
  - **Scope**: `apps/web/src/routes/$locale/admin/` — Side-by-side `en` / `id` Question Authoring editor with live Question Player preview, Generator parameter tester, Story/Lesson uploader, Event scheduler, and Student Feedback triage queue.
  - **Acceptance Criteria**: Content editors can author, preview in both languages, and publish a new question without developer intervention.

- [ ] **P4-QA-01** `[QA]` Expand Automated E2E Test Suite (All 4 Portals, Bilingual)
  - **Depends On**: `P4-GAME-01`, `P4-FE-02`, `P4-FE-03`
  - **Spec**: `docs/08-technical-architecture-and-database.md` §7
  - **Scope**: `tests/e2e/` — Playwright specs covering Student (Daily Orbit, Training, Academy, Stories, AstroQuiz, Duels, Brain Galaxy), Parent (Custom Assignment, OTP Settings), Teacher (CSV Import, Bulk Task), and Admin CMS in both `en` and `id`.
  - **Acceptance Criteria**: CI E2E suite passes deterministically across Chromium & WebKit mobile/tablet/desktop viewports.

- [ ] **P4-BE-04** `[BE]` Implement Server-Side Natural TTS Audio Pipeline & Caching (`en` + `id`)
  - **Depends On**: `P0-03`, `P1-CNT-02`
  - **Spec**: `docs/07-i18n-multilanguage-architecture.md` §4
  - **Scope**: `apps/api/internal/modules/tts/` — Child-friendly neural TTS synthesis with SHA-256 content hash caching in object storage (S3/R2) + CDN headers, fallback to Web Speech API, math-expression verbalizer (`3/4` → "three quarters" / "tiga per empat").
  - **Acceptance Criteria**: Repeated TTS requests hit CDN cache (`>95%` hit rate); math operators and fractions read naturally in both English and Indonesian.

- [ ] **P4-CNT-01** `[CONTENT]` Expand Full G1–G6 Bilingual Content Library (Release Candidate)
  - **Depends On**: `P4-FE-03`, `P1-CNT-02`, `P2-CNT-01`, `P2-CNT-02`
  - **Spec**: `docs/09-business-roadmap-and-operations.md` §1
  - **Scope**: `content/` — Complete target launch corpus: 150+ procedural templates + 1,500+ curated word/HOTS problems, 30+ Space Academy topics, 36+ AstroQuiz papers, 6 Planet Story Opening + Chapter 1–2 arcs in `en` & `id`.
  - **Acceptance Criteria**: Content completeness report shows zero empty skills in Grades 1–6 primary tracks; bilingual QA sign-off complete.

---

## Phase 5 — Security Audit, Performance, Launch & Deferred Monetization

> **Feature-First Policy**: All product features (Phases 0–4) run **100% unlocked** with zero paywalls. Monetization tasks (`P5-BE-01`, `P5-FE-01`) are strictly deferred until all Phases 0–4 features are complete and verified, and can be toggled on via feature flag (`BILLING_ENABLED=false` by default until ready).

- [ ] **P5-BE-01** `[BE]` *(Deferred — Post-Feature)* Implement Subscription, Entitlements & Payment Gateways (ID + Global)
  - **Depends On**: `P0-05`, `P2-BE-04`, `P4-QA-01`
  - **Spec**: `docs/09-business-roadmap-and-operations.md` §2
  - **Scope**: `apps/api/internal/modules/billing/` — Plan tiers (Free Explorer, Galaxy Premium Monthly/Annual, School B2B License), Midtrans/Xendit (QRIS, Virtual Account, e-wallet for Indonesia) + Stripe (Global cards), idempotent webhook handlers, grace periods, entitlement guards (e.g., 8am–8pm Brain Galaxy, full Story/HOTS/Quiz access).
  - **Acceptance Criteria**: Webhook replay tests idempotent; upgrading/downgrading updates entitlements immediately; My Subscription page displays live dates & receipts.

- [ ] **P5-FE-01** `[FE]` Build Pricing, Checkout & My Subscription Management UI
  - **Depends On**: `P2-FE-04`, `P5-BE-01`
  - **Spec**: `docs/02-features-student-portal.md` §15, `docs/09-business-roadmap-and-operations.md` §2
  - **Scope**: `apps/web/src/routes/$locale/pricing/`, `apps/web/src/routes/$locale/profile-settings/subscription/` — Localized pricing (IDR & USD), Parent-gated checkout flow, invoice history, cancel/renew controls, upgrade prompts that never pressure children directly without parent gate.
  - **Acceptance Criteria**: Child accounts see a "Ask a Parent" gate on purchase buttons; checkout tested in sandbox for IDR & USD.

- [ ] **P5-INFRA-01** `[INFRA]` Set Up Production Observability, Backups & Disaster Recovery
  - **Depends On**: `P0-01`
  - **Spec**: `docs/08-technical-architecture-and-database.md` §8
  - **Scope**: Structured JSON logging, OpenTelemetry traces, Sentry error tracking (PII-scrubbed), Prometheus/Grafana dashboards, automated encrypted daily PostgreSQL backups with tested restore script, uptime alerts.
  - **Acceptance Criteria**: Simulated DB restore succeeds within RPO/RTO targets; zero child PII appears in logs or Sentry events.

- [ ] **P5-QA-01** `[QA]` Execute Security, Anti-Cheat & Child Privacy Audit (UU PDP & COPPA)
  - **Depends On**: `P5-BE-01`, `P4-QA-01`
  - **Spec**: `docs/05-gamification-social-and-safety.md` §6
  - **Scope**: `tests/security/` — RBAC penetration checks, IDOR verification across student/parent/teacher endpoints, OTP brute-force protection, economy race-condition tests, CSP/XSS/CSRF headers, data export & right-to-delete (account erasure) verification.
  - **Acceptance Criteria**: Zero Critical/High vulnerabilities; verified account deletion purges child PII cleanly.

- [ ] **P5-QA-02** `[QA]` Run Load, Performance & PWA Lighthouse Audits
  - **Depends On**: `P4-QA-01`, `P5-INFRA-01`
  - **Spec**: `docs/08-technical-architecture-and-database.md` §7
  - **Scope**: `tests/load/` — k6 load scripts simulating 5,000 concurrent students during peak Daily Orbit (19:00–21:00 local time) + Leaderboard refresh storms; Lighthouse audit on tablet & mobile networks.
  - **Acceptance Criteria**: P95 API latency `<250ms` under peak load; Lighthouse Performance `>=90`, Accessibility `>=95`, PWA Pass.

- [ ] **P5-FE-02** `[FE]` Build Prerendered Public Marketing Landing Page (Bilingual `en` & `id`)
  - **Depends On**: `P0-03`, `P0-04`, `P4-QA-01`
  - **Spec**: `docs/01-product-vision-and-theme.md`, `docs/08-technical-architecture-and-database.md` §5
  - **Scope**: `apps/web/src/routes/$locale/(marketing)/` — Static build-time prerendered (`/en/index.html`, `/id/index.html`) Galaxy-themed landing page for Parents & Schools on Cloudflare Pages, interactive sample math question demo (no login required), curriculum showcase, FAQ, Privacy Policy & Terms of Service (`en` & `id`), plus optional pricing section controlled by `BILLING_ENABLED`.
  - **Acceptance Criteria**: Prerendered HTML includes SEO metadata, OpenGraph tags, hreflang (`en`/`id`), and interactive demo works smoothly on mobile & desktop.

- [ ] **P5-OPS-01** `[INFRA]` Execute Staged Beta Rollout & Production Launch Checklist
  - **Depends On**: `P5-QA-01`, `P5-QA-02`, `P5-FE-02`, `P4-CNT-01`
  - **Spec**: `docs/09-business-roadmap-and-operations.md` §4–5
  - **Scope**: Staging soak test, Closed Beta with pilot families/schools (G1–G6), telemetry dashboard verification (D1/D7 retention, Daily Orbit completion rate, crash-free sessions), production cutover runbook.
  - **Acceptance Criteria**: All launch gates green; rollback runbook tested; production live with monitoring active.

---

## Section 7 — Handoff & Activity Log

- [x] **PR6-BUDDY-ACCESSIBLE-NAME** — Localize the Astro Buddy fallback accessible name. *(Owner: agent-pr6-fixer | Done: 2026-11-28T16:27Z)*
  - **Depends On**: `P3-BE-03`, `P0-03`
  - **Spec**: `docs/02-features-student-portal.md` §12.2; `docs/07-i18n-multilanguage-architecture.md`
  - **Acceptance Criteria**: Missing or empty names use a localized fallback in en/id; custom names remain unchanged, with tests for each locale.
  - **Handoff**: AstroBuddyAvatar uses the `stories.customization.buddyFallbackName` key in en/id for missing, empty or whitespace-only names. Focused and full web tests, web typecheck/lint/build, and i18n parity passed; no new environment variables or APIs.

- [x] **PR6-CI-TEST-CONTENTION** — Reduce package contention in the PostgreSQL race suite without relaxing leaderboard timing assertions. *(Owner: agent-pr6-fixer | Done: 2026-11-28T16:20Z)*
  - **Depends On**: `PR6-CI-PG-ISOLATION`
  - **Spec**: `docs/08-technical-architecture-and-database.md` §5
  - **Acceptance Criteria**: Go race tests, including PostgreSQL tests and the existing 50 ms assertion, run without simultaneous package-level test binaries.
  - **Handoff**: CI uses `go test -race -p 1 ./...`; the full uncached suite passed locally against seeded disposable PostgreSQL. The 50 ms timing test remains load-sensitive; serializing packages reduces contention but cannot guarantee a timing threshold on shared runners.

- [x] **PR6-CI-PG-ISOLATION** — Run PostgreSQL tests in CI and isolate the showcase singleton test. *(Owner: agent-pr6-fixer | Done: 2026-11-28T16:15Z)*
  - **Depends On**: `P0-01`, `P3-BE-03`
  - **Spec**: `docs/08-technical-architecture-and-database.md` §5; `docs/02-features-student-portal.md` §12.2
  - **Acceptance Criteria**: CI initializes a disposable PostgreSQL database with migrations and seed and runs the Go race suite with it; the showcase test never mutates the shared singleton row.
  - **Handoff**: CI Go job uses a disposable PostgreSQL 16 service and existing seed command; showcase integration test uses its own schema. Local disposable PostgreSQL 17: seeded full `go test -race -count=1 ./...`, `go vet ./...`, and Go binary builds passed; two concurrent showcase tests preserved a public sentinel row. No production migration or environment variable.

- [x] **PR6-SHOWCASE-SEED-ASSERTION** — Verify the initial showcase seed is durably written before replacing the test fixture. *(Owner: agent-pr6-fixer | Done: 2026-11-28T15:42Z)*
  - **Depends On**: `P3-BE-03`
  - **Spec**: `docs/02-features-student-portal.md` §12.2
  - **Acceptance Criteria**: The PostgreSQL showcase test fails if seed-on-missing does not write its shared row; run only against a disposable isolated database.
  - **Handoff**: Assert the stored row exists and matches the seed before writing the distinct fixture. The targeted PostgreSQL test passed on a newly created disposable database, and customization `go test` and `go vet` passed. Singleton-key concurrency remains tracked in issue #15.

- [!] **PR6-REVIEW-ROUND4-EASY** — Address the owner-approved easy PR #6 findings, excluding the pending ID-format decision. *(Owner: agent-pr6-fixer | Blocked: #10 requires coordinated crew lookup error handling and creation semantics; defer with #11 rather than swallow a read error.)*
  - **Depends On**: `P3-BE-03`, `P3-BE-04`, `P3-BE-05`
  - **Spec**: `docs/02-features-student-portal.md` (showcase, Star Duels, Star Crew and events)
  - **Acceptance Criteria**: Correct showcase persistence and tests, preserve active duels, refresh crew reads without swallowing errors, fix gift lookup errors and zero-reward Meteor completion, and verify relevant tests.
  - **Handoff**: Implemented showcase, duel retention, test fixture cleanup/clock, bonus status error propagation and zero-reward Meteor completion. PostgreSQL integration tests, Go tests, vet and build pass. #10 remains open; active duels have no expiry policy and can grow without bound. No migration or new environment variable.

- [x] **PR3-REVIEW-ROUND3** — Remove the streak mutex/pool lock-order cycle found on `4876e87`. *(Owner: agent-pr3-integrity | Done: 2026-11-28T03:31Z)*
  - **Depends On**: `PR3-REVIEW-ROUND2`
  - **Spec**: `docs/02-features-student-portal.md` §4
  - **Acceptance Criteria**: No streak operation holds the memory mutex across a pool call, and no transaction waits for that mutex while holding a connection.

- [x] **PR3-REVIEW-ROUND2** — Address the seven CodeRabbit findings on the transaction refactor. *(Owner: agent-pr3-integrity | Done: 2026-11-28T03:02Z)*
  - **Depends On**: `PR3-SESSION-RECOVERY`
  - **Spec**: `docs/02-features-student-portal.md` §3–4; `docs/04-curriculum-and-pedagogy.md` §4.1–4.2
  - **Acceptance Criteria**: Streak writes share the answer transaction, no PG state is reused after rollback, request contexts reach lock waits, and the review's test/High Score gaps are covered.

- [x] **PR3-SESSION-RECOVERY** — Serialize persisted learning-session writes and recover accepted Daily Orbit effects after rollover. *(Owner: agent-pr3-integrity | Done: 2026-11-28T02:35Z)*
  - **Depends On**: `PR3-STREAK-RETRY`
  - **Spec**: `docs/02-features-student-portal.md` §3–4; `docs/04-curriculum-and-pedagogy.md` §4.1
  - **Acceptance Criteria**: Parallel requests preserve answers; Training stats and answers commit together; accepted Daily Orbit effects survive the operating window and date rollover.

- [x] **PR3-STREAK-RETRY** — Preserve accepted Daily Orbit answers when streak recording fails and correct Question Player E2E mock routing. *(Owner: agent-pr3-integrity | Done: 2026-11-28T01:16Z)*
  - **Depends On**: `PR3-LEARNING-INTEGRITY`
  - **Spec**: `docs/02-features-student-portal.md` §3–4
  - **Acceptance Criteria**: A failed streak write leaves the slot locked and retryable for streak recording without changing the answer or duplicating rewards; E2E mocks match their respective endpoints with query parameters.

- [x] **PR3-LEARNING-INTEGRITY** — Fix procedural grading/generation, canonical skill attribution, and retry-safe Daily Orbit proficiency recording. *(Owner: agent-pr3-integrity | Done: 2026-11-28T01:10Z)*
  - **Depends On**: `P1-ENG-01`, `PR3-INTEGRITY`
  - **Spec**: `docs/02-features-student-portal.md` §3, `docs/03-question-engine-and-math-tools.md`, `docs/04-curriculum-and-pedagogy.md` §4.1
  - **Acceptance Criteria**: Correct fraction and MCQ grading, valid elapsed-time questions, aligned skill registries, and one reward/attempt after recovering from a recorder failure.

> Agents MUST append an entry here whenever claiming a blocker (`[!]`) or completing a task (`[x]`). Newest entries at the top.

| UTC Timestamp | Agent ID | Task ID | Status | Summary / Handoff Notes |
|---|---|---|---|---|
| 2026-11-28T11:28Z | `agent-pr5-review` | `PR5-REVIEW-ROUND17` | `[x]` | Closed the same-ID concurrent submission finding: after the final persist, `SubmitQuestion` refreshes the durable assignment and compares the committed answer with the graded snapshot even when the ID did not change. Added a blocked same-ID concurrent-submission regression; a changed committed answer now returns 409. Full Go race suite, vet/build, forced typecheck/lint/test/build, i18n parity pass. |
| 2026-11-28T11:24Z | `agent-pr5-review` | `PR5-REVIEW-ROUND16` | `[x]` | Closed the final three findings: submission-side reward eligibility requires `persisted`, the 409 handler only closes the assignment that produced the conflict while always invalidating its list, and answered-question retries compare the committed answer against the graded snapshot. Barrier tests now use cancellable contexts. Full Go race suite, vet/build, forced typecheck/lint/test/build, i18n parity pass. |
| 2026-11-28T11:11Z | `agent-pr5-review` | `PR5-REVIEW-ROUND15` | `[x]` | Closed the final five findings: load rewards require `persisted`, alias retirement carries revisions changed during the save into a second durable write, alias submissions return 409 when the committed answer/items differ, barrier tests release in cleanup with bounded waits, and the assignment UI handles 409 by closing the detail, invalidating the list, and showing the changed-assignment message. Full Go race suite, vet/build, forced typecheck/lint/test/build, i18n parity pass. |
| 2026-11-28T10:28Z | `agent-pr5-review` | `PR5-REVIEW-ROUND14` | `[x]` | Closed the discarded-submission case: when a concurrent request-ID alias has different questions, `SubmitQuestion` now returns an explicit `ErrSubmissionConflict` (HTTP 409) instead of success with a grade that was not stored, and no reward is credited. Added a blocked-save regression with different alias questions. Full Go race suite, vet/build, forced typecheck/lint/test/build, and i18n parity pass. |
| 2026-11-28T10:22Z | `agent-pr5-review` | `PR5-REVIEW-ROUND13` | `[x]` | Closed the final load/reward alias race: pending load rewards now resolve their request-ID aliases to the durable ID after persistence and only credit records that remain completed, while `SubmitQuestion` rechecks completion after alias resolution. Added tests for discarded different-question progress and a blocked load whose durable alias is created concurrently; both assert the durable ID is credited and no progress is lost. Full Go race suite, vet/build, forced typecheck/lint/test/build, and i18n parity pass. |
| 2026-11-28T10:16Z | `agent-pr5-review` | `PR5-REVIEW-ROUND12` | `[x]` | Closed the concurrent alias-reward window: after `SubmitQuestion` persists, it resolves the local request-ID record to the durable ID before crediting and re-renders the detail, so a durable record created during the save cannot be rewarded twice. Added a barrier regression where the durable create lands while the local submission is blocked in `SaveMerged`; the ledger receives the durable ID. Full Go race suite, vet/build, forced typecheck/lint/test/build, and i18n parity pass. |
| 2026-11-28T10:09Z | `agent-pr5-review` | `PR5-REVIEW-ROUND11` | `[x]` | Fixed the final alias-persistence finding: a merged request-ID alias keeps `persisted=false` when it carries local submitted progress, so `ensureStudentLoaded` saves that progress before reward reconciliation. Added a store-checking ledger to the failed-create test, asserting the durable record is complete at the moment AC is credited. Full Go race suite, vet/build, forced typecheck/lint/test/build, and i18n parity pass. |
| 2026-11-28T09:58Z | `agent-pr5-review` | `PR5-REVIEW-ROUND10` | `[x]` | Closed the remaining alias and overlap findings: `ensureStudentLoaded` reconciles a retained local record into the durable request-ID record before listing/grading, so a failed create cannot be submitted or rewarded twice; request-ID creates derive deterministic seeds so a retry reproduces the same questions and can carry progress. Question-level Save is disabled while a save is pending, and a later paper now exposes `supplyPodPendingPaper` with a bilingual banner while the reserving paper keeps its retry. Added a failed-create alias progress/reward test, a pending-award assertion, and full checks pass. |
| 2026-11-28T09:35Z | `agent-pr5-review` | `PR5-REVIEW-ROUND9` | `[x]` | Closed the cross-instance Supply Pod identity finding: `phase2_state` now stores `weeklyPaper` (week -> earning paper), reserved atomically before the ledger, so a different instance submitting another paper cannot take the badge or double-credit; the earning paper can recover its badge on retry, including after a failed final write. Added an idempotent-ledger cross-instance regression and kept paper-specific `supplyPodRetryable`. Also moved assignment `needsPersist` calculation under `s.mu`, paused checkpoints during pending saves, and prevented alias merges with different question specs from copying answers/correctness. Full Go race suite, vet/build, forced typecheck/lint/test/build, i18n parity, OpenAPI/client sync pass. |
| 2026-11-28T09:31Z | `agent-pr5-review` | `PR5-REVIEW-ROUND8` | `[x]` | Closed the three findings from the review on `5498c89`: assignment reconciliation computes `needsPersist` under `s.mu` before releasing it; the AstroQuiz checkpoint interval now also pauses while a save is pending; and request-ID alias merges with different item specs no longer copy submitted answers/correctness onto the durable assignment. Added alias-progress assertions. Full Go race suite, vet/build, forced typecheck/lint/test/build, i18n parity, and OpenAPI/client sync pass. |
| 2026-11-28T09:18Z | `agent-pr5-review` | `PR5-REVIEW-ROUND7` | `[x]` | Closed the review on `d64d171`: Academy `RetryScrollAward` persists solved progress before the ledger credit; assignment alias merges preserve the durable ID/title/seeds and load-time reconciliation persists pending completion before AC; AstroQuiz keeps a paper's Supply Pod badge across a failed final write and exposes paper-specific `supplyPodRetryable`; the web client tracks per-question draft revisions, sends only dirty answers, clears a draft only when its revision still matches, and ignores late `IN_PROGRESS` saves after submission; checkpoints pause while submit is pending. Added regression tests for each backend case plus HTTP 503 mapping and direct alias identity. Verified with full `go test -race`/vet/build, forced typecheck/lint/test/build, i18n parity, OpenAPI/client regen, and a live A→B→A save test: the button stayed "Simpan" and submit still sent the restored answer. |
| 2026-11-28T08:49Z | `agent-pr5-security` | `PR5-REVIEW-FIX-4` | `[x]` | Closed the retained CodeRabbit concerns on `063f2e5` that were still reproducible on `46d5a0d`: Academy and Assignments now make the qualifying progress durable before crediting the wallet (a crash after the credit can no longer lose the answer that earned it); assignment persistence reconciles request-ID merges so a concurrent create answers with the durable assignment ID; re-submitting an answered assignment question returns the stored result (200) instead of 400; AstroQuiz Supply Pod ledger failures return 503 (retryable). Lint: four duplicated `isUUID` copies replaced by `reqguard.IsUUID`, plus ineffassign/SA4006/unused fixes — `golangci-lint` v2.15 (built with go1.27) is clean for all PR #5 files; 5 remaining findings are pre-existing Phase 1 code (`training/service.go:384` errcheck, `RealIP` deprecation in `server/server.go`, De Morgan in streak/training/dailyorbit pgstores). Verified: `go vet`, `go test -race ./...` incl. PostgreSQL suite, OpenAPI unchanged. |
| 2026-11-28T08:20Z | `agent-pr5-review` | `PR5-REVIEW-ROUND6` | `[x]` | Closed the remaining cross-instance submission finding: `SubmitPaper` now grades and freezes the target attempt inside `state.MergeStore.SaveMerged`'s row lock, using the durable drafts rather than a stale instance cache, then awards the Supply Pod from the canonical claim week. Added `TestSubmitGradesDurableDraftsAcrossInstances` (stale instance submits empty while another instance saved a correct answer; result is 1/10 and durable). Also fixed the untouched Save notice: `QuestionPlayer` now honors a `false` result from `onSaveDraft`, and AstroQuiz returns `false` without a request when the active answer is not locally dirty; live check showed no request and the button stayed "Simpan". Full Go race suite, vet/build, forced typecheck/lint/test/build, and i18n parity pass. |
| 2026-11-28T08:11Z | `agent-pr5-review` | `PR5-REVIEW-ROUND5B` | `[x]` | Follow-up on the dirty-answer review: the per-question Save and per-question Submit actions now mark an answer dirty only when `draftEdits` already has that question; an untouched cached answer is neither saved nor submitted, so another tab’s newer value cannot be overwritten. Live two-action check: untouched Save produced 0 `/save` requests, and untouched per-question Submit followed by paper submission sent `{"drafts":{}}`. `typecheck` and `lint` pass. |
| 2026-11-28T08:04Z | `agent-pr5-review` | `PR5-REVIEW-ROUND5` | `[x]` | Closed CodeRabbit's final review on `063f2e5` plus its two follow-ups: timestamped Academy unchecks now clear legacy checkmarks and stale timestamp-free snapshots can no longer overwrite newer timestamped values; assignment merge selects correctness with the winning answer/timestamp; assignment reward reconciliation logs and defers ledger failures so list/detail reads stay available while submissions still return `ErrRewardUnavailable`; an incoming submitted AstroQuiz snapshot is merged as one immutable answers+score unit; AstroQuiz save/checkpoint/submit payloads now contain only locally dirty answers, cleared only when the current draft still matches the saved request. Checkpoints pause after 2 minutes without input or while the tab is hidden, so an abandoned tab cannot keep resetting the server gap. Added regression tests for all four backend cases. Verified with full `go test -race`/vet/build, forced typecheck/lint/test/build, i18n parity, and live browser checks: a Q1 save body contained only `{"1":...}` while Q2 stayed cached, idle checkpoints stopped and resumed after activity, and the Supply Pod retry remained idempotent. |
| 2026-11-28T07:35Z | `agent-pr5-review` | `PR5-REVIEW-ROUND4` | `[x]` | Closed the five findings from the latest CodeRabbit review on `0395fad`: Academy auth failures now propagate (DB errors return 500 instead of a misleading 401); assignments reload/merge store state on every list/detail call so records written by another instance appear without a restart (new cross-instance regression test); AstroQuiz checkpoints in-progress papers every 60s so the server-measured timer survives long working sessions and the 15-minute gap cap; QuestionPlayer only shows "Saved" when the saved revision still matches the visible draft; submitted AstroQuiz papers with a pending Supply Pod award show a retry button, hidden once the weekly pod is claimed. Verified: `gofmt`, `go vet`, `go build`, `go test -race ./...` against PostgreSQL, forced `typecheck`/`lint`/`test`/`build`, `i18n:check`; live browser check confirmed the 60s checkpoint persisted `elapsedSec=60`, stale-draft Save did not show "Saved", and Supply Pod retry changed the banner to `+5 AC` with one idempotent ledger row. |
| 2026-11-28T07:15Z | `agent-pr5-review` | `PR5-REVIEW-ROUND3` | `[x]` | Closed the latest review round: per-field timestamps prevent stale Academy drafts/self-checks and AstroQuiz answers from overwriting newer state; submitted attempts keep answers+score as one unit; canonical persisted claim week is reloaded before the ledger; assignment records use per-record revisions for persistence acks, request-ID dedupe in merged state, completion derived from merged answers with reward reconciliation on load; profile child listing restricted to demo-parent; training skills accepts `studentId` only for a verified linked parent; AstroQuiz passes submitting to the player and prefers server answers after submit; caption URLs never fall back to a mismatched language; Academy mutation callbacks are session-generation guarded; profile mutation callbacks capture the account ID in variables. Verified with full `go test -race`, vet/build, forced typecheck/lint/test/build, OpenAPI regen, i18n parity, and new concurrency/stale-snapshot tests. |
| 2026-11-28T07:10Z | `agent-pr5-security` | `PR5-REVIEW-FIX-3` | `[x]` | On top of `PR5-REVIEW-ROUND2`, closed the remaining easy/medium findings: enabling OTP now requires a verified code (no lockout without a delivery provider) and family settings + the OTP flag commit in one transaction that re-checks OTP under row locks; AstroQuiz timer is server-measured (client `elapsedSec` ignored, 15-min idle gap cap, 24h cap) and the Supply Pod week/deadline follow the user's timezone; parent streak uses the parent's timezone; Academy/AstroQuiz progress writes are student-only (parents/teachers 403, adults may print answer keys via `answerKeyAvailable`); 400/404s no longer echo internal errors; OpenAPI `security` declared. Web: cache cleared on every identity change (hydrate/refresh too), account-scoped keys for AstroQuiz/print/assignments/parent, late mutation results keyed by request identity, OTP code required when turning OTP on, profile sends only changed fields, "Saved" only after success, 44px targets, YouTube (`youtube-nocookie`) lesson videos via `storyYoutubeIds`/`conceptYoutubeIds` in `academy/media.json` (IDs still empty). Verified: `go vet`, `go test -race ./...` incl. PostgreSQL suite (local PG16 + seed), OpenAPI regen, `i18n:check`, `typecheck`, `lint`, `test`, `build`, Playwright against the live API at 375/768/1440. Open: child-scoped skill picker, real video IDs, pre-existing 120px top-nav overflow at 375px. |
| 2026-11-28T06:45Z | `agent-pr5-review` | `PR5-REVIEW-ROUND2` | `[x]` | Follow-up review round: state saves now snapshot immutable maps and merge under a PostgreSQL row lock (`SaveMerged`), with a concurrent-write restart test; loaded-state double checks; assignment create request IDs and pending-persist retries; Academy scroll retry endpoint/UI; AstroQuiz original claim week persisted before the ledger with a week-boundary test; profile patches update only supplied columns and reject student callers; profile children listing + child selector; print answer keys use a separate submission-gated query; Academy media loads from embedded `media.json`; timer capped at 24h; editing locked while submitting. Verified with full `go test -race`, vet/build, forced typecheck/lint/test/build, i18n parity, and browser checks for print keys, student profile restrictions, and Grade-3 child switching. |
| 2026-11-28T06:29Z | `agent-pr5-security` | `PR5-REVIEW-FIX-2` | `[x]` | Closed the gaps left after `PR5-REVIEW-FIX` (verified by HTTP probes): `PATCH /profile/settings` and `GET /parent/overview` now require a parent session (students got 200), settings input is validated (language, curricula, HH:MM window), OTP fails closed without a verifier, and overview/settings no longer echo internal errors. New `internal/platform/reqguard` bounds answer maps (≤10 parts, ≤64-byte values) in Academy/AstroQuiz/Assignments, caps builder payloads (title ≤120, ≤20 templates) and AstroQuiz `elapsedSec` (≤24h), and only accepts `Bearer` Authorization headers. Web: profile save and Parent Portal gated to parents with a new `parentOnlyNotice` key (`en`/`id`). Verified: `go vet`, `go test -race ./...`, OpenAPI regen, `i18n:check`, `typecheck`, `lint`, `test`, `build`. Not run: `golangci-lint` (binary built with go1.25 < go1.27 target), PostgreSQL integration suite. |
| 2026-11-28T06:06Z | `agent-pr5-review` | `PR5-REVIEW-FIX` | `[x]` | Resolved PR #5 review findings: Phase 2 state now persists per student/module in `phase2_state` (migration `202611280003`) and survives restarts; Academy/AstroQuiz/Assignments rewards apply outside service locks and stay retryable with idempotency keys; assignment ownership/IDOR, parent-link checks, auth 401s, durable UUID IDs, due-date validation, stable sorting, and duplicate-submit rejection; profile bundle/overview read real linked-child data and the OTP code flow is wired in the settings UI; locale query params publish on all bilingual operations; frontend cache keys, video player, quiz draft/timer restore, per-question Save, print MCQ/answer-key labels, and bilingual strings fixed. Content manifests are now enforced by Go parity tests. Verified: full Go suite with disposable PostgreSQL (including restart test), `go vet`, `go build`, OpenAPI regen, `pnpm i18n:check`, forced `typecheck`/`lint`/`test`/`build`, and browser checks for OTP settings, assignment builder, resumed quiz, Indonesian parent portal, and the video player. No new env vars; API paths unchanged. |
| 2026-11-28T03:31Z | `agent-pr3-integrity` | `PR3-REVIEW-ROUND3` | `[x]` | Streak PostgreSQL paths no longer take the in-memory mutex: reads copy under the lock, mutations use `mutateRecord`, and the Daily Orbit transaction path is lock-free. Added a `MaxConns=1` regression that submits while `GetStatus` loops. Full race suite, vet, build, and unchanged OpenAPI pass. |
| 2026-11-28T03:02Z | `agent-pr3-integrity` | `PR3-REVIEW-ROUND2` | `[x]` | Streak now commits inside the Daily Orbit transaction, eliminating the second pool connection; PostgreSQL session/prefs caches are no longer populated, request contexts reach all lock waits, and midnight resets update only the dated count. Tests now assert rolled-back receipts/stats and both persisted answered slots; High Score stars update only after 10 answers. A one-connection pool regression passes, and the old split path was confirmed to hang. Full race suite, vet, build, and unchanged OpenAPI pass. |
| 2026-11-28T02:35Z | `agent-pr3-integrity` | `PR3-SESSION-RECOVERY` | `[x]` | Daily Orbit mutations now hold a per-student advisory lock and commit session, prefs, wallet, and proficiency together; no partially accepted answer survives a rollover. Training loads sessions with `FOR UPDATE` and commits XP, stats, and the answered slot atomically. Streak midnight reconciliation clears only a stale date's count. Verified with PostgreSQL restart and concurrent-instance regressions (both fail with the locks removed), full race suite, vet, build, and unchanged OpenAPI. |
| 2026-11-28T01:16Z | `agent-pr3-integrity` | `PR3-STREAK-RETRY` | `[x]` | Daily Orbit stores the accepted answer before streak recording; the streak counter and calendar now write atomically. Playwright matches three mock endpoints by pathname. PostgreSQL milestone failure/restart regression, Go race suite/vet/build, and both targeted Chromium desktop E2E scenarios passed. No additional migration or API change. |
| 2026-11-28T01:10Z | `agent-pr3-integrity` | `PR3-LEARNING-INTEGRITY` | `[x]` | Fixed fraction grading, elapsed times, triangle choices, and skill attribution in `mathengine`, Training catalog, and seed. Daily Orbit persists pending Proficiency work and deduplicates attempts with migration `202611280002`; reseed existing template links, no API or env changes. Race suite, vet, build, OpenAPI comparison, PostgreSQL reseed and repeated restart/retry regression passed. |
| 2026-11-28T00:52Z | `agent-pr3-integrity` | `PR3-INTEGRITY` | `[x]` | Daily Orbit, Streak, and Training now reject failed PostgreSQL reads instead of writing default state; the final skip stays retryable if its bonus fails. Verified with Go race tests, vet, build, OpenAPI comparison, and a disposable PostgreSQL read-failure and restart probe. No migrations or environment changes. |
| 2026-11-27T01:33Z | `agent-phase3` | `P3-BE-01..P3-CNT-03` | `[x]` | Completed all 13 Phase 3 tasks on branch `feat/p3-gamification-stories-social`: Go APIs & unit/race tests for Super Nova (`supernova`: Super Sight 5-diff normalized hit-detection + Super Warp 2-min sprint + Practice isolation + weekly S score), Planet Stories (`stories`: 6 Grade Planets, free 20-page Opening + atomic idempotent 20 AC unlock + page bookmarks), Customization (`customization`: 8 Avatar packs, My Astro Buddy builder, Galaxy Showcase `>=1 SP` masked names, Daily Star Badges 1–4★ + atomic 20 AC recovery), Social (`social`: Star Duels strict 3 energy/day TZ reset + grade-isolated masked matchmaking + idempotent SP + Star Crew unique 9-digit codes), and Leaderboards/Events (`leaderboards`: 6 Open + 4 Seasonal challenges, `<50ms` 10k-pilot benchmark, Top 50 & Nearby ±5, Hall of Fame, Sunday Meteor 10-Q, Quarterly Creator Challenge, Notifications & idempotent +2 AC Daily Bonus Gift); bilingual content packs (`content/stories/`, `content/badges/daily-star-badges-365.json`, `content/super-sight/puzzles-30.json`); 2 new i18n namespaces (`stories`, `social` in `en`/`id`); and 9 new responsive React pages + reusable SVG `AstroBuddyAvatar` + live Bonus Gift modal & Notifications dropdown in `TopNavShell`. Verified: `go vet`, `go test -race` (all 18 packages PASS), 3 Go binaries, OpenAPI 3.1 → TS sync, `pnpm i18n:check` (11 namespaces), `typecheck`, `lint`, `vitest`, `vite build` + PWA, and 0 forbidden brand hits. Phase 4 tasks unblocked. |
| 2026-11-26T23:34Z | `agent-phase2` | `P2-BE-01..P2-CNT-02` | `[x]` | Completed all 10 Phase 2 tasks on branch `feat/p2-academy-quiz-parent`: Space Academy Go API & UI (`academy`: 12 G1–G6 topics, 4-stage timeline Story Launch/Watch & Learn/Do & Learn with Save+Try Again+atomic Star Scrolls/Self-Check), AstroQuiz Go API & UI + chromeless A4 `@media print` Worksheet renderer (`astroquiz`: 18 papers across Essential/Top School/HOTS × G1–G6, live timer, hidden solutions until full submit, 1/week Supply Pod +5 AC cap), Assignments + Parent Custom Assignment Builder (`assignments`: linked-child authorization, proficiency-tagged skill picker, AC rewards), Profile/OTP-Gated Family Settings/Help/Parent Mission Control (`profile`: OTP gate, Brain Galaxy 0–25 min cap, language/curricula toggles, auto-promotion display), plus bilingual `content/academy/` & `content/quizzes/` manifests and 3 new i18n namespaces (`academy`, `astro-quiz`, `portals`). Verified: `go vet`, `go test -race` (all 13 packages PASS), 3 Go binaries, OpenAPI 3.1 → TS sync, `pnpm i18n:check` (9 namespaces), `typecheck`, `lint`, `vitest`, `vite build` + PWA all green. Phase 3 tasks unblocked. |
| 2026-11-26T23:16Z | `agent-phase1` | `P1-ENG-01..P1-CNT-02` | `[x]` | Completed all 12 Phase 1 MVP tasks on branch `feat/p1-core-learning-mvp`: Go server-authoritative `mathengine` (24 deterministic bilingual G1–G6 procedural templates, dot/comma decimal & fraction equivalence grader, EN/ID TTS verbalizer), bilingual `content/curricula/` (6 tracks + HOTS 4 quadrants/9 heuristics) & `content/questions/` packs, Go APIs for Daily Orbit (`dailyorbit`: 10-Q tiered path, 4 tiers, 3x change question, 409 lock, skip, +5 AC bonus), Orbit Streak (`streak`: 5-correct milestone, Nova Pet, max-3 Cryo Shields midnight reconcile), Galaxy Training (`training`: 6 curricula, HOTS wheel, Proficiency % & High Score stars), Reports (`reports`), synced OpenAPI 3.1 → `@planetlogi/api-client`, 4 interactive on-screen Math Tools (`StarSketchpad`, `EquationPad`, `BarModelBuilder`, `ColumnOperationsGrid`), Unified `QuestionPlayer` (4 modes + TTS + SVG diagrams), and full Dashboard/Exp-Level/Daily Orbit/Streak/Training/HOTS/Reports UI in `en` & `id`. Verified: `go vet`, `go test -race` (all 9 packages), 3 Go binaries, `pnpm i18n:check` (6 namespaces), `typecheck`, `lint`, `vitest`, `vite build` + PWA all green. Phase 2 tasks unblocked. |
| 2026-11-26T15:45Z | `agent-phase0` | `P0-01..P0-08` | `[x]` | Completed all 8 Phase 0 tasks: polyglot monorepo (`apps/web` Vite+React+TanStack Router+PWA for Cloudflare Pages, `apps/api` Go Chi/Huma+`asynq`+`/healthz`+`/readyz`+graceful shutdown, Docker/CI), multi-subject PostgreSQL 16 `goose` migration + seed CLI (`pilot1`/`planet123`), bilingual `@planetlogi/i18n` (`en`/`id` + CI parity script), `@planetlogi/ui` Galaxy Design System, Go Auth/RBAC/OTP module + OpenAPI→`@planetlogi/api-client`, atomic double-entry Economy Ledger + Timezone Operating-Window service, Login (Live SP feed) + Persistent App Shell, and Workbox PWA build (`dist/sw.js`, `manifest.webmanifest`, `_redirects`). Verified: `pnpm i18n:check`, `pnpm -r typecheck`, `pnpm -r lint`, `pnpm -r test`, `pnpm -r build`, `go vet ./...`, `go test -race ./...`, `pnpm api:gen` all green. Phase 1 tasks are now unblocked. |
| 2026-11-26T07:45Z | `architect-planner` | `PLAN-INIT` | `[x]` | Created PlanetLogi A–Z Master Plan (`PLAN.md`), Multi-Agent Protocol (`AGENTS.md`), Task Board (`TASKS.md`), and modular specifications (`docs/01`–`09`). Ready for Phase 0 (`P0-01`) kickoff. |
