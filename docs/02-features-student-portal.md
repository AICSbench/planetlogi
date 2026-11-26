# 02 — Student Portal Complete Functional Specification (A–Z)

> **Owner Tracks**: `FE`, `BE`, `QA`  
> **Scope**: Exhaustive screen-by-screen and flow-by-flow functional specification of the **AlgiPlanet Student Portal** for Elementary Grades 1–6.

---

## 1. Authentication & Login Screen (`/login`)

- **Unauthenticated Redirect**: Accessing any protected route redirects to `/[locale]/login?redirect=<encoded_path>`.
- **Split-Screen Galaxy Layout** (stacks vertically on mobile):
  - **Left Panel — Live Galaxy Activity Feed**: Displays anonymized real-time *"Latest Star Points Submitted"* stream (e.g., masked name + Avatar/Astro Buddy + Grade + `+3 SP earned in Daily Orbit`, refreshed via SSE or polling every 15s) to create an active community feel.
  - **Right Panel — Sign-In Card**:
    - AlgiPlanet Logo & Language Toggle (`EN` / `ID`).
    - **Login ID** input (student username or parent email).
    - **Password** input with **Show / Hide** toggle button.
    - **Sign In** CTA, *Forgot Password* link (routes to parent email/OTP recovery), and *Parent / Teacher Login* notice.
- **Post-Login Routing**: Redirects to `redirect` query param if valid internal path, otherwise `/[locale]/dashboard`.

---

## 2. Persistent Navigation Shell & Student Dashboard (`/dashboard`)

### 2.1 Persistent Top Navigation Bar
- **Left Cluster**:
  1. **AlgiPlanet Logo** (links to `/dashboard`).
  2. **Subject & Plan Switcher Dropdown** (e.g., `Galaxy Math Premium ▾`): Launches with **Math** active; architecture supports future expansion subjects (Science, English, etc.) cleanly via feature flags.
- **Primary Navigation Links**:
  1. **Home** (`/dashboard`)
  2. **Report** (`/report/proficiency`)
  3. **Leaderboard** (`/leaderboard`)
  4. **Achievement** (`/achievement/daily-badges`)
  5. **Events** (`/events`)
- **Right Utility Cluster** (auto-collapses secondary actions into a **3-dot overflow menu** on viewports `<1440px`):
  1. **Orbit Streak Counter** (glowing flame icon + active streak count; links to `/streak/daily-orbit`).
  2. **Bonus Gift Icon** (cosmic gift capsule; opens Daily Login / Mystery Bonus modal when claimable).
  3. **Notification Bell**: Popover dropdown listing live notifications (incoming Star Duels, new Parent/Teacher Assignments, Event announcements, Streak warnings; displays localized *"No new notifications!"* when empty).
  4. **9-Dot App Launcher**: Quick-jump grid modal linking directly to:
     - **Space Academy** (`/academy`)
     - **Star Duels** (`/star-duels`)
     - **Daily Orbit** (`/daily-orbit`)
     - **Sunday Meteor** (`/events/sunday-meteor`)
     - **AstroQuiz** (`/astro-quiz`)
     - **Assignments** (`/assignments`)
     - **Brain Galaxy** (`/brain-galaxy`)
     - **Planet Stories** (`/stories`)
     - **Galaxy Training** (`/training`)
  5. **Account Menu Chevron**: Dropdown with:
     - **Edit Avatar** (`/profile-settings/avatar`)
     - **Edit Profile** (`/profile-settings/profile`)
     - **Settings** (`/profile-settings/settings`)
     - **My Subscription** (`/profile-settings/subscription`)
     - **Help** (`/help`)
     - **Logout**

### 2.2 Dashboard 4-Pillar Hub (`/dashboard`)
- **Left Column — Student Profile Card**:
  - Current **Avatar** (clickable → Edit Avatar), Student **First Name**, **Grade Badge** (`Grade 1`–`Grade 6`), and **Pilot XP Level Pill** (`Lvl 1`–`Lvl 100`, clickable → `/exp-level` showing full XP curve, next milestone rewards, and rank title).
  - **My Astro Buddy** preview widget linking to `/astro-buddy`.
  - **AlgiPlanet Live Classes / Academy Info** tooltip/card (optional future live tutoring info).
