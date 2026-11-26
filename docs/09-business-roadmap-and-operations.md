# 09 — Content Pipeline, Monetization, Implementation Roadmap & Operations

> **Owner Tracks**: `PM`, `COORDINATOR`, `CONTENT`, `OPS`, `BILLING`  
> **Scope**: Bilingual content production strategy, B2C & B2B monetization model, phased A–Z execution roadmap, parallel multi-agent staffing plan, success KPIs, and risk mitigation.

---

## 1. Bilingual Content Production Strategy (Grades 1–6)

Content is the heartbeat of AlgiPlanet. Using a **hybrid Procedural + Curated pipeline** dramatically reduces authoring cost while maximizing pedagogical richness:

| Content Asset | MVP Target (Phase 1–2) | Full Launch Target (Phase 4–5) | Production Approach |
|---|---|---|---|
| **Procedural Question Templates** | 60 templates (G1–G6 core ops) → generates 10,000+ unique items | 150+ templates → generates 50,000+ unique items | Authored in Go + JSON templates (`apps/api/internal/mathengine/`) with automated seed & bilingual validation |
| **Curated Word, HOTS & Explorer Problems** | 300 bilingual items | 1,500+ bilingual items (incl. Bar Model & SVG visuals) | Authored side-by-side (`en` + `id`) in Admin CMS with pedagogy review |
| **Space Academy Guided Topics** | 12 core topics (2 per grade) | 30+ topics/grade roadmap (start with 60–90 high-impact lessons) | Concise 2–4 min animated concept videos (bilingual voice/captions) + Do & Learn + Self-Check |
| **AstroQuiz Exam Papers** | 18 papers (3 categories × 6 grades) | 36–60 papers (revisions, Top School year-end, HOTS) | Assembled from vetted static & seeded questions + verified A4 print layout |
| **Planet Stories** | 6 Opening Chapters (1 per grade planet, 20 pages each) | 6 Opening + 12 unlockable chapters (Prologue/Ch1–2) | Text-free cosmic illustrations + dynamic `en`/`id` text overlays |
| **Super Sight Visual Puzzles** | 30 spot-the-difference pairs | 100+ seasonal & cosmic pairs | Layered vector scenes with 5 exact difference bounding boxes |
| **Daily Star Badges Trivia** | 365 bilingual space/math trivia cards | 365+ plus seasonal holiday editions | Batch-authored JSON verified by elementary educators |

---

## 2. Monetization & Entitlement Model (Deferred to Phase 5 — Feature-First)

> **Build Order Policy**: AlgiPlanet follows a strict **Feature-First** rollout. Throughout **Phases 0–4**, every student, parent, teacher, story, quiz, and Brain Galaxy feature ships **100% unlocked** (zero paywalls, `BILLING_ENABLED=false`). Monetization (`P5-BE-01`, `P5-FE-01`) is activated ONLY in **Phase 5** once all core features are complete and validated.

When enabled in Phase 5, AlgiPlanet introduces a generous, child-friendly **Freemium B2C** model (parent-gated checkout only) alongside **School B2B** licenses.

### 2.1 Plan Comparison
| Feature | Free Explorer (B2C) | AlgiPlanet Galaxy Premium (B2C) | School Fleet License (B2B) |
|---|---|---|---|
| **Daily Orbit (10 Qs/day)** | Included (Elementary & Intermediate) | Full (All 4 Tiers + All 6 Curricula) | Full + School Curriculum Sync |
| **Orbit Streak & Cryo Shields** | Included | Included + Bonus Monthly Cryo Shield | Included |
| **Galaxy Training (Self-Practice)** | Selected foundational skills | Unlimited G1–G6 + Full HOTS Wheel + Explorer Series | Unlimited + Teacher Remediation Links |
| **Space Academy (Video Lessons)** | First lesson per topic | Unlimited Topics & Star Scrolls | Unlimited |
| **AstroQuiz & Printables** | 1 sample paper / grade | Unlimited Papers + Printable A4 + Weekly Supply Pods | Unlimited + Teacher Answer Keys |
| **Planet Stories** | All 6 Opening Chapters | All Chapters unlockable via earned Astro Coins | All Chapters |
| **Brain Galaxy Mini-Games** | 10 min/day (`08:00–20:00`) | 25 min/day (`08:00–20:00`, parent-tunable) | 25 min/day (`08:00–18:00` school policy) |
| **Parent Portal & Custom Homework** | Weekly summary only | Full Proficiency Analytics + Mistake Review + Custom Assignment Builder | Included + Teacher Portal & Class Heatmaps |

