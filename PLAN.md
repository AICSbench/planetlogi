# AlgiPlanet — Master Engineering & Product Plan (A–Z)

> **AlgiPlanet** is a galaxy-themed, gamified, bilingual (**English & Indonesian**) elementary mathematics learning platform for **Grades 1–6 (Primary 1–6 / SD Kelas 1–6, ages 6–12)**, built **Responsive Web + PWA first** (optimized for tablets/iPads, laptops, and mobile devices) with an API-first backend ready for future native mobile apps.

---

## 1. Executive Summary

AlgiPlanet transforms daily elementary math practice into an engaging cosmic expedition across six themed planets (Grades 1–6). Students build consistent daily study habits through short, tiered challenges, structured video-and-practice lessons, interactive Singapore-style problem-solving tools, illustrated math storybooks, healthy screen-time-capped cognitive mini-games, and safe peer competition — while parents and teachers gain real-time skill proficiency analytics and personalized assignment controls.

### Core Product Pillars
1. **Star Challenges** — Daily Orbit (10-question tiered daily challenge), Orbit Streak & Cryo Shields, Super Nova Challenges (Visual & Speed sprints), Star Duels (1v1 Peer Challenge), and Cosmic Events (Sunday Meteor & Quarterly Creator Challenge).
2. **Galaxy Training** — Multi-curriculum self-practice across 6 tracks (**Kurikulum Merdeka**, **Singapore Math MOE**, **Cambridge Primary 0096**, **Challenging Maths**, **HOTS 4-Quadrant Heuristic Wheel**, and **Real-World Explorer Series**).
3. **Space Academy** — Structured guided learning paths (*Story Launch → Concept Briefing → Guided Practice "Do & Learn" → Mission Self-Check*) awarding Star Scrolls.
4. **Cosmic Fun** — **Planet Stories** (6 grade-level planets with illustrated digital storybooks unlocked by learning) and **Brain Galaxy** (5 cognitive mini-game zones protected by a strict 25-minute/day healthy cap).

### Key Differentiators & Constraints
- **Elementary Focus (Grades 1–6 / SD 1–6)**: Tuned for ages 6–12 across foundational arithmetic, word problems, bar-modeling, and Olympiad/HOTS heuristics.
- **Responsive Web + PWA First**: Installable on iPad, Android tablets, Chromebooks, desktops, and phones; offline-resilient shell and printable worksheet support.
- **Bilingual From Day One (`en` + `id`)**: Full i18n architecture covering UI, math problems, worked solutions, TTS read-aloud voices, storybooks, videos, and notifications — designed to add new locales cleanly later.
- **Healthy Child-Safe Gamification**: Operating hours enforcement (e.g., 06:00–22:00 for challenges, 25 min/day max for Brain Galaxy), masked peer names on public leaderboards, OTP-protected Parent Settings, and strict compliance with child data privacy standards (UU PDP & COPPA principles).

---

## 2. AlgiPlanet Galaxy Theme & Naming Glossary

All UI copy, code identifiers, database models, and assets MUST use **AlgiPlanet**'s original galaxy terminology:

