# PlanetLogi — Multi-Agent Execution & Coordination Protocol

This file defines the mandatory workflow for all AI coding agents and human contributors working on **PlanetLogi** in parallel. Every agent MUST follow these rules strictly to avoid merge conflicts, duplicate work, broken dependencies, or inconsistent conventions.

---

## 1. Mandatory Task Lifecycle & Status Marking Rules

All work is tracked centrally in [`TASKS.md`](./TASKS.md). Every task uses one of **four canonical status markers**:

| Marker | Meaning | Format in `TASKS.md` |
|---|---|---|
| `[ ]` | **Unclaimed / Pending** — Ready if all `Depends On` tasks are `[x]` | `- [ ] **P1-FE-01** ...` |
| `[~]` | **In Progress (Locked)** — Actively claimed and being executed by one agent | `- [~] **P1-FE-01** ... *(Owner: agent-fe-1 \| Started: 2026-11-26T08:00Z)*` |
| `[x]` | **Completed & Verified** — Implemented, tested, linted, and acceptance criteria passed | `- [x] **P1-FE-01** ... *(Owner: agent-fe-1 \| Done: 2026-11-26T10:30Z)*` |
| `[!]` | **Blocked** — Cannot proceed due to missing dependency, spec ambiguity, or external blocker | `- [!] **P1-FE-01** ... *(Owner: agent-fe-1 \| Blocked: reason)*` |

### Step-by-Step Execution Protocol
1. **Before Starting Any Work (Claim First!)**:
   - Pull the latest workspace state and read `TASKS.md`.
   - Select **at most 1–2 tightly related unclaimed tasks (`[ ]`)** whose `Depends On` tasks are ALL marked `[x]`.
   - **Immediately edit `TASKS.md`** to mark your task `[~]`, recording your `Agent ID` (e.g., `agent-backend-1`, `agent-ui-2`, `agent-content-1`) and ISO-8601 UTC timestamp.
   - Commit or save the claim in `TASKS.md` before modifying application files so no other parallel agent picks up the same task.
2. **During Execution**:
   - Read the exact linked spec section in `docs/` before implementing.
   - Only modify files within your task's declared **Scope / Directory Ownership** (see Section 2).
   - If you discover a required shared-file change (e.g., `apps/api/db/migrations/`, OpenAPI contract / `packages/api-client/`, or `packages/i18n/` base files), check if another agent currently holds a lock (`[~]`) on that shared area; coordinate via the Handoff Log or split into a tiny prerequisite task.
   - If blocked for any reason, change `[~]` to `[!]` with a clear, actionable blocker note in `TASKS.md` and log it in the **Handoff & Activity Log**.
3. **Upon Completion (Verify Before Marking Done!)**:
   - Verify EVERY Acceptance Criterion listed for the task in `TASKS.md`.
   - Run automated checks relevant to your track (`typecheck`, `lint`, `unit/integration tests`, `build`).
   - Confirm both **English (`en`)** and **Indonesian (`id`)** translation keys exist for every user-facing string you introduced.
   - Update `TASKS.md`: change `[~]` → `[x]`, record completion UTC timestamp, and add a 1–3 line summary in the **Handoff & Activity Log** noting:
     - What was built & key file paths
     - Any new env vars, migrations, or API endpoints downstream agents need to know
     - Verification commands run and their results.
4. **Release Rule**:
   - If an agent stops mid-task without finishing, it MUST either revert incomplete breaking changes and reset `[~]` → `[ ]`, OR document exact partial progress and set `[!]` so the next agent can resume cleanly.

---

## 2. Parallel Workstreams & Directory Ownership Matrix

To allow 4–8 agents to work simultaneously without file conflicts, the monorepo is partitioned into clean ownership zones:

| Track Code | Agent Role | Primary Directory Ownership | Shared Touchpoints (Coordinate!) |
|---|---|---|---|
| `INFRA` | Infra & DevOps Agent | `/`, `docker/`, `.github/`, `packages/config/` | `package.json`, `turbo.json`, `Makefile` |
| `DB` | Database & Schema Agent | `apps/api/db/` (`goose` SQL migrations, `sqlc` queries, seeds) | `apps/api/internal/db/` (generated) |
| `SHARED` | API Contract & i18n Agent | `packages/api-client/`, `packages/i18n/` | OpenAPI spec, locale JSONs |
| `UI` | Galaxy Design System Agent | `packages/ui/`, `apps/web/src/styles/` | Tailwind theme tokens |
| `BE` | Go Backend API & Workers Agent | `apps/api/internal/modules/<module>/` (one agent per module!) | `apps/api/internal/server/router.go` |
| `ENGINE` | Math Engine & Tools Agent | `apps/api/internal/mathengine/` (Go), `apps/web/src/components/question-player/` (React) | OpenAPI question DTOs |
| `FE` | Student & Portal Frontend Agent | `apps/web/src/routes/$locale/<feature>/` (one agent per route group!) | Navigation shell, root layout |
| `GAMES` | Brain Galaxy Mini-Games Agent | `packages/brain-games/`, `apps/web/src/routes/$locale/brain-galaxy/` | Session timer API |
| `CONTENT` | Curriculum & Bilingual Content Agent | `content/curricula/`, `content/questions/`, `content/stories/` | Go seed CLI |
| `QA` | Testing & Security Agent | `tests/e2e/`, `tests/security/`, `tests/load/` | Test fixtures |