- **Center Area — 4-Pillar Navigation Tabs**:
  1. **Star Challenges**: Cards for *Daily Orbit*, *Super Nova Challenge*, *Star Duels*, and *Sunday Meteor*.
  2. **Galaxy Training**: Quick launch to *Missions (6 Curricula)*, *HOTS Constellation Wheel*, *AstroQuiz*, and *Assignments*.
  3. **Space Academy**: Resume last lesson + browse Grade topics & Star Scrolls earned.
  4. **Cosmic Fun**: Quick launch to *Planet Stories*, *Brain Galaxy* (showing remaining daily minutes), *My Astro Buddy*, and *Galaxy Showcase*.
- **Right Panel — Live Currency & Daily Progress Widget**:
  - Displays **Today's Star Points (SP)**, **Total SP**, and **Astro Coins (AC)** balance with info tooltips explaining how each currency is earned and spent.

---

## 3. Daily Orbit (`/daily-orbit`)

The core daily habit loop engineered to take **15–20 minutes**.

- **Operating Hours**: Open **`06:00 – 22:00`** daily in the student's configured timezone. Outside this window, the page displays a friendly cosmic sleeping screen (*"Our planets are recharging! Come back at 6:00 AM."*) while still allowing students to review past worked solutions.
- **10-Question Progression Path** across 3 escalating reward stages:
  - **Q1–Q3** (*"Launchpad Warmup"* — Circle nodes): **1 SP** each
  - **Q4–Q5** (*"Asteroid Hop"* — Circle nodes): **1 SP** each
  - **Q6–Q8** (*"Nebula Push"* — Hexagon nodes): **2 SP** each
  - **Q9–Q10** (*"Supernova Finale"* — Star nodes): **3 SP** each
  - **Max Daily Orbit SP**: `3(1) + 2(1) + 3(2) + 2(3) = 17 SP`
  - **Completion Bonus**: **+5 Astro Coins (AC)** upon completing all 10 questions.
- **4 Progressive Difficulty Tiers** (accessible via Tier Modal):
  1. **Elementary** (Default / Unlocked)
  2. **Intermediate** (Unlocks at **15 total SP**)
  3. **Advanced** (Unlocks at **50 total SP**)
  4. **Master** (Unlocks at **100 total SP**)
- **Active Curriculum(s) Filter Popover**:
  - Allows toggling active question pools for Daily Orbit: **HOTS** and **Singapore Math MOE (G1–G6)** enabled by default, plus **Kurikulum Merdeka**, **Cambridge Primary 0096**, and **Challenging Maths**.
- **Today's SP Breakdown Dropdown**:
  - Summarizes today's points by source: *Daily Orbit*, *Star Duels*, *Sunday Meteor Challenge*, and *Today's Total SP*.
- **Question Management — "Change Question" (Max 3 Edits/Day)**:
  - Before submitting an answer on any Daily Orbit question, student may click **Change Question** (remaining count `3/3` displayed) and choose:
    1. *Make it easier*
    2. *Make it harder*
    3. *Change topic*
- **Single-Attempt Lock & Skip Mechanics**:
  - Submitting an answer locks the question permanently (server rejects repeat submissions with `HTTP 409 Conflict` / `400`).
  - Clicking **Show Answer** before submitting opens a **Skip Question Confirmation** modal warning that **0 SP** will be earned for that question; confirming reveals the full worked solution and marks the node skipped.

---

## 4. Orbit Streak & Cryo Shields (`/streak/daily-orbit`)

- **Daily Milestone Rule**: Answering **at least 5 questions correctly** in Daily Orbit advances the streak by +1 day.
- **Nova Pet Growth Tracker**: Visual cosmic companion evolves across streak milestones (e.g., Egg → Spark → Hatchling → Star Fox → Cosmic Phoenix) with daily progress ring (`X / 5 correct today`).
- **Cryo Shields (Streak Freeze)**:
  - Student can store up to **3 Cryo Shields**.
  - If a student misses a calendar day, 1 Cryo Shield is automatically consumed at midnight to preserve the streak without penalty.
  - **Top-Up / Info Modal** explains auto-protection and how shields are earned (streak milestones, Supply Pods, or Astro Coins).
- **Monthly Orbit Calendar**: Color-coded monthly view showing Completed days (flame), Cryo-Protected days (crystal shield), and Missed days.
- **Daily Reminder Toggle**: *"Turn on Reminder"* requests browser Web Push notification permission (or in-app reminder time preference).

---

## 5. Super Nova Challenge (`/super-nova`)

- **Schedule**: Open **`06:00 – 22:00`**, **Monday – Saturday**.
- **Weekly Diamond `S` Score**: Aggregates weekly points from both modes to feed the weekly *Super Nova* leaderboard.
- **Mode 1: Super Sight (`/super-nova/super-sight`)**:
  - Spot-the-difference visual cognition puzzle using galaxy/kid illustrations.
  - **Rules**: **5 differences** per puzzle, **max 5 wrong taps/attempts**, **10 pts per difference found**, **5-minute countdown limit**.
  - Includes **Score History** table.