### 2.2 Localized Billing Strategy
- **Indonesia (`IDR`)**: Midtrans / Xendit supporting **QRIS**, **Virtual Accounts (BCA/Mandiri/BNI/BRI)**, and **e-Wallets (GoPay/OVO/Dana/ShopeePay)** — Monthly, Quarterly, and Annual Family plans (multi-child sibling discount).
- **International (`USD`)**: Stripe supporting credit/debit cards and Apple Pay / Google Pay — Monthly and Annual plans.
- **School B2B**: Annual per-seat school invoicing with bulk license keys and CSV onboarding.

---

## 3. Phased Implementation Roadmap (A–Z)

All tasks below map 1-to-1 to Task IDs in [`TASKS.md`](./TASKS.md).

### Phase 0 — Foundation & Architecture (Weeks 1–2)
- **Objective**: Stand up monorepo, PostgreSQL/Redis infrastructure, full DB schema v1, `en`/`id` i18n framework, AlgiPlanet Galaxy Design System, Auth/RBAC, Persistent App Shell, PWA manifest, and Economy Ledger core.
- **Milestone Gate**: A test student can log in on mobile/tablet/desktop, toggle English/Indonesian, see the Galaxy shell, and install the PWA.

### Phase 1 — Core Learning Engine & Daily Orbit MVP (Weeks 3–6)
- **Objective**: Ship the heart of AlgiPlanet — Unified Question Player, 4 Interactive Math Tools, Procedural Generator, Daily Orbit (10-Q path, 4 tiers, 3x Change Question), Orbit Streak + Nova Pet + Cryo Shields, Galaxy Training (6 curricula + HOTS 4-Quadrant Wheel), and Proficiency/High-Score Reports.
- **Milestone Gate**: Internal dogfooding with Grades 1–6 students completing Daily Orbit, solving word problems with Bar Model Builder, and viewing accurate Proficiency % reports.

### Phase 2 — Space Academy, AstroQuiz, Assignments & Parent Portal (Weeks 7–10)
- **Objective**: Add structured learning (*Space Academy* videos + *Do & Learn* with *Try Again* + *Self-Check*), timed & A4-printable *AstroQuiz* papers with Weekly Supply Pods, Student Assignments, and the **Parent Portal** (including Parent Custom Assignment Builder & OTP-protected Settings).
- **Milestone Gate**: Parents can review weak topics, hand-pick 5 questions to assign to their child, and print an A4 AstroQuiz worksheet.

### Phase 3 — Gamification, Planet Stories, Social & Cosmic Events (Weeks 11–14)
- **Objective**: Ignite retention & delight — *Super Nova Challenges* (*Super Sight* & *Super Warp*), *Planet Stories* (6 Grade Worlds + 20-page digital viewer), *My Astro Buddy* customizer & *Galaxy Showcase*, *Daily Star Badges* (4 tiers + 20 AC recovery), 8 Avatar packs, *Star Duels* (1v1, 3 energy/day), *Star Crew* (9-digit code), Redis-powered *Galaxy Leaderboards* (6 Open + 4 Seasonal), *Sunday Meteor Challenge*, Notifications, and Bonus Gifts.
- **Milestone Gate**: Full feature parity with the complete gamified social loop; leaderboard refreshes cleanly every 5 minutes with masked peer names.

### Phase 4 — Brain Galaxy, School/Teacher Portal & Admin CMS v2 (Weeks 15–18)
- **Objective**: Launch *Brain Galaxy* (5 cognitive HTML5 mini-game zones under strict 25-min/day server cap), *Teacher & School Portal* (CSV import, printable login slips, class mastery heatmap, bulk assignments, annual grade rollover), *Admin Bilingual CMS v2*, Server-side Neural TTS audio caching, and Release-Candidate G1–G6 content expansion.
- **Milestone Gate**: Pilot elementary school onboards a 30-student class via CSV and runs a 2-week classroom pilot cleanly.