| Domain Concept | AlgiPlanet Official Name (EN) | AlgiPlanet Official Name (ID) |
|---|---|---|
| Platform Name | **AlgiPlanet** | **AlgiPlanet** |
| Daily Challenge | **Daily Orbit** | **Orbit Harian** |
| Competitive Points | **Star Points (SP)** | **Poin Bintang (SP)** |
| Spendable Currency | **Astro Coins (AC)** | **Koin Astro (AC)** |
| XP & Rank | **Pilot XP & Rank (Lvl 1–100)** | **XP & Pangkat Pilot (Lvl 1–100)** |
| Streak & Mascot | **Orbit Streak & Nova Pet** | **Streak Orbit & Hewan Nova** |
| Streak Freeze | **Cryo Shield** (max 3) | **Perisai Krio** (maks 3) |
| Super Challenge | **Super Nova Challenge** (*Super Sight* & *Super Warp*) | **Tantangan Super Nova** (*Penglihatan Super* & *Kecepatan Warp*) |
| Self-Practice Hub | **Galaxy Training** | **Latihan Galaksi** |
| Guided Video Lessons | **Space Academy** | **Akademi Antariksa** |
| Guided Lesson Tokens | **Star Scrolls** | **Gulungan Bintang** |
| Digital Storybooks | **Planet Stories** | **Cerita Planet** |
| Cognitive Mini-Games | **Brain Galaxy** | **Galaksi Otak** |
| Timed/Printable Papers | **AstroQuiz** | **Kuis Astro** |
| Weekly Quiz Reward | **Supply Pod** | **Kapsul Hadiah** |
| 1v1 Peer Battle | **Star Duels** | **Duel Bintang** |
| Friend Team System | **Star Crew** (9-digit Crew Code) | **Kru Bintang** (Kode 9 Digit) |
| Custom Character | **My Astro Buddy** & **Galaxy Showcase** | **Teman Astroku** & **Pameran Galaksi** |
| Daily Collectible Cards | **Daily Star Badges** (Bronze/Silver/Gold/Holographic) | **Lencana Bintang Harian** |
| Sunday Event | **Sunday Meteor Challenge** | **Tantangan Meteor Minggu** |
| 6 Grade Planets | G1 *Terra Pebble*, G2 *Tropica*, G3 *Aquaris*, G4 *Crystalia*, G5 *Lumina*, G6 *Solaris* | G1 *Planet Kerikil*, G2 *Planet Tropika*, G3 *Planet Akuaris*, G4 *Planet Kristalia*, G5 *Planet Lumina*, G6 *Planet Solaris* |

---

## 3. Documentation Map (A–Z Specifications)

Read ONLY the documents relevant to your assigned track, plus [`AGENTS.md`](./AGENTS.md) and [`TASKS.md`](./TASKS.md).

| Document | Scope | Primary Audience (Agent Tracks) |
|---|---|---|
| [`AGENTS.md`](./AGENTS.md) | **Multi-agent operating protocol**, task claiming/status rules, file ownership, PR/commit standards, QA gates | **ALL Agents (Mandatory First Read)** |
| [`TASKS.md`](./TASKS.md) | **Master parallel execution board** (Phase 0–5, atomic Task IDs, dependencies, status checkboxes, handoff log) | **ALL Agents (Live Coordination Board)** |
| [`docs/01-product-vision-and-theme.md`](./docs/01-product-vision-and-theme.md) | Vision, personas, Galaxy UI/UX design system, color tokens, responsive/PWA rules, accessibility | Design, Frontend, Product |
| [`docs/02-features-student-portal.md`](./docs/02-features-student-portal.md) | Exhaustive Student Portal specs (Shell, 4 Pillars, Daily Orbit, Super Nova, Training, Academy, Stories, AstroQuiz, Duels, Leaderboards, Brain Galaxy, Reports, Settings) | Frontend, Backend, QA |
| [`docs/03-question-engine-and-math-tools.md`](./docs/03-question-engine-and-math-tools.md) | Question Player (4 mode behaviors), input widgets, Procedural Question Generator, 4 Interactive Tools (Scratchpad, Equation Pad, Bar Model, Column Calc), TTS, Printables | Engine, Frontend, Content |
| [`docs/04-curriculum-and-pedagogy.md`](./docs/04-curriculum-and-pedagogy.md) | Grades 1–6 Multi-Curriculum map (Kurikulum Merdeka, Singapore MOE, Cambridge 0096, Challenging, HOTS 4-Quadrant/9 Heuristics, Creator Series), Mastery & Star algorithms | Content, Backend, Analytics |
| [`docs/05-gamification-social-and-safety.md`](./docs/05-gamification-social-and-safety.md) | Dual economy (SP/AC), XP Levels 1–100, Streak/Cryo Shields, Badge tiers, Screen-Time & Hours Engine, Privacy/OTP/Anti-Cheat | Backend, Gamification, Security |
| [`docs/06-portals-parent-teacher-admin.md`](./docs/06-portals-parent-teacher-admin.md) | Parent Portal (custom assignment builder, controls, reports), Teacher/School Portal (rosters, bulk tasks), Admin CMS (bilingual authoring, events, moderation) | Full-Stack, Admin/CMS |
| [`docs/07-i18n-multilanguage-architecture.md`](./docs/07-i18n-multilanguage-architecture.md) | Complete EN + ID localization architecture (UI strings, bilingual questions/solutions, number formatting, TTS voices, stories, videos, QA) | Frontend, Backend, Content |
| [`docs/08-technical-architecture-and-database.md`](./docs/08-technical-architecture-and-database.md) | System architecture, Tech Stack, PostgreSQL schema (ERD + tables), Redis patterns, REST/WS API contracts, PWA caching, CI/CD & testing | Architect, Backend, DevOps |
| [`docs/09-business-roadmap-and-operations.md`](./docs/09-business-roadmap-and-operations.md) | Content pipeline, Freemium B2C + School B2B monetization, Phase 0–5 Roadmap, parallel agent staffing matrix, KPIs, risk register | PM, Coordinator, Ops |