- **Mode 2: Super Warp (`/super-nova/super-warp`)**:
  - Rapid-fire **2-minute timed math sprint** tailored to the student's grade.
  - **Rules**: **1 pt per correct answer**, **auto-skips after 10 seconds** per question with **no penalty**.
  - Two entry modes: **"Practise First"** (unranked warmup) and **"Challenge Now"** (ranked attempt).
  - **Fair Pause Mechanic**: Clicking *Pause* takes effect **after the current question** finishes so students cannot freeze a question to calculate offline.
  - Includes **Score History** table.

---

## 6. Galaxy Training / Self-Practice (`/training`)

Offers **6 Structured Curriculum Tracks** (detailed pedagogically in `docs/04-curriculum-and-pedagogy.md`):
1. **Kurikulum Merdeka** (Grades 1–6)
2. **Singapore Math MOE** (Grades 1–6, via **Change Grade** selector; categorized into *Core Topics*, *Word Problems*, and *Challenging*)
3. **Cambridge Primary 0096** (Grades 1–6)
4. **Challenging Maths** (Olympiad/Enrichment)
5. **HOTS Constellation Wheel** (`/training/hots`):
   - Interactive **4-Quadrant Heuristic Wheel** covering **9 problem-solving strategies** across **3 Age Bands** (`6–9`, `8–11`, `10–13`):
     - *Using a Representation*: Draw Diagrams, Draw Models, Make a List
     - *Walking Through the Process*: Act It Out, Work Backwards
     - *Making a Guess*: Guess & Check, Look for Patterns
     - *Changing the Problem*: Make Supposition, Simplify the Problem
6. **Real-World Explorer Series** (`/training/explorer-series`):
   - Competition-style & student-inspired real-world math challenges organized by year & division (**Junior** G1–G3 & **Senior** G4–G6).

### Skill List View Modes & Indicators
Students can toggle every curriculum skill list between two visual modes:
- **High Score Mode**: Shows **3-star rating** per skill (best 10-question session score scaled `0/10` → 0–3 stars) + **5-level difficulty meter**.
- **Proficiency % Mode**: Color-coded mastery badges:
  - **Mastery** (`80–100%`, Emerald)
  - **Competent** (`60–79%`, Blue)
  - **Beginning** (`0–59%`, Amber)
  - **Incomplete** (`<10 questions answered`, Slate)
  - Displays **National Avg %** side-by-side with **Your Proficiency % (`X out of Y`)**.
- Each skill row includes direct **Video Tutorial** modal button and **Practice** CTA (launches a 10-question Galaxy Training session with progress bar *"Complete 10 questions to unlock rewards"*).

---

## 7. Space Academy (`/academy`)

Structured video-and-guided-practice curriculum designed for first-time concept learning.

- **Topic Hub**: Grid of Grade-level topic cards (e.g., ~30 topics per grade such as *Whole Numbers 1–2*, *Addition & Subtraction*, *Fractions*, etc.), each displaying **Star Scrolls** earned (`0 out of 14`).
- **Lesson List (Inside Topic)**: Lists sequential lessons with age tags (e.g., `6–8`) and completion percentage (`0%–100%`).
- **4-Stage Lesson Activity Timeline**:
  1. **Story Launch (*Fun with Math*)**: Short animated/embedded story video introducing the math concept in a real-world or cosmic setting.
  2. **Watch & Learn**: Concise instructional concept video(s) with bilingual subtitles/audio track.
  3. **Do & Learn**: Interactive 4–6 question guided drill using the Space Academy Question Player (`Save`, `Check Answer` → `Try Again` on wrong answer to encourage mastery) awarding **Star Scrolls**.
  4. **Mission Self-Check**: Interactive learning-objective checklist (e.g., *"Count on to 10"*, *"Write numbers 0–10 in numerals and words"*) with a direct button to jump into **Galaxy Training** for extra practice.

---

## 8. Planet Stories (`/stories`)

- **6 Grade Worlds Dropdown**:
  - Grade 1: *Terra Pebble*
  - Grade 2: *Planet Tropica*
  - Grade 3: *Planet Aquaris*
  - Grade 4: *Planet Crystalia*
  - Grade 5: *Planet Lumina*
  - Grade 6: *Planet Solaris*