### Phase 5 — Monetization, Security Audit, Scale & Public Launch (Weeks 19–22)
- **Objective**: Integrate Midtrans/Xendit + Stripe billing, build bilingual marketing site with interactive math sample, run UU PDP/COPPA security & privacy audit, execute 5,000-CCU k6 peak load tests, complete Lighthouse PWA audits, run Closed Beta, and launch publicly.
- **Milestone Gate**: Zero High/Critical security findings, P95 `<250ms` under peak evening traffic, live billing verified in IDR & USD, Production live.

---

## 4. Parallel Multi-Agent Staffing Matrix

Up to **6–8 specialized agents** can operate concurrently by following [`AGENTS.md`](./AGENTS.md) and claiming independent tracks in [`TASKS.md`](./TASKS.md):

| Agent Slot | Primary Track(s) | Parallel Focus Across Phases |
|---|---|---|
| **Agent 1 (`agent-infra`)** | `INFRA`, `DB` | Monorepo, Docker, Coolify/Cloudflare Pages CI/CD, PostgreSQL/`goose`/`sqlc`, PWA, Observability, Launch |
| **Agent 2 (`agent-ui`)** | `UI`, `SHARED` | Galaxy Design System, i18n framework, SVG Astro Buddy & Badge shaders |
| **Agent 3 (`agent-engine`)** | `ENGINE`, `GAMES` | `math-engine` procedural generator, 4 Math Tools, Question Player, Brain Galaxy mini-games |
| **Agent 4 (`agent-be-core`)** | `BE` (Core & Portal) | Auth/RBAC, Economy Ledger, Daily Orbit, Streak, Training, Academy, Parent/Teacher APIs, Billing |
| **Agent 5 (`agent-be-social`)** | `BE` (Social & CMS) | Super Nova, Stories, Badges, Star Duels, Star Crew, Redis Leaderboards, Events, Admin CMS, TTS |
| **Agent 6 (`agent-fe-1`)** | `FE` (Core Student) | Login, Shell, Dashboard, Daily Orbit, Streak, Training, HOTS Wheel, Reports, Space Academy |
| **Agent 7 (`agent-fe-2`)** | `FE` (Fun & Portals) | AstroQuiz + Printables, Stories Viewer, Super Nova, Duels, Leaderboards, Parent/Teacher/Admin UI |
| **Agent 8 (`agent-content-qa`)** | `CONTENT`, `QA` | Bilingual G1–G6 Curricula, Question Packs, Stories, Trivia, Playwright E2E, Security & Load Tests |

---

## 5. Key Success Metrics (KPIs)

- **Learning Habit**: Daily Orbit completion rate (`>= 65%` of active daily users), 7-day Orbit Streak retention (`>= 35%`).
- **Pedagogical Impact**: Average skill Proficiency lift (`+20%` accuracy after 4 weeks of consistent practice), Space Academy lesson completion rate (`>= 70%`).
- **Engagement & Balance**: Weekly Active Days (`>= 4.0 days/week`), healthy Brain Galaxy average usage (`<= 20 min/day`, zero bypasses).
- **Business & Technical Health**: Free-to-Premium conversion (`>= 6–8%`), Monthly Churn (`< 5%`), Crash-free sessions (`>= 99.5%`), API P95 latency (`< 250ms`), Bilingual QA parity (`100%`).

---

## 6. Risk Register & Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| **Parallel agent merge conflicts** on shared schema/i18n files | High | Enforce `TASKS.md` `[~]` claim locks, namespaced per-feature locale JSONs, and directory ownership rules in `AGENTS.md` |
| **Question bank running dry or low quality** | High | Build deterministic Procedural Generator in Phase 1 + CMS dual-locale QA gate before publishing |
| **Screen-time / game addiction pushback from parents** | Medium | Server-enforced 25-min Brain Galaxy cap, operating hours, earn-to-play Astro Coins, and transparent Parent Portal controls |
| **Child privacy / UU PDP & COPPA non-compliance** | Critical | Zero ad trackers, zero free-text child chat, masked leaderboard names, Parent OTP gate, automated data deletion & Phase 5 security audit |
| **Evening peak traffic spikes (19:00–21:00 WIB)** | Medium | Redis-cached 5-minute leaderboards, CDN caching for stories/TTS/videos, and k6 load testing at 5,000+ CCU before launch |
| **Bilingual layout breaks (Indonesian text overflow)** | Medium | Design all components for +25% copy expansion and run Playwright visual/E2E suites in both `en` and `id` |