### Conflict-Avoidance Rules
- **Modular Routing**: Use TanStack Router file-based routing (`apps/web/src/routes/$locale/...`) on the frontend and per-module `RegisterRoutes(api)` functions in Go (`apps/api/internal/modules/<module>/`) so parallel agents do not collide on giant route files.
- **OpenAPI Contract Sync**: Whenever a Go `BE` agent changes request/response DTOs, regenerate `openapi.json` and `packages/api-client/` (`pnpm api:gen`) so `FE` agents immediately get updated TypeScript types.
- **Per-Feature i18n Namespaces**: Store translations in namespaced files (`packages/i18n/locales/{en,id}/{feature}.json`, e.g., `daily-orbit.json`, `space-academy.json`, `astro-quiz.json`) instead of one giant `common.json` so parallel frontend agents never clash on locale files.
- **Additive SQL Migrations**: Never edit an already-applied `goose` SQL migration file; always add a new timestamped `.sql` migration and run `sqlc generate`.

---

## 3. Non-Negotiable Engineering & Product Guardrails

1. **Original Branding Only**:
   - Every string, comment, code identifier, asset filename, CSS token, and doc must use **PlanetLogi** galaxy terminology. Never reference external competitor brand names anywhere in code, docs, tests, or git history.
2. **Grades 1–6 Scope**:
   - Design all UI, curricula, and content for **Elementary School Grades 1–6 (Primary 1–6, ages 6–12)**.
3. **Bilingual Parity (`en` & `id`)**:
   - Zero hardcoded user-facing strings in TSX/TS components. Every label, toast, error message, math hint, and story page MUST resolve via the i18n system in both `en` and `id`.
   - Maintain consistent math notation localization (e.g., decimal separator & currency display rules defined in [`docs/07-i18n-multilanguage-architecture.md`](./docs/07-i18n-multilanguage-architecture.md)).
4. **Responsive Web + PWA First**:
   - Every screen MUST be tested across **3 viewports**: Mobile (`375px`), Tablet/iPad (`768px–1024px`, primary elementary learning device), and Desktop (`1440px+`).
   - Touch targets for child UI controls MUST be at least `44x44px` (`48x48px` preferred for Grades 1–2).
5. **Server-Side Authority for Economy & Timing**:
   - Never trust client clocks or client-submitted scores for **Star Points (SP)**, **Astro Coins (AC)**, **XP**, **Daily Orbit operating hours (06:00–22:00)**, **Change Question daily limit (3/day)**, **Star Duel energy (3/day)**, or **Brain Galaxy 25-minute daily cap**. All rules MUST be enforced authoritatively on the backend using the user's configured timezone.
6. **Child Privacy & Safety**:
   - Never expose raw student full names, emails, or phone numbers on public/school/national leaderboards or in Peer Challenge matchmaking. Always use masked display names + avatars/Astro Buddies.
   - Parent-sensitive endpoints (settings, OTP, billing, profile edits) require authenticated parent session / OTP verification.
7. **Feature-First Delivery (Monetization Deferred to Phase 5)**:
   - During **Phases 0–4**, ALL product features MUST remain **100% unlocked for all users** (no paywalls, locked modals, or payment gates).
   - Do not implement payment gateways or plan-tier restrictions until **Phase 5** (`P5-BE-01`, `P5-FE-01`), after all core Student, Parent, Teacher, Brain Galaxy, and CMS features are finished. Keep any entitlement checks behind a simple default-allow interface so Phase 5 can wire in plans cleanly without rewriting feature code.

---

## 4. Definition of Done (DoD) Checklist

Before switching any task from `[~]` to `[x]` in `TASKS.md`, confirm:
- [ ] All acceptance criteria in `TASKS.md` and referenced `docs/*.md` are met.
- [ ] Code compiles cleanly (`pnpm typecheck` for web/packages; `go vet ./...` & `go build ./...` for Go API) with zero errors.
- [ ] Linter and formatter pass (`pnpm lint` for TS/React; `golangci-lint run` for Go).
- [ ] Unit/integration tests added and passing for core business logic and API endpoints.
- [ ] Both `en` and `id` locale keys exist and render without missing-key warnings or layout overflow.
- [ ] Responsive behavior verified on mobile, tablet, and desktop widths (for UI tasks).
- [ ] No hardcoded secrets, PII leaks, or competitor brand strings.
- [ ] `TASKS.md` updated to `[x]` with timestamp and Handoff Log entry.

---

## 5. Git Branch & Commit Convention (When Using Git)

- **Branch naming**: `<track>/<task-id>-<short-slug>` (e.g., `be/p1-be-02-daily-orbit-api`, `fe/p1-fe-03-question-player`).
- **Commit format**: `<type>(<scope>): [<TASK-ID>] <concise summary>`
  - Example: `feat(daily-orbit): [P1-BE-02] implement 10-question tiered scoring and 3x change-question limit`
- **Small, Frequent Merges**: Merge completed atomic tasks into `main` promptly so downstream dependent tasks unblock quickly.