- **Horizontal Chapter Timeline**:
  - **Opening Chapter** (e.g., *Miko the Star Fox*) is **unlocked free by default** for every planet.
  - Subsequent story arcs (**Prologue / Chapter 1 / Chapter 2 / ...**) unlock for **20 Astro Coins (AC)** each.
- **Illustrated Digital Storybook Viewer (`/stories/[planetId]/viewer/[chapterId]`)**:
  - ~20-page illustrated picture book with swipe/arrow page navigation, bottom page-thumbnail strip, fullscreen toggle, bilingual text switch (`EN`/`ID`), and optional TTS narration.
  - Links to **AlgiPlanet Book Club** reading challenges.

---

## 9. AstroQuiz (`/astro-quiz`)

Structured exam and revision paper hub across **Grades 1–6**:
- **3 Paper Categories**:
  1. **Essential Skills** (Topic-by-topic revision papers)
  2. **Top School Year-End Papers** (Full exam-style mock papers, e.g., `Math-F1`, `Math-I1`)
  3. **HOTS Papers** (Heuristic papers grouped by strategy & age band)
- **Weekly Supply Pod Reward**: Completing any paper before the weekly deadline (Sunday `23:59` local time) awards **1 Supply Pod per week** containing up to **5 Astro Coins (AC)**.
- **Exam Player & Printable Mode**:
  - Live elapsed timer (`00:00:00`), 20-question Proficiency sidebar, **Save** draft, **Submit** (*"Submit paper to see the solutions"*), and **Print Worksheet** (`/astro-quiz/print/[id]`) rendering an ink-friendly A4 printable layout.

---

## 10. Assignments (`/assignments`)

- **My Assignments (`/assignments`)**:
  - Tabs: **All**, **Complete**, **Incomplete**.
  - Filters: **Assigned Year** (e.g., `2026`), **Assigned By** (`All`, `AlgiPlanet`, `Parents`, `Teachers`), and **Refresh** button.
  - Shows due date, progress (`X/Y questions`), assigned by badge, and reward (`AC` + `XP`).
- **Create Personalised Assignment (`/assignments/parent-assign`)**:
  - Parent-accessible builder (gated when accessed by child or opened directly from Parent Portal) allowing selection of specific questions per skill with live child proficiency tags to generate targeted practice homework.

---

## 11. Social Features: Star Duels, Star Crew & Galaxy Leaderboards

### 11.1 Star Duels — 1v1 Peer Challenge (`/star-duels`)
- **Daily Energy Cap**: **3 energy cells per day** (prevents grinding; resets midnight user TZ).
- **Tabs**: **New Challenge**, **Incoming**, **History**, **Rules**.
- **3 Matchmaking Options**:
  1. **My Classmate** (select from class roster dropdown)
  2. **My Star Crew** (select from friend crew list)
  3. **Random Pilot** (`Match` button pairing with a same-grade opponent using masked identity)