---

## 4. High-Level Roadmap Summary

| Phase | Name | Primary Deliverable | Parallel Tracks Active |
|---|---|---|---|
| **Phase 0** | Foundation & Architecture | Monorepo, DB schema, Auth (Parent/Child), i18n (`en`/`id`), Galaxy Design System, PWA shell, CI/CD | Infra, Design System, Auth, i18n |
| **Phase 1** | Core Learning Engine (MVP) | Question Player, 4 Math Tools, Procedural Generator, Daily Orbit, Orbit Streak + Cryo Shields, Galaxy Training (G1–G6 Curricula), Proficiency & High-Score Reports | Engine, Frontend, Backend, Content |
| **Phase 2** | Structured Content & Assessments | Space Academy (Video + Do & Learn + Self-Check), AstroQuiz (Timed + Printable + Supply Pods), Assignments + Parent Custom Assignment Builder, Parent Portal | Portal, Academy, Quiz, Content |
| **Phase 3** | Gamification, Stories & Social | Planet Stories (6 Worlds + 20-page Reader), My Astro Buddy + Showcase, Daily Star Badges, Avatars, Star Duels (1v1), Star Crew, Galaxy Leaderboards, Super Nova Challenges, Cosmic Events | Gamification, Social, Realtime, Media |
| **Phase 4** | Brain Galaxy & School Ecosystem | Brain Galaxy (5-zone HTML5 cognitive mini-games + 25-min healthy cap), Teacher & School Portal (rosters, bulk assignments, class/school rankings, auto grade promotion), Admin CMS v2 | Mini-Games, B2B Portal, CMS |
| **Phase 5** | Monetization, Scale & Launch | Subscriptions & Payment Gateways (ID + Global), Load Testing, Security/Child-Safety Audit (UU PDP/COPPA), Beta Launch & Telemetry | Billing, Security, QA, DevOps |

---

## 5. Quick Start for Agents

1. **Read [`AGENTS.md`](./AGENTS.md)** in full before touching any code or task.
2. **Open [`TASKS.md`](./TASKS.md)**, locate an unclaimed task (`[ ]`) whose dependencies are all marked `[x]`.
3. **Claim the task immediately** by updating its status in `TASKS.md` from `[ ]` to `[~]` with your `Agent ID` and UTC timestamp BEFORE writing any code.
4. **Stay strictly within the task's File Ownership boundaries** so parallel agents do not collide.
5. **Run all verification checks** listed in the task's Acceptance Criteria, then update `TASKS.md` from `[~]` to `[x]` and append a concise entry to the **Handoff & Activity Log**.