- Awards **Star Points (SP)** based on duel score & win bonus (counted in Today's SP breakdown).

### 11.2 Star Crew — Best Friends Team (`/star-crew`)
- Each student receives a unique **9-digit Crew Code** to invite friends safely (no open chat, no public search by personal details).
- Crew members pool weekly SP to compete on the weekly **Star Crew** leaderboard.

### 11.3 Galaxy Leaderboards (`/leaderboard`)
- Updates every **5 minutes** via Redis-backed ranking snapshots.
- Subject switcher (`Math` active; multi-subject ready).
- **6 Open Challenges**:
  1. **Hero of the Day** (Daily `06:00–22:00`)
  2. **Top Brain** (Quarterly)
  3. **Champion Class** (Monthly)
  4. **Star Crew** (Weekly Mon–Sun)
  5. **Super Nova** (Weekly Mon–Sat, ranked by diamond `S` score)
  6. **Orbit Streak** (Daily `06:00–22:00`)
- **4 Seasonal / Closed Event Challenges**:
  - *Zodiac Warp*, *Comet vs Meteor*, *Star Academy Cup*, *Galactic City Race* (activated on schedule via Admin CMS).
- **Drill-Down Controls**:
  - Scopes: **My Class**, **My School**, **National** (e.g., *Indonesia Schools* / *Global*).
  - Views: **Show Top 50**, **See Who is Nearby** (`±5` positions around student), **Grade Filter** (`Grade 1–6`), and **Hall of Fame** archive.
  - **Privacy**: Peer names are always masked (e.g., `Amr S.` or `Pilot A***`).

---

## 12. Collectibles, Customization & Brain Galaxy

### 12.1 Daily Star Badges (`/achievement/daily-badges`)
- Monthly calendar grid of collectible cosmic/math trivia cards.
- **4 Upgradeable Rarity Tiers** per day: **1★ Bronze**, **2★ Silver**, **3★ Gold**, **4★ Holographic** (upgraded by completing Daily Orbit + bonus milestones / future multi-subject challenges).
- Missed days can be unlocked retroactively for **20 Astro Coins (AC)**.

### 12.2 My Astro Buddy (`/astro-buddy`) & Galaxy Showcase (`/showcase`)
- **Astro Buddy Builder**: Interactive SVG/layered alien-monster customizer:
  - **Body Shape**, **15 Cosmic Colors**, **Horns / Antennae / Ears**, **Eyes**, **Mouth**.
  - **2-Part Cosmic Name Generator**: ~50 kid-friendly adjectives (e.g., *Turbo, Starry, Cosmic, Nebula, Bouncy*) + student's first name (zero free-text profanity risk!).
- **Galaxy Showcase (`/showcase`)**: Showcases students' custom Astro Buddies ranked by Star Points across **Grades 1–6** (requires `>=1 SP` to appear).

### 12.3 Brain Galaxy (`/brain-galaxy`)
- Interactive **HTML5 Galaxy Island** with **5 Cognitive Zones**:
  1. **Memory** (pattern & sequence recall)
  2. **Flexibility** (rule-switching & categorization)
  3. **Speed** (reflex arithmetic & visual matching)
  4. **Problem Solving** (spatial & logic puzzles)
  5. **Adventure** (multi-stage math exploration)
- **Strict Healthy Usage Guardrails (Server-Enforced)**:
  - **Operating Hours**: `08:00–18:00` (School accounts) or `08:00–20:00` (Individual Standard/Premium accounts; parents can tighten further).
  - **Daily Time Cap**: **25 minutes/day maximum** with live countdown HUD and automatic graceful pause/exit when time hits `00:00`.
  - **Entry Economy**: **1 Astro Coin (AC) per game per day** (re-entering the same game later the same day is free).

---

## 13. Cosmic Events (`/events`)

- **Sunday Meteor Challenge**: Opens every **Sunday `06:00–22:00`** — special 10-question timed/themed challenge awarding bonus SP & AC.
- **Quarterly Creator Challenge**: Seasonal real-world problem-solving competition where students submit solutions (and optional problem ideas moderated via CMS) to win medals and be featured in the *Real-World Explorer Series*.

---

## 14. Progress Reports (`/report/proficiency`)

- **Proficiency Report Tab**:
  - Filters: All **6 Curricula** & **Grade Level (G1–G6)**.
  - Categorizes every skill into **Mastery (80–100%)**, **Competent (60–79%)**, **Beginning (0–59%)**, and **Incomplete (<10 Qs)** with expandable per-topic grids and direct "Practice Now" deep links.
- **High Score Report Tab**:
  - Visualizes best scores across **star tiers** (`9–10`, `6–8`, `4–5`, `1–3` out of 10) for every practiced skill.

---

## 15. Account, Profile, Settings, Subscription & Help (`/profile-settings/*`)

- **Edit Avatar (`/profile-settings/avatar`)**: 8 illustrated cosmic & kid theme packs (*Space Pets*, *Cosmic Sea Creatures*, *Frost Explorers*, *Star Royals*, *Astro Dinos*, *Algi Crew*, *Mini Bots*, *Mecha Pilots*).
- **Edit Profile (`/profile-settings/profile`)**:
  - **Parent Profile**: Parent Name, Phone Number, Email (masked).
  - **Child Profile**: Avatar, First Name, School Name (with visible note of scheduled automatic grade promotion, e.g., *"Auto-promotes to Grade 2 on 1 Jul 2027"*), Current Grade, Birthday, Gender, Change Password, and link to Parent Portal.
- **Settings (`/profile-settings/settings`)** *(Parent-OTP gated when enabled)*:
  - Toggle **OTP Account Security**.
  - **Language Switcher**: **English (`en`)** and **Bahasa Indonesia (`id`)** (architecture extensible to additional languages).
  - Active **Daily Orbit Curriculum(s)** & **Star Duel Curriculum(s)** checkboxes.
  - Healthy screen-time & reminder preferences.
- **My Subscription (`/profile-settings/subscription`)**: Active plan name (e.g., *AlgiPlanet Math Premium*), validity dates, billing history, and *Manage Subscription* CTA (Parent-gated).
- **Help (`/help`)**: Bilingual Help Center with searchable FAQs, video walkthroughs, and parent support contact form.
